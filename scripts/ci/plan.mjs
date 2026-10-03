import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { appendFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const MACHINERY = ['.github/workflows/ci.yml', '.github/actions/', 'scripts/ci/'];

const NOT_CODE = [
  'docs/',
  '.claude/',
  'AGENTS.md',
  'CONTRIBUTING.md',
  'README.md',
  'skills-lock.json',
  'sonar-project.properties',
  '.github/pull_request_template.md',
  '.github/workflows/production-monitor.yml',
];

/**
 * SETS names the files each kind of job may read. A rule ending in `/` is a
 * directory and any other rule is one file; the longest rule that matches a
 * path decides, and `base` decides where none does. A job checks out exactly
 * its set, so a test that reads a file outside it fails instead of being
 * skipped on a change to that file.
 */
export const SETS = {
  deck: { base: 'none', include: ['docs/design/', 'scripts/check-design-deck.mjs'], exclude: [] },
  server: { base: 'all', include: ['web/tests/e2e/fixtures/'], exclude: [...NOT_CODE, 'web/'] },
  web: {
    base: 'all',
    include: ['server/internal/modules/classes/domain/joincode.go'],
    exclude: [...NOT_CODE, 'server/', 'migrations/', 'seed/', 'docker/'],
  },
  code: { base: 'all', include: [], exclude: NOT_CODE },
};

/**
 * JOBS lists every job the gate waits for, with the set it reads. Its ids are
 * the job ids in .github/workflows/ci.yml; `shards` splits a job into that many
 * matrix legs, each proven separately.
 */
export const JOBS = [
  { id: 'deck', set: 'deck' },
  { id: 'contract', set: 'code' },
  { id: 'server-lint', set: 'server' },
  { id: 'server-test', set: 'server' },
  { id: 'web-check', set: 'web' },
  { id: 'web-unit', set: 'web', shards: 2 },
  { id: 'e2e', set: 'web' },
  { id: 'e2e-live', set: 'code' },
];

const RELEASE_REF = 'refs/heads/main';

function ruleMatches(rule, path) {
  return rule.endsWith('/') ? path.startsWith(rule) : path === rule;
}

function includesOf(set) {
  return [...set.include, ...MACHINERY];
}

/** inSet reports whether a repository path belongs to a set. */
export function inSet(set, path) {
  let verdict = set.base === 'all';
  let longest = -1;
  for (const [rules, included] of [
    [set.exclude, false],
    [includesOf(set), true],
  ]) {
    for (const rule of rules) {
      if (rule.length > longest && ruleMatches(rule, path)) {
        longest = rule.length;
        verdict = included;
      }
    }
  }
  return verdict;
}

/**
 * sparsePatterns renders a set as non-cone sparse-checkout patterns, shortest
 * rule first, so that git selects the same paths as inSet.
 */
export function sparsePatterns(set) {
  const rules = [
    ...set.exclude.map((rule) => ({ rule, prefix: '!/' })),
    ...includesOf(set).map((rule) => ({ rule, prefix: '/' })),
  ].sort((a, b) => a.rule.length - b.rule.length || a.rule.localeCompare(b.rule));
  return [...(set.base === 'all' ? ['/*'] : []), ...rules.map(({ rule, prefix }) => prefix + rule)];
}

/** parseTree reads the output of `git ls-tree -r -z` into mode, sha and path. */
export function parseTree(raw) {
  return raw
    .split('\0')
    .filter(Boolean)
    .map((line) => {
      const tab = line.indexOf('\t');
      const [mode, , sha] = line.slice(0, tab).split(' ');
      return { mode, sha, path: line.slice(tab + 1) };
    });
}

/**
 * hashOf digests the mode, blob and path of every tree entry in a set. Two
 * commits give a set the same hash exactly when the set's files are identical.
 */
export function hashOf(set, entries) {
  const digest = createHash('sha256').update('quizzivy-ci-1\n');
  for (const entry of entries) {
    if (inSet(set, entry.path)) digest.update(`${entry.mode} ${entry.sha} ${entry.path}\n`);
  }
  return digest.digest('hex').slice(0, 20);
}

/** markersOf names the artifacts that record a pass of a job on a hash, one per shard. */
export function markersOf(job, hash) {
  if (!job.shards) return [`ci-pass-${job.id}-${hash}`];
  return Array.from({ length: job.shards }, (_, i) => `ci-pass-${job.id}-${i + 1}of${job.shards}-${hash}`);
}

/**
 * trusted reports whether an artifact can stand as proof: it has not expired
 * and the run that uploaded it was on a branch of this repository, not a fork.
 */
export function trusted(artifact) {
  const run = artifact.workflow_run;
  return (
    artifact.expired === false &&
    Boolean(run) &&
    typeof run.repository_id === 'number' &&
    run.repository_id === run.head_repository_id
  );
}

/**
 * isFresh reports whether a run must ignore earlier proof: a re-run of all
 * jobs asks for exactly that, and a push to main is what the deploy ships.
 */
export function isFresh({ event, ref, attempt }) {
  return attempt > 1 || (event === 'push' && ref === RELEASE_REF);
}

/**
 * plan decides, per job, whether it runs. A job is skipped only when every one
 * of its markers is found, which `lookup` answers with the run that uploaded
 * it, or null. A lookup that fails counts as no proof.
 */
export async function plan({ entries, fresh, lookup }) {
  const jobs = {};
  for (const job of JOBS) {
    const set = SETS[job.set];
    const hash = hashOf(set, entries);
    const markers = markersOf(job, hash);
    const proofs = fresh ? [] : await Promise.all(markers.map((name) => lookup(name).catch(() => null)));
    const proven = proofs.length === markers.length && proofs.every((run) => run !== null);
    jobs[job.id] = {
      run: !proven,
      set: job.set,
      hash,
      markers,
      shards: markers.map((_, i) => i + 1),
      proof: proven ? proofs[0] : null,
      sparse: sparsePatterns(set).join('\n'),
    };
  }
  return { jobs };
}

function artifactLookup({ api, repository, token }) {
  return async (name) => {
    const url = `${api}/repos/${repository}/actions/artifacts?per_page=100&name=${encodeURIComponent(name)}`;
    const response = await fetch(url, {
      headers: {
        accept: 'application/vnd.github+json',
        authorization: `Bearer ${token}`,
        'x-github-api-version': '2022-11-28',
      },
    });
    if (!response.ok) throw new Error(`${response.status} listing ${name}`);
    const found = (await response.json()).artifacts.find(trusted);
    return found ? found.workflow_run.id : null;
  };
}

function summaryOf(result, { fresh, server, repository }) {
  const rows = Object.entries(result.jobs).map(([id, job]) => {
    const decision = job.run
      ? 'run'
      : `skip, passed in [${job.proof}](${server}/${repository}/actions/runs/${job.proof})`;
    return `| ${id} | ${job.set} | \`${job.hash}\` | ${decision} |`;
  });
  return [
    '### CI plan',
    '',
    fresh ? 'Every job runs: earlier passes are not used for this run.' : 'A job is skipped when the same files already passed it.',
    '',
    '| Job | Reads | Hash | Decision |',
    '|---|---|---|---|',
    ...rows,
    '',
  ].join('\n');
}

async function main() {
  const env = process.env;
  const entries = parseTree(execFileSync('git', ['ls-tree', '-r', '-z', 'HEAD'], { encoding: 'utf8', maxBuffer: 1 << 28 }));
  const fresh = isFresh({ event: env.GITHUB_EVENT_NAME, ref: env.GITHUB_REF, attempt: Number(env.GITHUB_RUN_ATTEMPT ?? 1) });
  const lookup = artifactLookup({ api: env.GITHUB_API_URL, repository: env.GITHUB_REPOSITORY, token: env.GITHUB_TOKEN });
  const result = await plan({ entries, fresh, lookup });
  const summary = summaryOf(result, { fresh, server: env.GITHUB_SERVER_URL, repository: env.GITHUB_REPOSITORY });
  if (env.GITHUB_OUTPUT) appendFileSync(env.GITHUB_OUTPUT, `plan=${JSON.stringify(result)}\n`);
  if (env.GITHUB_STEP_SUMMARY) appendFileSync(env.GITHUB_STEP_SUMMARY, summary);
  process.stdout.write(summary);
}

if (process.argv[1] === fileURLToPath(import.meta.url)) await main();
