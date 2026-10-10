import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test } from 'node:test';

import {
  JOBS,
  SETS,
  artifactLookup,
  freshnessOf,
  hashOf,
  inSet,
  isFresh,
  newest,
  outcomesOf,
  parseTree,
  plan,
  sparsePatterns,
  trusted,
} from '../plan.mjs';

const git = (args, input) => execFileSync('git', args, { encoding: 'utf8', maxBuffer: 1 << 28, input });
const tracked = parseTree(git(['ls-tree', '-r', '-z', 'HEAD']));
const paths = tracked.map((entry) => entry.path);

const rulesOf = (set) => sparsePatterns(set).filter((pattern) => pattern !== '/*');
const nested = {
  base: 'none',
  include: ['server/', 'server/internal/modules/classes/domain/joincode.go'],
  exclude: ['server/internal/'],
};
const entry = (path, sha = 'a'.repeat(40)) => ({ mode: '100644', sha, path });
const at = (run, time) => (run === null ? null : { run, at: time });
const passes = (run) => async (name) => (name.startsWith('ci-pass-') ? at(run, 100) : null);
const sameRepo = { id: 7, repository_id: 1, head_repository_id: 1 };
const artifact = (created_at, over = {}) => ({ expired: false, created_at, workflow_run: sameRepo, ...over });

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
  assert.equal(inSet(nested, 'server/go.mod'), true);
  assert.equal(inSet(nested, 'server/internal/core/core.go'), false);
  assert.equal(inSet(nested, 'server/internal/modules/classes/domain/joincode.go'), true);
  assert.equal(inSet(nested, 'web/package.json'), false);
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
    for (const [name, set] of Object.entries({ ...SETS, nested })) {
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

test('the local scripts under scripts/dev move no hash, and no job reads them', () => {
  for (const [name, set] of Object.entries(SETS)) {
    assert.equal(inSet(set, 'scripts/dev/heavy.sh'), false, name);
  }
  const text = readFileSync(new URL('../../../.github/workflows/ci.yml', import.meta.url), 'utf8');
  assert.ok(!text.includes('scripts/dev'), 'ci.yml reads scripts/dev');
  for (const action of ['db-tools', 'minio', 'record', 'web']) {
    const body = readFileSync(new URL(`../../../.github/actions/${action}/action.yml`, import.meta.url), 'utf8');
    assert.ok(!body.includes('scripts/dev'), `${action} reads scripts/dev`);
  }
});

test('a renamed file changes the hash although its content does not', () => {
  assert.notEqual(hashOf(SETS.server, [entry('server/a.go')]), hashOf(SETS.server, [entry('server/b.go')]));
});

test('the workflow, the gate and the plan name the same jobs', () => {
  const text = readFileSync(new URL('../../../.github/workflows/ci.yml', import.meta.url), 'utf8');
  const jobs = text.slice(text.indexOf('\njobs:\n'));
  const ids = [...jobs.matchAll(/^  ([a-z0-9-]+):$/gm)].map((match) => match[1]);
  const needs = jobs.match(/^  result:\n(?:    .*\n)*?    needs: \[(.*)\]$/m)[1].split(', ');
  const planned = JOBS.map((job) => job.id);
  assert.deepEqual(ids, ['plan', ...planned, 'result', 'sonar']);
  assert.deepEqual(needs, ['plan', ...planned]);
  for (const id of planned) {
    const body = jobs.slice(jobs.indexOf(`\n  ${id}:\n`), jobs.indexOf('\n\n  #', jobs.indexOf(`\n  ${id}:\n`)));
    const key = /^[a-z0-9]+$/.test(id) ? `jobs.${id}` : `jobs['${id}']`;
    const entry = `fromJSON(needs.plan.outputs.plan).${key}`;
    assert.ok(body.includes(`if: ${entry}.run\n`), `${id} does not obey its own plan entry`);
    assert.ok(body.includes(`JOB: \${{ toJSON(${entry}) }}\n`), `${id} reads another job's entry`);
    assert.ok(body.includes('sparse-checkout: ${{ fromJSON(env.JOB).sparse }}\n'), `${id} does not check out its set`);
    assert.match(
      body,
      /uses: \.\/\.github\/actions\/record\n {8}if: always\(\)\n {8}with:\n {10}marker: \$\{\{ fromJSON\(env\.JOB\)\.outcomes\[(0|strategy\.job-index)\]\[job\.status\] \}\}$/,
      `${id} does not end by recording its outcome`,
    );
  }
});

test('a job names a pass and a failure per shard, keyed by job status', () => {
  assert.deepEqual(outcomesOf({ id: 'deck' }, 'abc'), [{ success: 'ci-pass-deck-abc', failure: 'ci-fail-deck-abc' }]);
  assert.deepEqual(outcomesOf({ id: 'web-unit', shards: 2 }, 'abc'), [
    { success: 'ci-pass-web-unit-1of2-abc', failure: 'ci-fail-web-unit-1of2-abc' },
    { success: 'ci-pass-web-unit-2of2-abc', failure: 'ci-fail-web-unit-2of2-abc' },
  ]);
  assert.equal(outcomesOf({ id: 'deck' }, 'abc')[0].cancelled, undefined);
});

test('evidence from a fork or from an expired artifact is not trusted', () => {
  assert.equal(trusted({ expired: false, workflow_run: sameRepo }), true);
  assert.equal(trusted({ expired: true, workflow_run: sameRepo }), false);
  assert.equal(trusted({ expired: false, workflow_run: { ...sameRepo, head_repository_id: 2 } }), false);
  assert.equal(trusted({ expired: false, workflow_run: null }), false);
  assert.equal(trusted({ expired: false, workflow_run: { id: 7 } }), false);
  assert.equal(trusted({ workflow_run: sameRepo }), false);
});

test('the newest trusted artifact is the one that speaks', () => {
  assert.equal(newest([]), null);
  assert.deepEqual(
    newest([
      artifact('2026-10-01T00:00:00Z', { workflow_run: { ...sameRepo, id: 1 } }),
      artifact('2026-10-03T00:00:00Z', { workflow_run: { ...sameRepo, id: 3 } }),
      artifact('2026-10-02T00:00:00Z', { workflow_run: { ...sameRepo, id: 2 } }),
    ]),
    { run: 3, at: Date.parse('2026-10-03T00:00:00Z') },
  );
  assert.deepEqual(
    newest([
      artifact('2026-10-03T00:00:00Z', { workflow_run: { ...sameRepo, id: 9, head_repository_id: 2 } }),
      artifact('2026-10-04T00:00:00Z', { expired: true }),
      artifact('2026-10-01T00:00:00Z'),
    ]),
    { run: 7, at: Date.parse('2026-10-01T00:00:00Z') },
  );
  assert.equal(newest([artifact('not a date')]), null);
});

test('the lookup asks for one name and answers with its newest trusted artifact', async () => {
  const asked = [];
  const request = async (url, init) => {
    asked.push({ url, authorization: init.headers.authorization });
    return {
      ok: true,
      json: async () => ({
        artifacts: [
          artifact('2026-10-03T00:00:00Z', { workflow_run: { ...sameRepo, id: 9, head_repository_id: 2 } }),
          artifact('2026-10-02T00:00:00Z', { workflow_run: { ...sameRepo, id: 5 } }),
        ],
      }),
    };
  };
  const lookup = artifactLookup({ api: 'https://api.example', repository: 'o/r', token: 't', fetch: request });
  assert.deepEqual(await lookup('ci-pass-deck-a b'), { run: 5, at: Date.parse('2026-10-02T00:00:00Z') });
  assert.deepEqual(asked, [
    { url: 'https://api.example/repos/o/r/actions/artifacts?per_page=100&name=ci-pass-deck-a%20b', authorization: 'Bearer t' },
  ]);

  const refused = artifactLookup({ api: 'a', repository: 'o/r', token: 't', fetch: async () => ({ ok: false, status: 503 }) });
  await assert.rejects(refused('x'), /503/);
});

test('a re-run and a push to main ignore earlier proof', () => {
  assert.equal(isFresh({ event: 'pull_request', ref: 'refs/pull/1/merge', attempt: 1 }), false);
  assert.equal(isFresh({ event: 'push', ref: 'refs/heads/develop', attempt: 1 }), false);
  assert.equal(isFresh({ event: 'push', ref: 'refs/heads/work/redesign-r3', attempt: 1 }), false);
  assert.equal(isFresh({ event: 'push', ref: 'refs/heads/main', attempt: 1 }), true);
  assert.equal(isFresh({ event: 'pull_request', ref: 'refs/heads/main', attempt: 1 }), false);
  assert.equal(isFresh({ event: 'pull_request', ref: 'refs/pull/1/merge', attempt: 2 }), true);
  assert.equal(isFresh({ event: 'pull_request', ref: 'refs/pull/1/merge', attempt: Number.NaN }), true);
});

test('freshness is read from the run attempt, the event and the ref GitHub sets', () => {
  const env = { GITHUB_EVENT_NAME: 'pull_request', GITHUB_REF: 'refs/pull/1/merge', GITHUB_RUN_ATTEMPT: '1' };
  assert.equal(freshnessOf(env), false);
  assert.equal(freshnessOf({ ...env, GITHUB_RUN_ATTEMPT: '2' }), true);
  assert.equal(freshnessOf({ ...env, GITHUB_RUN_ATTEMPT: undefined }), true);
  assert.equal(freshnessOf({ ...env, GITHUB_EVENT_NAME: 'push', GITHUB_REF: 'refs/heads/main' }), true);
  assert.equal(freshnessOf({ ...env, GITHUB_EVENT_NAME: 'push', GITHUB_REF: 'refs/heads/develop' }), false);
});

test('a job is skipped only when every shard has a pass', async () => {
  const all = await plan({ entries: tracked, fresh: false, lookup: passes(42) });
  for (const job of JOBS) {
    assert.equal(all.jobs[job.id].run, false, job.id);
    assert.equal(all.jobs[job.id].proof, 42, job.id);
  }

  const none = await plan({ entries: tracked, fresh: false, lookup: passes(null) });
  for (const job of JOBS) assert.equal(none.jobs[job.id].run, true, job.id);

  const oneShard = await plan({
    entries: tracked,
    fresh: false,
    lookup: async (name) => (name.startsWith('ci-pass-') && !name.includes('-2of2-') ? at(42, 100) : null),
  });
  assert.equal(oneShard.jobs['web-unit'].run, true);
  assert.equal(oneShard.jobs['web-unit'].proof, null);
  assert.equal(oneShard.jobs['web-check'].run, false);
});

test('a failure after a pass withdraws it, and a pass after a failure restores it', async () => {
  const outcome = (pass, fail) => async (name) => at(name.startsWith('ci-pass-') ? 42 : 43, name.startsWith('ci-pass-') ? pass : fail);

  const failedSince = await plan({ entries: tracked, fresh: false, lookup: outcome(100, 200) });
  for (const job of JOBS) assert.equal(failedSince.jobs[job.id].run, true, job.id);

  const passedSince = await plan({ entries: tracked, fresh: false, lookup: outcome(200, 100) });
  for (const job of JOBS) {
    assert.equal(passedSince.jobs[job.id].run, false, job.id);
    assert.equal(passedSince.jobs[job.id].proof, 42, job.id);
  }

  const together = await plan({ entries: tracked, fresh: false, lookup: outcome(100, 100) });
  for (const job of JOBS) assert.equal(together.jobs[job.id].run, true, job.id);

  const oneShardFailed = await plan({
    entries: tracked,
    fresh: false,
    lookup: async (name) => {
      if (name.startsWith('ci-pass-')) return at(42, 100);
      return name.includes('-1of2-') ? at(43, 200) : null;
    },
  });
  assert.equal(oneShardFailed.jobs['web-unit'].run, true);
  assert.equal(oneShardFailed.jobs.e2e.run, false);
});

test('a lookup that fails runs the job, whichever marker it was for', async () => {
  const broken = async () => {
    throw new Error('503');
  };
  const result = await plan({ entries: tracked, fresh: false, lookup: broken });
  for (const job of JOBS) assert.equal(result.jobs[job.id].run, true, job.id);

  const failureUnknown = await plan({
    entries: tracked,
    fresh: false,
    lookup: async (name) => (name.startsWith('ci-pass-') ? at(42, 100) : broken()),
  });
  for (const job of JOBS) assert.equal(failureUnknown.jobs[job.id].run, true, job.id);
});

test('a fresh run looks nothing up and runs everything', async () => {
  let calls = 0;
  const result = await plan({
    entries: tracked,
    fresh: true,
    lookup: async () => {
      calls += 1;
      return at(42, 100);
    },
  });
  assert.equal(calls, 0);
  for (const job of JOBS) assert.equal(result.jobs[job.id].run, true, job.id);
});

test('jobs on one set share a hash and a checkout, and name their own markers', async () => {
  const result = await plan({ entries: tracked, fresh: false, lookup: passes(null) });
  assert.equal(result.jobs['server-lint'].hash, result.jobs['server-test'].hash);
  assert.equal(result.jobs['server-lint'].sparse, result.jobs['server-test'].sparse);
  assert.notEqual(result.jobs['server-lint'].outcomes[0].success, result.jobs['server-test'].outcomes[0].success);
  assert.notEqual(result.jobs['server-test'].hash, result.jobs['web-check'].hash);
  assert.deepEqual(result.jobs['web-unit'].shards, [1, 2]);
  assert.equal(result.jobs['web-unit'].outcomes.length, 2);
  assert.match(result.jobs.deck.sparse, /^\/docs\/design\/$/m);
  assert.doesNotMatch(result.jobs.deck.sparse, /^\/\*$/m);
  const names = Object.values(result.jobs).flatMap((job) => job.outcomes.flatMap((o) => [o.success, o.failure]));
  assert.equal(new Set(names).size, names.length);
});
