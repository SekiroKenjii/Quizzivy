import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';

const tracked = execFileSync('git', ['ls-files', '-z'], { encoding: 'utf8', maxBuffer: 1 << 28 })
  .split('\0')
  .filter(Boolean);

const WEB_TESTS = /^web\/tests\/.+\.tsx?$/;
const GO_TESTS = /^server\/.+_test\.go$/;

const DISABLED_WEB =
  /\b(?:(?:it|test|describe|suite|bench)\.(?:skip|skipIf|todo|fixme|fails|only)|(?:ctx|context|task)\.skip|x(?:it|test|describe)|f(?:it|describe))\s*\(/g;

const GO_SKIP = /\b\w+\.(?:Skip|Skipf|SkipNow)\(\s*("(?:[^"\\]|\\.)*"|`[^`]*`)?/g;

const NAMES_A_PREREQUISITE = /\b(?:TEST_|S3_|IMPORT_)[A-Z_]+|\bAPP_DATABASE_URL\b/;

const ENVIRONMENT_SKIPS = [
  'RSS measurement is Linux-specific',
  'VmRSS not reported',
  'no /proc/self/status',
  'private MinIO required',
  'isolated local converter image required',
  'runtime deliberately rejects root workers',
  'could not take the table lock within 5s',
  "the centre's rule is not to take requests",
];

const disabledInWeb = (source) => [...source.matchAll(DISABLED_WEB)].map((m) => m[0]);

const unapprovedGoSkips = (source) =>
  [...source.matchAll(GO_SKIP)]
    .map((m) => ({ call: m[0], message: m[1]?.slice(1, -1) ?? '' }))
    .filter(
      ({ message }) =>
        !NAMES_A_PREREQUISITE.test(message) && !ENVIRONMENT_SKIPS.some((known) => message.startsWith(known)),
    );

const REFUSAL =
  "A failing test is fixed or put to the owner, never skipped (AGENTS.md). A Go skip that names the variable it waits for is allowed. Any other is a test that stops running somewhere, so it takes the owner's word and an entry in ENVIRONMENT_SKIPS.";

test('no web test is skipped, marked todo, focused or expected to fail', () => {
  const found = tracked
    .filter((path) => WEB_TESTS.test(path))
    .flatMap((path) => disabledInWeb(readFileSync(path, 'utf8')).map((call) => `${path}: ${call}`));
  assert.deepEqual(found, [], REFUSAL);
});

test('every Go skip waits for something the machine lacks, and says what', () => {
  const found = tracked
    .filter((path) => GO_TESTS.test(path))
    .flatMap((path) => unapprovedGoSkips(readFileSync(path, 'utf8')).map(({ call }) => `${path}: ${call}`));
  assert.deepEqual(found, [], REFUSAL);
});

test('the scanners refuse what they are there to refuse', () => {
  for (const source of [
    "it.skip('a', () => {})",
    "describe.skipIf(ci)('a', () => {})",
    "test.fixme('a', async () => {})",
    "it.todo('a')",
    "it.only('a', () => {})",
    "xit('a', () => {})",
    "it('a', (ctx) => { ctx.skip() })",
  ]) {
    assert.equal(disabledInWeb(source).length, 1, source);
  }
  assert.equal(disabledInWeb("it('skips a blank', () => { expect(skip(1)).toBe(2) })").length, 0);

  assert.equal(unapprovedGoSkips('t.Skip("flaky on CI")').length, 1);
  assert.equal(unapprovedGoSkips('t.Skipf("not today: %v", err)').length, 1);
  assert.equal(unapprovedGoSkips('t.SkipNow()').length, 1);
  assert.equal(unapprovedGoSkips('t.Skip("TEST_DATABASE_URL is not set")').length, 0);
  assert.equal(unapprovedGoSkips('t.Skip("private MinIO required")').length, 0);
});
