// Integration checks of the release archives, after scripts/package.mjs. They
// check every archive dist/ holds, so a local run that packaged only darwin
// checks that one; CI and the release package all three.
import test from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const id = "io.github.lithe-idea.go-support";
const source = JSON.parse(readFileSync(`${id}/plugin.json`, "utf8"));
const version = source.version;
const UNIVERSAL = 0xcafebabe;
const CPU_X86_64 = 0x01000007;
const CPU_ARM64 = 0x0100000c;
const archives = [
  { platform: "darwin", program: "bin/backend" },
  { platform: "windows-x64", program: "bin/backend.exe", machine: 0x8664 },
  { platform: "windows-arm64", program: "bin/backend.exe", machine: 0xaa64 },
]
  .map((archive) => ({ ...archive, path: resolve(`dist/go-support-${version}-${archive.platform}.zip`) }))
  .filter((archive) => existsSync(archive.path));

const unzip = (archive, args) => execFileSync("unzip", [...args, archive.path], { timeout: 10_000, maxBuffer: 16 * 1024 * 1024 });
const read = (archive, path) => execFileSync("unzip", ["-p", archive.path, `${id}/${path}`], { timeout: 10_000, maxBuffer: 16 * 1024 * 1024 });
const entries = (archive) => unzip(archive, ["-Z1"]).toString("utf8").trim().split("\n");

test("dist holds an archive to check", () => {
  assert.ok(archives.length > 0, `no dist/go-support-${version}-*.zip; run node scripts/package.mjs`);
});

for (const archive of archives) {
  test(`${archive.platform}: plugin.json is the source one, launching ${archive.program}`, () => {
    const manifest = JSON.parse(read(archive, "plugin.json"));
    assert.equal(manifest.backend.launch.path, archive.program);
    assert.deepEqual({ ...manifest, backend: { ...manifest.backend, launch: { ...manifest.backend.launch, path: source.backend.launch.path } } }, source);
  });

  test(`${archive.platform}: the backend is an executable for the platform, with its notices`, () => {
    const listing = execFileSync("unzip", ["-Z", archive.path, `${id}/${archive.program}`], { encoding: "utf8", timeout: 10_000 });
    assert.match(listing, /^-rwxr-xr-x/, "the archive keeps the program's mode");
    const program = read(archive, archive.program);
    if (archive.machine === undefined) {
      assert.equal(program.readUInt32BE(0), UNIVERSAL, "a universal Mach-O binary");
      const count = program.readUInt32BE(4);
      const cpus = Array.from({ length: count }, (_, index) => program.readUInt32BE(8 + index * 20)).sort();
      assert.deepEqual(cpus, [CPU_X86_64, CPU_ARM64].sort(), "slices for Intel and Apple Silicon");
    } else {
      assert.equal(program.toString("latin1", 0, 2), "MZ");
      const header = program.readUInt32LE(0x3c);
      assert.equal(program.toString("latin1", header, header + 4), "PE\0\0");
      assert.equal(program.readUInt16LE(header + 4), archive.machine, "the PE machine of the platform");
    }
    assert.match(read(archive, "THIRD_PARTY_LICENSES.txt").toString("utf8"), /^lithe-plugin-sdk .* \(Apache-2\.0\)/m);
    assert.ok(!entries(archive).some((entry) => entry.includes("backend/") || entry.endsWith(".rs")), "no backend sources");
  });
}

for (const archive of archives.filter((entry) => entry.platform.startsWith("windows"))) {
  test(`${archive.platform}: GOROOT resolves go.exe`, () => {
    const manifest = JSON.parse(read(archive, "plugin.json"));
    const go = manifest.toolDefinitions.find((tool) => tool.id === "go");
    const forWindows = (list) => list.filter((entry) => entry.platforms?.includes("windows")).map((entry) => entry.path);
    assert.deepEqual(forWindows(go.executableLayouts), ["bin/go.exe"]);
    assert.deepEqual(forWindows(go.executables), ["bin/go.exe"]);
  });

  test(`${archive.platform}: every archive entry can be extracted on Windows`, () => {
    const seen = new Set();
    for (const entry of entries(archive)) {
      for (const part of entry.replace(/\/$/, "").split("/")) {
        assert.doesNotMatch(part, /[<>:"\\|?*\x00-\x1f]|[. ]$/, `${entry} has a name Windows refuses`);
        assert.doesNotMatch(part, /^(con|prn|aux|nul|com\d|lpt\d)(\.|$)/i, `${entry} uses a reserved Windows name`);
      }
      const folded = entry.toLowerCase();
      assert.ok(!seen.has(folded), `${entry} differs from another entry only in case`);
      seen.add(folded);
    }
  });
}
