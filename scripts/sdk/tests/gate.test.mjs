// make sdk-check end to end: a throwaway repository whose base moved on after
// the branch point, and a stub oasdiff that records what it was asked to diff.
import assert from 'node:assert/strict';
import { execFileSync, spawnSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { after, test } from 'node:test';

const CHECK = resolve('scripts/sdk/check.mjs');
const root = mkdtempSync(join(tmpdir(), 'sdk-gate-'));
after(() => rmSync(root, { recursive: true, force: true }));

// The stub prints its findings and exits 1 for `breaking` when STUB_BREAKING
// is set, as oasdiff --fail-on ERR does; it logs the base file it was given.
const STUB = join(root, 'oasdiff.mjs');
writeFileSync(
  STUB,
  `import { appendFileSync, readFileSync } from 'node:fs';
const [command, base] = process.argv.slice(2);
appendFileSync(process.env.STUB_LOG, command + ' ' + readFileSync(base, 'utf8').match(/marker: (\\w+)/)[1] + '\\n');
if (process.env.STUB_FAIL) process.exit(121);
if (command === 'breaking' && process.env.STUB_BREAKING) { console.log('- removed GET /x'); process.exit(1); }
`,
);

const contract = (version, marker) => `openapi: 3.1.0\ninfo:\n  title: T\n  version: ${version}\n  x-note:\n    marker: ${marker}\npaths: {}\n`;

let counter = 0;
function repo({ baseVersion = '0.13.2', headVersion = '0.13.2', web = '0.13.2', change = true } = {}) {
  const dir = join(root, `r${counter++}`);
  mkdirSync(join(dir, 'api'), { recursive: true });
  mkdirSync(join(dir, 'web'));
  const git = (...args) => execFileSync('git', args, { cwd: dir, encoding: 'utf8' });
  const commit = (message) => {
    git('add', '-A');
    git('-c', 'user.name=t', '-c', 'user.email=t@t', 'commit', '-q', '--allow-empty', '-m', message);
  };
  git('init', '-q', '-b', 'develop');
  writeFileSync(join(dir, 'api/openapi.yaml'), contract(baseVersion, 'branchpoint'));
  writeFileSync(join(dir, 'web/package.json'), JSON.stringify({ version: web }));
  commit('branch point');
  git('checkout', '-q', '-b', 'feature');
  if (change) writeFileSync(join(dir, 'api/openapi.yaml'), contract(headVersion, 'head'));
  writeFileSync(join(dir, 'web/package.json'), JSON.stringify({ version: web }));
  commit('feature');
  // The base moves on after the branch point: diffing against its tip would
  // read this as the feature removing it.
  git('checkout', '-q', 'develop');
  writeFileSync(join(dir, 'api/openapi.yaml'), contract(baseVersion, 'basetip'));
  commit('base moves on');
  git('checkout', '-q', 'feature');
  return dir;
}

function gate(dir, env = {}) {
  const log = join(dir, 'stub.log');
  writeFileSync(log, '');
  const result = spawnSync(process.execPath, [CHECK], {
    cwd: dir,
    encoding: 'utf8',
    env: { ...process.env, GITHUB_STEP_SUMMARY: '', SDK_BASE: 'develop', OASDIFF: `${process.execPath} ${STUB}`, STUB_LOG: log, ...env },
  });
  return { ...result, calls: readFileSync(log, 'utf8').trim().split('\n').filter(Boolean) };
}

test('it diffs against the merge-base, not the base tip', () => {
  const run = gate(repo());
  assert.equal(run.status, 0, run.stderr);
  assert.deepEqual(run.calls, ['breaking branchpoint', 'changelog branchpoint']);
  assert.match(run.stdout, /Nothing breaks the SDK rules/);
});

test('an unchanged contract runs no oasdiff', () => {
  const run = gate(repo({ change: false }));
  assert.equal(run.status, 0);
  assert.deepEqual(run.calls, []);
  assert.match(run.stdout, /unchanged since the merge-base/);
});

test('a breaking change without the label fails only when the gate blocks', () => {
  const dir = repo();
  const report = gate(dir, { STUB_BREAKING: '1' });
  assert.equal(report.status, 0);
  assert.match(report.stdout, /::warning title=SDK contract check::.*contract-breaking/);
  const block = gate(dir, { STUB_BREAKING: '1', SDK_GATE: 'block' });
  assert.equal(block.status, 1);
  assert.match(block.stdout, /::error title=SDK contract check::/);
  const labelled = gate(dir, { STUB_BREAKING: '1', SDK_GATE: 'block', CONTRACT_BREAKING: 'true' });
  assert.equal(labelled.status, 0, labelled.stdout);
});

test('the release rules apply only with SDK_RELEASE', () => {
  const dir = repo({ headVersion: '0.13.2', web: '0.13.3' });
  assert.equal(gate(dir, { SDK_GATE: 'block' }).status, 0);
  const release = gate(dir, { SDK_GATE: 'block', SDK_RELEASE: 'true' });
  assert.equal(release.status, 1);
  assert.match(release.stdout, /must equal web\/package.json/);
  assert.match(release.stdout, /still 0\.13\.2/);
  const good = gate(repo({ headVersion: '0.13.3', web: '0.13.3' }), { SDK_GATE: 'block', SDK_RELEASE: 'true' });
  assert.equal(good.status, 0, good.stdout);
});

test('a tool or base failure warns while report-only and fails when blocking', () => {
  const dir = repo();
  const tool = gate(dir, { STUB_FAIL: '1' });
  assert.equal(tool.status, 0);
  assert.match(tool.stdout, /::warning title=SDK contract check::could not run: oasdiff breaking exited 121/);
  assert.notEqual(gate(dir, { STUB_FAIL: '1', SDK_GATE: 'block' }).status, 0);
  const base = gate(dir, { SDK_BASE: 'nope' });
  assert.equal(base.status, 0);
  assert.match(base.stdout, /could not run: no merge-base with nope/);
  assert.notEqual(gate(dir, { SDK_BASE: 'nope', SDK_GATE: 'block' }).status, 0);
});
