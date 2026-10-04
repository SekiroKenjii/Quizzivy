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

/**
 * outcomesOf names the artifacts that record how a job ended on a hash, one
 * pair per shard: `success` for a pass and `failure` for a failed run. The
 * keys are the values of `job.status`, so a cancelled job has no name to
 * record under.
 */
export function outcomesOf(job, hash) {
  const legs = job.shards ? Array.from({ length: job.shards }, (_, i) => `${i + 1}of${job.shards}-`) : [''];
  return legs.map((leg) => ({
    success: `ci-pass-${job.id}-${leg}${hash}`,
    failure: `ci-fail-${job.id}-${leg}${hash}`,
  }));
}

/**
 * trusted reports whether an artifact can stand as evidence: it has not
 * expired and the run that uploaded it was on a branch of this repository,
 * not a fork.
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
 * newest returns the most recent trusted artifact of a listing as its run and
 * the time it was uploaded, or null when the listing has none.
 */
export function newest(artifacts) {
  let found = null;
  for (const artifact of artifacts.filter(trusted)) {
    const at = Date.parse(artifact.created_at);
    if (Number.isFinite(at) && (found === null || at > found.at)) found = { run: artifact.workflow_run.id, at };
  }
  return found;
}

/**
 * isFresh reports whether a run must ignore earlier proof: a re-run of all
 * jobs asks for exactly that, and a push to main is what the deploy ships.
 * An attempt it cannot read as the first is treated as a re-run.
 */
export function isFresh({ event, ref, attempt }) {
  return attempt !== 1 || (event === 'push' && ref === RELEASE_REF);
}

/** freshnessOf reads isFresh's inputs from the environment GitHub gives a run. */
export function freshnessOf(env) {
  return isFresh({ event: env.GITHUB_EVENT_NAME, ref: env.GITHUB_REF, attempt: Number(env.GITHUB_RUN_ATTEMPT) });
}

/**
 * plan decides, per job, whether it runs. A job is skipped only when every
 * shard has a pass that is newer than any failure recorded for the same hash,
 * so the latest outcome on a set of files is the one that counts. `lookup`
 * answers a marker name with the newest trusted artifact of that name, or
 * null; a lookup that fails counts as no proof.
 */
export async function plan({ entries, fresh, lookup }) {
  const jobs = {};
  for (const job of JOBS) {
    const set = SETS[job.set];
    const hash = hashOf(set, entries);
    const outcomes = outcomesOf(job, hash);
    const proofs = fresh ? [] : await Promise.all(outcomes.map((names) => standing(names, lookup)));
    const proven = proofs.length === outcomes.length && proofs.every((run) => run !== null);
    jobs[job.id] = {
      run: !proven,
      set: job.set,
      hash,
      outcomes,
      shards: outcomes.map((_, i) => i + 1),
      proof: proven ? proofs[0] : null,
      sparse: sparsePatterns(set).join('\n'),
    };
  }
  return { jobs };
}

async function standing(names, lookup) {
  try {
    const [pass, fail] = await Promise.all([lookup(names.success), lookup(names.failure)]);
    return pass !== null && (fail === null || pass.at > fail.at) ? pass.run : null;
  } catch {
    return null;
  }
}

/**
 * artifactLookup answers a marker name from the repository's artifacts. It
 * reads the newest hundred of that name, which is more than thirty days of
 * runs on one hash can leave.
 */
export function artifactLookup({ api, repository, token, fetch: request = fetch }) {
  return async (name) => {
    const url = `${api}/repos/${repository}/actions/artifacts?per_page=100&name=${encodeURIComponent(name)}`;
    const response = await request(url, {
      headers: {
        accept: 'application/vnd.github+json',
        authorization: `Bearer ${token}`,
        'x-github-api-version': '2022-11-28',
      },
    });
    if (!response.ok) throw new Error(`${response.status} listing ${name}`);
    return newest((await response.json()).artifacts);
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
    fresh
      ? 'Every job runs: earlier passes are not used for this run.'
      : 'A job is skipped when the same files passed it and have not failed it since.',
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
  const fresh = freshnessOf(env);
  const lookup = artifactLookup({ api: env.GITHUB_API_URL, repository: env.GITHUB_REPOSITORY, token: env.GITHUB_TOKEN });
  const result = await plan({ entries, fresh, lookup });
  const summary = summaryOf(result, { fresh, server: env.GITHUB_SERVER_URL, repository: env.GITHUB_REPOSITORY });
  if (env.GITHUB_OUTPUT) appendFileSync(env.GITHUB_OUTPUT, `plan=${JSON.stringify(result)}\n`);
  if (env.GITHUB_STEP_SUMMARY) appendFileSync(env.GITHUB_STEP_SUMMARY, summary);
  process.stdout.write(summary);
}

if (process.argv[1] === fileURLToPath(import.meta.url)) await main();
