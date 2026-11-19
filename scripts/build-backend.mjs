#!/usr/bin/env node
// Builds the plugin's backend, the Rust program in backend/ that draws the
// settings regions, for one platform into .artifacts/backend-<platform>/, where
// scripts/package.mjs takes it from:
//
//   darwin          backend, a universal binary of aarch64- and x86_64-apple-darwin
//                   joined with lipo and signed ad hoc, which Apple Silicon needs
//                   to run it. macOS only.
//   windows-x64     backend.exe for x86_64-pc-windows-msvc. Windows only.
//   windows-arm64   backend.exe for aarch64-pc-windows-msvc, cross-built on x64
//                   Windows with the MSVC ARM64 tools. Windows only.
//
// Next to the program it writes THIRD_PARTY_LICENSES.txt: every crate linked
// into that platform's build, its license, and the license files it ships, read
// from `cargo metadata`. The targets must be installed (`rustup target add`).
//
// Usage: node scripts/build-backend.mjs <darwin|windows-x64|windows-arm64>

import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { copyFileSync, existsSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";

const PLATFORMS = {
  darwin: { host: "darwin", targets: ["aarch64-apple-darwin", "x86_64-apple-darwin"], program: "backend" },
  "windows-x64": { host: "win32", targets: ["x86_64-pc-windows-msvc"], program: "backend.exe" },
  "windows-arm64": { host: "win32", targets: ["aarch64-pc-windows-msvc"], program: "backend.exe" },
};
const BACKEND = resolve("backend");
const CARGO_TIMEOUT_MS = 20 * 60_000;

const platform = process.argv[2];
const spec = PLATFORMS[platform];
if (!spec) {
  console.error(`usage: node scripts/build-backend.mjs <${Object.keys(PLATFORMS).join("|")}>`);
  process.exit(2);
}
if (process.platform !== spec.host) {
  console.error(`✗ ${platform} builds on ${spec.host}, this is ${process.platform}`);
  process.exit(1);
}

const output = resolve(".artifacts", `backend-${platform}`);
rmSync(output, { recursive: true, force: true });
mkdirSync(output, { recursive: true });

const built = spec.targets.map((target) => {
  execFileSync("cargo", ["build", "--release", "--locked", "--target", target], {
    cwd: BACKEND, stdio: "inherit", timeout: CARGO_TIMEOUT_MS,
  });
  return join(BACKEND, "target", target, "release", spec.program);
});
const program = join(output, spec.program);
if (built.length === 1) {
  copyFileSync(built[0], program);
} else {
  execFileSync("lipo", ["-create", ...built, "-output", program], { stdio: "inherit", timeout: 60_000 });
  // The linker signs only the arm64 slice. An ad-hoc signature without a
  // timestamp covers both and is the same for the same bytes.
  execFileSync("codesign", ["--sign", "-", "--force", "--timestamp=none", program], { stdio: "inherit", timeout: 60_000 });
  execFileSync("codesign", ["--verify", "--strict", program], { stdio: "inherit", timeout: 60_000 });
}

writeFileSync(join(output, "THIRD_PARTY_LICENSES.txt"), notices(spec.targets));
const sha256 = createHash("sha256").update(readFileSync(program)).digest("hex");
console.log(`.artifacts/backend-${platform}/${spec.program} ${sha256}`);

/**
 * The license notices of every crate a build for `targets` links, the backend
 * itself left out. A crate that ships no license file, such as the plugin SDK,
 * whose repository keeps none, is covered by the Apache License 2.0 text at
 * the end, which is this repository's LICENSE.
 */
function notices(targets) {
  const crates = new Map();
  for (const target of targets) {
    const metadata = JSON.parse(execFileSync("cargo", [
      "metadata", "--format-version", "1", "--locked", "--filter-platform", target,
    ], { cwd: BACKEND, encoding: "utf8", maxBuffer: 64 * 1024 * 1024, timeout: 5 * 60_000 }));
    const packages = new Map(metadata.packages.map((entry) => [entry.id, entry]));
    const root = metadata.resolve.root;
    for (const node of metadata.resolve.nodes) {
      if (node.id === root) continue;
      const entry = packages.get(node.id);
      crates.set(`${entry.name} ${entry.version}`, entry);
    }
  }
  const sorted = [...crates.values()].sort((a, b) => a.name.localeCompare(b.name));
  const lines = [
    "Third-party software in the Go Support backend",
    "",
    "The backend program links these crates. Each line gives the crate, its",
    "version, its license and its source; the license files the crates ship follow.",
    "",
  ];
  let needsApache = false;
  for (const entry of sorted) {
    const source = entry.repository ?? (entry.source ?? "").replace(/^git\+/, "").replace(/[?#].*$/, "");
    lines.push(`${entry.name} ${entry.version} (${entry.license}) ${source}`);
  }
  const seen = new Map();
  for (const entry of sorted) {
    const directory = dirname(entry.manifest_path);
    const files = readdirSync(directory).filter((name) => /^(licen[cs]e|copying|notice)/i.test(name)).sort();
    if (files.length === 0) {
      if (!/\bApache-2\.0\b/.test(entry.license ?? "")) {
        throw new Error(`${entry.name} ${entry.version} ships no license file and is not Apache-2.0: ${entry.license}`);
      }
      needsApache = true;
      lines.push("", `==== ${entry.name} ${entry.version} ====`, "", "Ships no license file; Apache License 2.0, text below.");
      continue;
    }
    for (const file of files) {
      const text = lf(readFileSync(join(directory, file), "utf8")).trimEnd();
      const digest = createHash("sha256").update(text).digest("hex");
      lines.push("", `==== ${entry.name} ${entry.version}: ${file} ====`, "");
      if (seen.has(digest)) {
        lines.push(`The same text as ${seen.get(digest)} above.`);
      } else {
        seen.set(digest, `${entry.name} ${entry.version}: ${file}`);
        lines.push(text);
      }
    }
  }
  if (needsApache) {
    const license = resolve("LICENSE");
    if (!existsSync(license)) throw new Error("LICENSE, the Apache License 2.0 text, is missing");
    lines.push("", "==== Apache License 2.0 ====", "", lf(readFileSync(license, "utf8")).trimEnd());
  }
  return `${lines.join("\n")}\n`;
}

/** `text` with LF line ends, which a Windows checkout's CRLF files lack. */
function lf(text) {
  return text.replace(/\r\n/g, "\n");
}
