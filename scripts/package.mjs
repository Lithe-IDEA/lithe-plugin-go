#!/usr/bin/env node
// Packages the plugin into one archive per platform, under the asset names
// Lithe Store's release tracker recognizes (docs/publishing.md there):
//
//   dist/go-support-<version>-darwin.zip         Apple Silicon and Intel Macs
//   dist/go-support-<version>-windows-x64.zip    Windows x64
//   dist/go-support-<version>-windows-arm64.zip  Windows ARM64
//
// Each holds the package directory and the backend that draws its settings
// regions, built beforehand by scripts/build-backend.mjs into
// .artifacts/backend-<platform>/: bin/backend, a universal binary, on macOS and
// bin/backend.exe on Windows, with THIRD_PARTY_LICENSES.txt next to plugin.json.
// The Windows archives' plugin.json names bin/backend.exe as the backend; all
// three declare the same version, pluginAPIVersion and requires, as the Store
// requires. Everything else, Lithe launching the user's own go, gopls and dlv,
// is the same on every platform. Nothing is downloaded at build or install
// time. Packaging needs POSIX zip, so it runs on macOS or Linux.
//
// Usage: node scripts/package.mjs [--expect-tag <tag>] [--platforms darwin,windows-x64,windows-arm64]

import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { chmodSync, cpSync, existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { parseArgs } from "node:util";

const PACKAGE_DIR = "io.github.lithe-idea.go-support";
const ASSET_PREFIX = "go-support";
const BACKEND_PATH = "bin/backend";
const PLATFORMS = {
  darwin: { program: "backend", builtOn: "macOS" },
  "windows-x64": { program: "backend.exe", builtOn: "Windows" },
  "windows-arm64": { program: "backend.exe", builtOn: "Windows" },
};

const { values } = parseArgs({
  options: {
    "expect-tag": { type: "string" },
    platforms: { type: "string", default: Object.keys(PLATFORMS).join(",") },
  },
});
const platforms = values.platforms.split(",").filter(Boolean);

const source = readFileSync(`${PACKAGE_DIR}/plugin.json`, "utf8");
const manifest = JSON.parse(source);
const problems = [];
if (manifest.id !== PACKAGE_DIR) problems.push(`plugin.json id ${manifest.id} must equal the directory ${PACKAGE_DIR}`);
if (!/^[0-9]+\.[0-9]+\.[0-9]+$/.test(manifest.version)) problems.push(`version ${manifest.version} is not MAJOR.MINOR.PATCH`);
if (manifest.vendor?.signatureRequirement !== "none") problems.push('vendor.signatureRequirement must be "none"');
if (manifest.id === "dev.lithe" || manifest.id.startsWith("dev.lithe.")) problems.push("dev.lithe.* is reserved for official plugins");
if (manifest.backend?.launch?.kind !== "packaged" || manifest.backend.launch.path !== BACKEND_PATH) {
  problems.push(`plugin.json backend.launch must be {"kind": "packaged", "path": "${BACKEND_PATH}"}`);
}
const cargoVersion = /^\[package\][^[]*?^version = "([^"]+)"/ms.exec(readFileSync("backend/Cargo.toml", "utf8"))?.[1];
if (cargoVersion !== manifest.version) problems.push(`backend/Cargo.toml version ${cargoVersion} must equal plugin.json version ${manifest.version}`);
if (values["expect-tag"] !== undefined && values["expect-tag"] !== `v${manifest.version}`) {
  problems.push(`tag ${values["expect-tag"]} does not match plugin.json version ${manifest.version} (expected v${manifest.version})`);
}
for (const platform of platforms) {
  const spec = PLATFORMS[platform];
  if (!spec) {
    problems.push(`unknown platform ${platform}; use ${Object.keys(PLATFORMS).join(", ")}`);
    continue;
  }
  for (const file of [spec.program, "THIRD_PARTY_LICENSES.txt"]) {
    if (!existsSync(join(".artifacts", `backend-${platform}`, file))) {
      problems.push(`.artifacts/backend-${platform}/${file} is missing: run node scripts/build-backend.mjs ${platform} on ${spec.builtOn}`);
    }
  }
}
if (problems.length > 0) {
  for (const problem of problems) console.error(`✗ ${problem}`);
  process.exit(1);
}

rmSync("dist", { recursive: true, force: true });
mkdirSync("dist", { recursive: true });
const staging = resolve(".artifacts", "package");
for (const platform of platforms) {
  const { program } = PLATFORMS[platform];
  const built = join(".artifacts", `backend-${platform}`);
  rmSync(staging, { recursive: true, force: true });
  const root = join(staging, PACKAGE_DIR);
  cpSync(PACKAGE_DIR, root, { recursive: true, filter: (path) => !path.endsWith(".DS_Store") });
  mkdirSync(join(root, "bin"), { recursive: true });
  cpSync(join(built, program), join(root, "bin", program));
  // A downloaded build artifact loses its mode, and the archive keeps this one.
  chmodSync(join(root, "bin", program), 0o755);
  cpSync(join(built, "THIRD_PARTY_LICENSES.txt"), join(root, "THIRD_PARTY_LICENSES.txt"));
  if (program !== "backend") {
    const forPlatform = JSON.parse(source);
    forPlatform.backend.launch.path = `bin/${program}`;
    writeFileSync(join(root, "plugin.json"), `${JSON.stringify(forPlatform, null, 2)}\n`);
  }
  const name = `${ASSET_PREFIX}-${manifest.version}-${platform}.zip`;
  // -X drops extra file attributes; -y stores a symbolic link as a link so
  // Lithe refuses it instead of the archive silently carrying its target.
  execFileSync("zip", ["-qrXy", resolve("dist", name), PACKAGE_DIR], { cwd: staging, timeout: 60_000 });
  const sha256 = createHash("sha256").update(readFileSync(resolve("dist", name))).digest("hex");
  console.log(`dist/${name} ${sha256}`);
}
rmSync(staging, { recursive: true, force: true });
