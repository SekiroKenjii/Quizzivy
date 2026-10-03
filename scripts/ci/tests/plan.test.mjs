import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test } from 'node:test';

import { JOBS, SETS, hashOf, inSet, isFresh, markersOf, parseTree, plan, sparsePatterns, trusted } from '../plan.mjs';

const git = (args, input) => execFileSync('git', args, { encoding: 'utf8', maxBuffer: 1 << 28, input });
const tracked = parseTree(git(['ls-tree', '-r', '-z', 'HEAD']));
const paths = tracked.map((entry) => entry.path);

const rulesOf = (set) => sparsePatterns(set).filter((pattern) => pattern !== '/*');
const entry = (path, sha = 'a'.repeat(40)) => ({ mode: '100644', sha, path });
const found = (run) => async () => run;

test('a rule is a plain path that git and inSet read the same way', () => {
  for (const [name, set] of Object.entries(SETS)) {
    for (const rule of [...set.include, ...set.exclude]) {
      assert.match(rule, /^[A-Za-z0-9._][A-Za-z0-9._/-]*$/, `${name}: ${rule}`);
    }
    const both = set.include.filter((rule) => set.exclude.includes(rule));
    assert.deepEqual(both, [], `${name} both includes and excludes a rule`);
  }
});

test('every rule still names something in the repository', () => {
  for (const [name, set] of Object.entries(SETS)) {
    for (const pattern of rulesOf(set)) {
      const rule = pattern.replace(/^!?\//, '');
      const hit = paths.some((path) => (rule.endsWith('/') ? path.startsWith(rule) : path === rule));
      assert.ok(hit, `${name}: ${rule} matches no tracked file`);
    }
  }
});

test('every job reads a set that exists', () => {
  for (const job of JOBS) assert.ok(SETS[job.set], job.id);
  assert.equal(new Set(JOBS.map((job) => job.id)).size, JOBS.length);
});

test('the longest rule decides, and the base decides where none matches', () => {
  assert.equal(inSet(SETS.web, 'web/src/main.tsx'), true);
  assert.equal(inSet(SETS.web, 'api/openapi.yaml'), true);
  assert.equal(inSet(SETS.web, 'server/cmd/api/main.go'), false);
  assert.equal(inSet(SETS.web, 'server/internal/modules/classes/domain/joincode.go'), true);
  assert.equal(inSet(SETS.web, 'server/internal/modules/classes/domain/joincode.go.bak'), false);
  assert.equal(inSet(SETS.web, 'migrations/00001_init.sql'), false);
  assert.equal(inSet(SETS.server, 'web/src/main.tsx'), false);
  assert.equal(inSet(SETS.server, 'web/tests/e2e/fixtures/unit5-listening.mp3'), true);
  assert.equal(inSet(SETS.server, 'fly.toml'), true);
  assert.equal(inSet(SETS.server, '.github/workflows/deploy.yml'), true);
  assert.equal(inSet(SETS.code, 'docs/plan/73-r3.md'), false);
  assert.equal(inSet(SETS.code, 'AGENTS.md'), false);
  assert.equal(inSet(SETS.code, 'server/README.md'), true);
  assert.equal(inSet(SETS.deck, 'docs/design/deck/support.js'), true);
  assert.equal(inSet(SETS.deck, 'docs/plan/73-r3.md'), false);
  assert.equal(inSet(SETS.deck, 'web/src/main.tsx'), false);
});

test('every set reads the workflow, its actions and these scripts', () => {
  for (const [name, set] of Object.entries(SETS)) {
    for (const path of ['.github/workflows/ci.yml', '.github/actions/minio/action.yml', 'scripts/ci/plan.mjs']) {
      assert.equal(inSet(set, path), true, `${name}: ${path}`);
    }
  }
});

test('git checks out exactly the paths inSet selects', () => {
  const dir = mkdtempSync(join(tmpdir(), 'ci-plan-'));
  try {
    for (const [name, set] of Object.entries(SETS)) {
      const rules = join(dir, `${name}.rules`);
      writeFileSync(rules, sparsePatterns(set).join('\n') + '\n');
      const byGit = git(['sparse-checkout', 'check-rules', '--no-cone', '-z', '--rules-file', rules], paths.join('\0') + '\0')
        .split('\0')
        .filter(Boolean)
        .sort();
      const byRule = paths.filter((path) => inSet(set, path)).sort();
      assert.deepEqual(byGit, byRule, name);
      assert.ok(byRule.length > 0, `${name} selects nothing`);
    }
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test('a hash moves only when a file of the set changes', () => {
  const before = [entry('server/a.go'), entry('web/src/a.ts'), entry('docs/plan/a.md'), entry('web/tests/e2e/fixtures/a.mp3')];
  const change = (path) => before.map((e) => (e.path === path ? entry(path, 'b'.repeat(40)) : e));

  assert.equal(hashOf(SETS.server, change('web/src/a.ts')), hashOf(SETS.server, before));
  assert.equal(hashOf(SETS.server, change('docs/plan/a.md')), hashOf(SETS.server, before));
  assert.notEqual(hashOf(SETS.server, change('server/a.go')), hashOf(SETS.server, before));
  assert.notEqual(hashOf(SETS.server, change('web/tests/e2e/fixtures/a.mp3')), hashOf(SETS.server, before));
  assert.equal(hashOf(SETS.web, change('server/a.go')), hashOf(SETS.web, before));
  assert.notEqual(hashOf(SETS.web, change('web/src/a.ts')), hashOf(SETS.web, before));
  assert.equal(hashOf(SETS.code, change('docs/plan/a.md')), hashOf(SETS.code, before));
  assert.notEqual(hashOf(SETS.code, [...before, entry('server/b.go')]), hashOf(SETS.code, before));
  assert.notEqual(
    hashOf(SETS.code, before.map((e) => (e.path === 'server/a.go' ? { ...e, mode: '100755' } : e))),
    hashOf(SETS.code, before),
  );
});

test('a renamed file changes the hash although its content does not', () => {
  assert.notEqual(hashOf(SETS.server, [entry('server/a.go')]), hashOf(SETS.server, [entry('server/b.go')]));
});

test('a sharded job has one marker per shard', () => {
  assert.deepEqual(markersOf({ id: 'deck' }, 'abc'), ['ci-pass-deck-abc']);
  assert.deepEqual(markersOf({ id: 'web-unit', shards: 2 }, 'abc'), [
    'ci-pass-web-unit-1of2-abc',
    'ci-pass-web-unit-2of2-abc',
  ]);
});

test('proof from a fork or from an expired artifact is not trusted', () => {
  const run = { id: 7, repository_id: 1, head_repository_id: 1 };
  assert.equal(trusted({ expired: false, workflow_run: run }), true);
  assert.equal(trusted({ expired: true, workflow_run: run }), false);
  assert.equal(trusted({ expired: false, workflow_run: { ...run, head_repository_id: 2 } }), false);
  assert.equal(trusted({ expired: false, workflow_run: null }), false);
  assert.equal(trusted({ expired: false, workflow_run: { id: 7 } }), false);
  assert.equal(trusted({ workflow_run: run }), false);
});

test('a re-run and a push to main ignore earlier proof', () => {
  assert.equal(isFresh({ event: 'pull_request', ref: 'refs/pull/1/merge', attempt: 1 }), false);
  assert.equal(isFresh({ event: 'push', ref: 'refs/heads/develop', attempt: 1 }), false);
  assert.equal(isFresh({ event: 'push', ref: 'refs/heads/main', attempt: 1 }), true);
  assert.equal(isFresh({ event: 'pull_request', ref: 'refs/pull/1/merge', attempt: 2 }), true);
});

test('a job is skipped only when every marker has proof', async () => {
  const all = await plan({ entries: tracked, fresh: false, lookup: found(42) });
  for (const job of JOBS) {
    assert.equal(all.jobs[job.id].run, false, job.id);
    assert.equal(all.jobs[job.id].proof, 42, job.id);
  }

  const none = await plan({ entries: tracked, fresh: false, lookup: found(null) });
  for (const job of JOBS) assert.equal(none.jobs[job.id].run, true, job.id);

  const oneShard = await plan({
    entries: tracked,
    fresh: false,
    lookup: async (name) => (name.includes('-2of2-') ? null : 42),
  });
  assert.equal(oneShard.jobs['web-unit'].run, true);
  assert.equal(oneShard.jobs['web-unit'].proof, null);
  assert.equal(oneShard.jobs['web-check'].run, false);
});

test('a lookup that fails runs the job', async () => {
  const result = await plan({
    entries: tracked,
    fresh: false,
    lookup: async () => {
      throw new Error('503');
    },
  });
  for (const job of JOBS) assert.equal(result.jobs[job.id].run, true, job.id);
});

test('a fresh run looks nothing up and runs everything', async () => {
  let calls = 0;
  const result = await plan({
    entries: tracked,
    fresh: true,
    lookup: async () => {
      calls += 1;
      return 42;
    },
  });
  assert.equal(calls, 0);
  for (const job of JOBS) assert.equal(result.jobs[job.id].run, true, job.id);
});

test('jobs on one set share a hash and a checkout, and name their own markers', async () => {
  const result = await plan({ entries: tracked, fresh: false, lookup: found(null) });
  assert.equal(result.jobs['server-lint'].hash, result.jobs['server-test'].hash);
  assert.equal(result.jobs['server-lint'].sparse, result.jobs['server-test'].sparse);
  assert.notEqual(result.jobs['server-lint'].markers[0], result.jobs['server-test'].markers[0]);
  assert.notEqual(result.jobs['server-test'].hash, result.jobs['web-check'].hash);
  assert.deepEqual(result.jobs['web-unit'].shards, [1, 2]);
  assert.match(result.jobs.deck.sparse, /^\/docs\/design\/$/m);
  assert.doesNotMatch(result.jobs.deck.sparse, /^\/\*$/m);
});
