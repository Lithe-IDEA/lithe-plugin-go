// Integration checks of the release archives, after scripts/package.mjs.
import test from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { createHash } from "node:crypto";

const id = "io.github.lithe-idea.go-support";
const version = JSON.parse(readFileSync(`${id}/plugin.json`, "utf8")).version;
const windows = resolve(`dist/go-support-${version}-windows.zip`);
const digest = (path) => createHash("sha256").update(readFileSync(path)).digest("hex");
const read = (path) => execFileSync("unzip", ["-p", windows, `${id}/${path}`], { timeout: 10_000, maxBuffer: 16 * 1024 * 1024 });

test("Windows ARM64 and x64 use the portable archive", () => {
  assert.equal(digest(windows), digest(`dist/go-support-${version}-darwin.zip`));
  const manifest = JSON.parse(read("plugin.json"));
  assert.equal(manifest.version, version);
  const go = manifest.toolDefinitions.find((tool) => tool.id === "go");
  const forWindows = (entries) => entries.filter((entry) => entry.platforms?.includes("windows")).map((entry) => entry.path);
  assert.deepEqual(forWindows(go.executableLayouts), ["bin/go.exe"]);
  assert.deepEqual(forWindows(go.executables), ["bin/go.exe"], "GOROOT resolves go.exe on Windows");
});

test("every archive entry can be extracted on Windows", () => {
  const entries = execFileSync("unzip", ["-Z1", windows], { encoding: "utf8", timeout: 10_000 }).trim().split("\n");
  const seen = new Set();
  for (const entry of entries) {
    for (const part of entry.replace(/\/$/, "").split("/")) {
      assert.doesNotMatch(part, /[<>:"\\|?*\x00-\x1f]|[. ]$/, `${entry} has a name Windows refuses`);
      assert.doesNotMatch(part, /^(con|prn|aux|nul|com\d|lpt\d)(\.|$)/i, `${entry} uses a reserved Windows name`);
    }
    const folded = entry.toLowerCase();
    assert.ok(!seen.has(folded), `${entry} differs from another entry only in case`);
    seen.add(folded);
  }
});
