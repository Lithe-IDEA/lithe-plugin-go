#!/usr/bin/env node
// Runs the commands plugin.json declares with the real go, the way Lithe runs
// them, on a module rendered from the project templates under a path with a
// space: the version probe, the model and dependency commands (decoded as JSON
// streams), the build, vet and filtered test tasks, the Go Build and Go Test
// run configuration types with their fields filled in, the directories the
// command installers read from `go env`, and the managed installs of gopls and
// dlv into a GOBIN of their own. The installed programs are then found the
// way Lithe finds a system command (on PATH, so gopls.exe on Windows), gopls
// answers its validation arguments, and `dlv dap` announces the endpoint the
// debug adapter declaration reads.
//
// GO selects the go executable (default `go`). LITHE_TEST_PLUGIN_DIR tests an
// unpacked release archive instead of the source directory. LITHE_TEST_INSTALL=0
// skips the gopls and dlv installs, which download and build them. Where Delve
// has no backend (Windows ARM64 as of Delve 1.27) its own build refuses, and
// the dlv checks are reported as skipped, not failed.
//
// Usage: node scripts/test.mjs

import assert from "node:assert/strict";
import { spawn, spawnSync } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, realpathSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { delimiter, isAbsolute, join, resolve } from "node:path";

const PACKAGE_DIR = "io.github.lithe-idea.go-support";
const pluginDir = resolve(process.env.LITHE_TEST_PLUGIN_DIR || PACKAGE_DIR);
const manifest = JSON.parse(readFileSync(join(pluginDir, "plugin.json"), "utf8"));
const buildSystem = manifest.buildSystems.find((entry) => entry.id === "go");
const tool = manifest.toolDefinitions.find((entry) => entry.id === "go");
const go = process.env.GO ?? "go";
const exe = process.platform === "win32" ? ".exe" : "";
const COMMAND_TIMEOUT_MS = 10 * 60_000;

/** Expands a declared argument list: no option has a value, conditions as given. */
function expand(list, { target, testCase } = {}) {
  const conditions = {
    targetSelected: target !== undefined,
    noTargetSelected: target === undefined,
    caseSelected: testCase !== undefined,
    noCaseSelected: testCase === undefined,
  };
  return list.flatMap((item) => {
    const value = typeof item === "string" ? item : item.value;
    const when = typeof item === "string" ? [] : [item.when].flat();
    if (!when.every((condition) => conditions[condition])) return [];
    if (/^\$\{optionArguments(:[A-Za-z]+)?\}$/.test(value)) return [];
    return [value
      .replaceAll("${target.name}", target ?? "")
      .replaceAll("${test.case:regex}", (testCase ?? "").replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))];
  });
}

/**
 * Expands a declared run configuration type the way Core plans a saved
 * configuration of it: a field passes nothing while hidden or empty, a field
 * with arguments passes them with `${value}` replaced (a checkbox when
 * checked), a field without them passes its value, which is an absolute path
 * for a file or directory and a list for an arguments field. No option has a
 * value.
 */
function expandRunType(type, values, userArguments = []) {
  const valueOf = (field) => values[field.id] ?? field.default;
  const shown = (field) => !field.visibleWhen
    || valueOf(type.fields.find((entry) => entry.id === field.visibleWhen.field)) === field.visibleWhen.equals;
  const passed = (field) => {
    const value = valueOf(field);
    if (!shown(field) || value === undefined || value === "" || value === false) return [];
    if (field.kind === "arguments") return value;
    if (!field.arguments) return [value];
    return field.arguments.map((argument) => argument.replaceAll("${value}", value));
  };
  return type.arguments.flatMap((argument) => {
    if (argument === "${optionArguments}") return [];
    if (argument === "${userArguments}") return userArguments;
    const field = argument.match(/^\$\{field:([A-Za-z0-9]+)\}$/);
    if (field) return passed(type.fields.find((entry) => entry.id === field[1]));
    assert.doesNotMatch(argument, /\$\{/, `the test does not expand ${argument}`);
    return [argument];
  });
}

function run(command, args, { cwd, env = process.env } = {}) {
  const result = spawnSync(command, args, {
    cwd, env, encoding: "utf8", timeout: COMMAND_TIMEOUT_MS, maxBuffer: 64 * 1024 * 1024, windowsHide: true,
  });
  if (result.error) throw new Error(`${command} ${args.join(" ")}: ${result.error.message}`);
  if (result.status !== 0) {
    throw new Error(`${command} ${args.join(" ")} exited with ${result.status ?? result.signal}\n${result.stdout}${result.stderr}`);
  }
  return result.stdout;
}

/** The jsonStream decoder: `go list -json` prints one top-level object per line-start brace. */
const jsonStream = (text) => text.split(/^(?=\{)/m).filter((part) => part.trim()).map((part) => JSON.parse(part));
const samePath = (a, b) => realpathSync.native(a) === realpathSync.native(b);

/** A check that cannot run on this platform for a reason outside the plugin. */
class Skip extends Error {}
// Delve refuses to build where it has no backend, through a package named
// your_<os>_architecture_is_not_supported_by_delve (Windows ARM64 as of 1.27).
const DELVE_UNSUPPORTED = /your_[a-z]+_architecture_is_not_supported_by_delve/;

let failed = false;
async function check(name, body) {
  const started = Date.now();
  try {
    await body();
    console.log(`✓ ${name} (${Date.now() - started} ms)`);
  } catch (error) {
    if (error instanceof Skip) return console.log(`- ${name}: ${error.message}`);
    failed = true;
    console.error(`✗ ${name}\n${error.stack}`);
  }
}

const scratch = realpathSync(mkdtempSync(join(tmpdir(), "lithe-go-test-")));
const root = join(scratch, "Lithe Go Demo");
try {
  let version;
  await check("the version probe reads a supported go", () => {
    const output = run(go, tool.version.arguments);
    const match = output.match(new RegExp(tool.version.pattern));
    assert.ok(match, `no version in ${JSON.stringify(output)}`);
    version = match[1];
    const minimum = buildSystem.invocation.runtimeDependencies.find((entry) => entry.id === "go").minimumVersion;
    const [major, minor] = version.split(".").map(Number);
    const [wantMajor, wantMinor] = minimum.split(".").map(Number);
    assert.ok(major > wantMajor || (major === wantMajor && minor >= wantMinor), `go ${version} is older than ${minimum}`);
  });
  if (!version) throw new Error("no usable go; the remaining checks need it");

  // The Go project template with sample code, plus a tested package.
  mkdirSync(join(root, "greet"), { recursive: true });
  const template = (path) => readFileSync(join(pluginDir, "templates", "go", path), "utf8");
  writeFileSync(join(root, "go.mod"), template("module/go.mod")
    .replace("{{ field.modulePath }}", "lithe-demo")
    .replace("{{ field.goVersion }}", version.split(".").slice(0, 2).join(".")));
  writeFileSync(join(root, "main.go"), template("sample/main.go"));
  writeFileSync(join(root, "greet", "greet.go"), 'package greet\n\nfunc Hello(name string) string { return "Hello, " + name + "!" }\n');
  writeFileSync(join(root, "greet", "greet_test.go"), [
    "package greet", "", 'import "testing"', "",
    'func TestGreet(t *testing.T) { if Hello("Go") != "Hello, Go!" { t.Fatal("wrong greeting") } }', "",
    'func TestOther(t *testing.T) { t.Fatal("the case filter must skip this test") }', "",
  ].join("\n"));

  await check("the model command lists the module, its program and its tested package", () => {
    const packages = jsonStream(run(go, buildSystem.model.arguments, { cwd: root }));
    const module = packages[0].Module;
    assert.equal(module.Path, "lithe-demo");
    assert.ok(samePath(module.Dir, root), `${module.Dir} is the module directory`);
    const main = packages.filter((entry) => entry.Name === "main").map((entry) => entry.ImportPath);
    assert.deepEqual(main, ["lithe-demo"]);
    const tested = packages.filter((entry) => entry.TestGoFiles).map((entry) => entry.ImportPath);
    assert.deepEqual(tested, ["lithe-demo/greet"]);
    assert.ok(packages.every((entry) => isAbsolute(entry.Dir)));
  });

  await check("the dependency command lists only the main module of a module without requirements", () => {
    const modules = jsonStream(run(go, buildSystem.dependencies.arguments, { cwd: root }));
    assert.deepEqual(modules.map((entry) => [entry.Path, entry.Main === true]), [["lithe-demo", true]]);
  });

  await check("build and vet run on the module and on a target", () => {
    const task = (id) => buildSystem.tasks.find((entry) => entry.id === id).arguments;
    run(go, expand(task("build")), { cwd: root });
    run(go, expand(task("vet")), { cwd: root });
    run(go, expand(task("vet"), { target: "lithe-demo/greet" }), { cwd: root });
  });

  await check("a filtered test run runs only the selected case", () => {
    const output = run(go, expand(buildSystem.test.arguments, { target: "lithe-demo/greet", testCase: "TestGreet" }), { cwd: root });
    assert.match(output, /^ok\s+lithe-demo\/greet/m);
  });

  const runType = (id) => buildSystem.runConfigurationTypes.find((entry) => entry.id === id);
  // A second program that prints its arguments, for the program arguments.
  mkdirSync(join(root, "cmd", "echo"), { recursive: true });
  writeFileSync(join(root, "cmd", "echo", "main.go"), [
    "package main", "", 'import (\n\t"fmt"\n\t"os"\n\t"strings"\n)', "",
    'func main() { fmt.Println("echo:", strings.Join(os.Args[1:], "|")) }', "",
  ].join("\n"));

  await check("Go Build runs a package by import path or relative path, with the program arguments", () => {
    const build = runType("build");
    assert.match(run(go, expandRunType(build, { package: "lithe-demo" }), { cwd: root }), /^Hello, Gopher!$/m);
    const output = run(go, expandRunType(build, { package: "./cmd/echo", goToolArguments: ["-mod=mod"] }, ["one two", "three"]), { cwd: root });
    assert.match(output, /^echo: one two\|three$/m);
  });

  await check("Go Build runs a file and a directory given as absolute paths", () => {
    const build = runType("build");
    const file = expandRunType(build, { runKind: "file", package: "./cmd/echo", file: join(root, "main.go") });
    assert.deepEqual(file, ["run", join(root, "main.go")], "the hidden package field passes nothing");
    assert.match(run(go, file, { cwd: root }), /^Hello, Gopher!$/m);
    const directory = expandRunType(build, { runKind: "directory", directory: join(root, "cmd", "echo") }, ["x"]);
    assert.match(run(go, directory, { cwd: root }), /^echo: x$/m);
  });

  await check("Go Test runs the selected tests of a package and of every package below a directory", () => {
    const test = runType("test");
    const pattern = "^TestGreet$";
    const output = run(go, expandRunType(test, { package: "./greet", pattern, goToolArguments: ["-v", "-count=1"] }), { cwd: root });
    assert.match(output, /^--- PASS: TestGreet/m);
    assert.doesNotMatch(output, /TestOther/, "the pattern leaves TestOther out");
    const byDirectory = expandRunType(test, { testKind: "directory", directory: root, pattern });
    assert.equal(byDirectory.at(-1), `${root}/...`);
    assert.match(run(go, byDirectory, { cwd: root }), /^ok\s+lithe-demo\/greet/m);
  });

  await check("Go Test with the default package and no pattern runs every test", () => {
    assert.throws(() => run(go, expandRunType(runType("test"), { goToolArguments: ["-count=1"] }), { cwd: root }), /--- FAIL: TestOther/);
  });

  await check("the installers' default directories come from go env", () => {
    for (const installer of manifest.commandInstallers) {
      const [gobin, gopath] = installer.targets.toolDefault.directory;
      const configured = run(go, gobin.arguments).trim();
      if (configured) assert.ok(isAbsolute(configured), `GOBIN ${configured}`);
      const list = run(go, gopath.arguments).trim();
      const first = gopath.pathList ? list.split(delimiter)[0] : list;
      assert.ok(isAbsolute(join(first, gopath.append)), `GOPATH ${first}`);
    }
  });

  if (process.env.LITHE_TEST_INSTALL === "0") {
    console.log("- skipped the gopls and dlv installs (LITHE_TEST_INSTALL=0)");
  } else {
    const bin = join(scratch, "Lithe Tools", "bin");
    mkdirSync(bin, { recursive: true });
    const env = { ...process.env, PATH: `${bin}${delimiter}${process.env.PATH}` };
    let noDelve;
    for (const installer of manifest.commandInstallers) {
      await check(`the managed ${installer.id} install writes ${installer.commands.join(", ")} to its GOBIN`, () => {
        const environment = Object.fromEntries(Object.entries(installer.targets.managed.environment)
          .map(([name, value]) => [name, value.replaceAll("${installDirectory}", bin)]));
        try {
          run(go, installer.arguments, { cwd: scratch, env: { ...process.env, ...environment } });
        } catch (error) {
          if (installer.id !== "dlv" || !DELVE_UNSUPPORTED.test(error.message)) throw error;
          noDelve = `Delve does not support ${process.platform}/${process.arch}; its install fails in Delve's own build`;
          throw new Skip(noDelve);
        }
        for (const command of installer.commands) {
          assert.ok(existsSync(join(bin, command + exe)), `${command}${exe} in ${readdirSync(bin).join(", ")}`);
        }
      });
    }

    await check("the language server is found as a system command and answers its validation arguments", () => {
      const server = manifest.languageServers.find((entry) => entry.languageID === "go");
      assert.equal(server.launch.kind, "systemCommand");
      const output = run(server.launch.commands[0], server.validationArguments, { cwd: root, env });
      assert.match(output, /golang\.org\/x\/tools\/gopls/);
    });

    await check("dlv dap announces the endpoint the debug adapter reads", async () => {
      if (noDelve) throw new Skip(noDelve);
      const { process: launch, endpoint } = manifest.debugAdapters.find((entry) => entry.id === "delve").launch;
      assert.equal(launch.launch.kind, "systemCommand");
      const child = spawn(launch.launch.commands[0], launch.arguments, { cwd: root, env, stdio: ["ignore", "pipe", "pipe"], windowsHide: true });
      try {
        const line = await new Promise((resolvePromise, reject) => {
          let output = "";
          const timer = setTimeout(() => reject(new Error(`no announcement within 30 s:\n${output}`)), 30_000);
          const read = (chunk) => {
            output += chunk;
            const found = output.split(/\r?\n/).find((entry) => entry.startsWith(endpoint.prefix));
            if (found) { clearTimeout(timer); resolvePromise(found); }
          };
          const other = endpoint.stream === "stdout" ? "stderr" : "stdout";
          child[endpoint.stream].setEncoding("utf8").on("data", read);
          child[other].setEncoding("utf8").on("data", (chunk) => { output += chunk; });
          child.once("error", (error) => { clearTimeout(timer); reject(error); });
          child.once("exit", (code) => { clearTimeout(timer); reject(new Error(`dlv exited with ${code}:\n${output}`)); });
        });
        assert.match(line.slice(endpoint.prefix.length).trim(), /^(\[[0-9a-f:]+\]|[^\s:]+):\d+$/i, "a host:port value");
      } finally {
        child.kill();
      }
    });
  }
} catch (error) {
  failed = true;
  console.error(error.stack);
} finally {
  rmSync(scratch, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 });
}
process.exitCode = failed ? 1 : 0;
