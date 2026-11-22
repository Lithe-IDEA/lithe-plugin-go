// Runs this plugin's host end-to-end tests against a pinned Lithe desktop host.
//
//   node scripts/e2e-host.mjs [--source <Lithe git URL or path>] [--ref <commit or branch> | --worktree <path>] [--run <regex>] [--remote]
//
// The tests in e2e/host/ drive the real plugin package through the desktop host's own test
// harness (an internal Go package), so they cannot be built in this repository. Each run checks
// out the host revision pinned in e2e/host/lithe-host.json into the disposable .artifacts/lithe-host
// (desktop/ and scripts/ only), maps every e2e/host/desktop/<path> test to its path in that
// checkout through a Go overlay and runs the host's scripts/desktop/desktop-go.sh test there, which builds the Core
// revision that host pins. The host repository never reads this repository: the tests live here
// and the pin says which host they last passed on. Bumping the pin can require adapting the tests
// to changes in the host's internal harness.
//
// Every variable listed in lithe-host.json "requiredEnvironment" must be set, because the tests
// skip without their plugin input and a skipped run proves nothing. "repositoryEnvironment" maps a
// variable to a path in this repository, used when the variable is not set. --source and --ref override
// the pin, for example to try an unmerged host branch; --run overrides the test selection.
// --worktree runs against an existing checkout with a Go test overlay, so it also supports
// uncommitted changes without copying plugin tests into the host source tree.
import { spawn } from 'node:child_process';
import { mkdir, mkdtemp, readFile, readdir, realpath, rm, writeFile } from 'node:fs/promises';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseArgs } from 'node:util';

const root = fileURLToPath(new URL('..', import.meta.url));
const tests = join(root, 'e2e', 'host');
const work = join(root, '.artifacts', 'lithe-host');

function run(command, args, options = {}) {
  return new Promise((resolvePromise, reject) => {
    const child = spawn(command, args, { stdio: 'inherit', ...options });
    child.on('error', reject);
    child.on('close', (code, signal) => {
      if (code === 0) resolvePromise();
      else reject(new Error(`${command} ${args.join(' ')} failed (${signal ?? `exit ${code}`})`));
    });
  });
}

/** Lists the files under e2e/host/desktop, relative to e2e/host. */
async function hostTestFiles(directory = join(tests, 'desktop')) {
  const files = [];
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) files.push(...await hostTestFiles(path));
    else if (entry.isFile()) files.push(relative(tests, path));
  }
  return files;
}

/** Checks out `ref` from `source` into the disposable work directory, desktop/ and scripts/ only. */
async function checkoutHost(source, ref) {
  await mkdir(work, { recursive: true });
  await run('git', ['init', '-q', work]);
  await run('git', ['-C', work, 'sparse-checkout', 'set', '--cone', 'desktop', 'scripts']);
  // A named remote, because the partial clone fetches missing blobs from it later.
  await run('git', ['-C', work, 'config', 'remote.origin.url', source]);
  await run('git', ['-C', work, 'fetch', '-q', '--depth', '1', '--filter=blob:none', 'origin', ref]);
  await run('git', ['-C', work, 'checkout', '-q', '--force', '--detach', 'FETCH_HEAD']);
  // Drops test files copied by an earlier run; ignored build state such as .artifacts/ stays.
  await run('git', ['-C', work, 'clean', '-q', '-f', '-d', '--', 'desktop']);
}

const { values } = parseArgs({
  options: {
    source: { type: 'string' },
    ref: { type: 'string' },
    worktree: { type: 'string' },
    run: { type: 'string' },
    remote: { type: 'boolean' },
  },
});
const pin = JSON.parse(await readFile(join(tests, 'lithe-host.json'), 'utf8'));
const remote = pin.remoteSSH;
if (values.worktree && (values.source || values.ref)) {
  throw new Error('Use --worktree on its own, or select a revision with --source and --ref.');
}
if (values.remote && !values.ref && !values.worktree) {
  throw new Error('Pass --ref or --worktree with a Lithe host that supports remote plugins.');
}
const required = { ...pin.requiredEnvironment, ...(values.remote ? remote.requiredEnvironment : {}) };
for (const [name, path] of Object.entries(pin.repositoryEnvironment ?? {})) {
  process.env[name] ||= join(root, path);
}
const missing = Object.entries(required).filter(([name]) => !process.env[name]);
if (missing.length > 0) {
  throw new Error(`Set the plugin inputs first:\n${missing.map(([name, what]) => `  ${name}: ${what}`).join('\n')}`);
}

let host;
if (values.worktree) {
  host = await realpath(values.worktree);
} else {
  // git runs inside the checkout, so a local source must be absolute.
  const given = values.source ?? pin.repository;
  const source = !/^[a-z]+:\/\//.test(given) ? resolve(given) : given;
  await checkoutHost(source, values.ref ?? pin.revision);
  host = await realpath(work);
}

const artifacts = join(root, '.artifacts');
await mkdir(artifacts, { recursive: true });
const overlayDirectory = await mkdtemp(join(artifacts, 'lithe-host-overlay-'));
try {
  const overlay = join(overlayDirectory, 'overlay.json');
  const Replace = Object.fromEntries((await hostTestFiles()).map(file => [join(host, file), join(tests, file)]));
  await writeFile(overlay, JSON.stringify({ Replace }));

  const packages = values.remote ? { [remote.package]: `^${remote.test}$` } : pin.packages;
  const tags = values.remote ? ['-tags', remote.tag] : [];
  for (const [pkg, selection] of Object.entries(packages)) {
    await run('bash', ['scripts/desktop/desktop-go.sh', 'test', pkg, '-overlay', overlay, '-count=1', '-v',
      '-timeout', pin.timeout ?? '30m', ...tags, '-run', values.run ?? selection], { cwd: host });
  }
} finally {
  await rm(overlayDirectory, { recursive: true, force: true });
}
