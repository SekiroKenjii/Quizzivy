import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';

import { verdict } from '../gate.mjs';

const planned = { jobs: { deck: { run: false }, contract: { run: true } } };
const needs = (results) =>
  Object.fromEntries(Object.entries({ plan: 'success', ...results }).map(([id, result]) => [id, { result }]));

test('a run is green when each job did what the plan asked', () => {
  assert.deepEqual(verdict(needs({ deck: 'skipped', contract: 'success' }), planned), []);
});

test('a planned job that failed, was cancelled or was skipped fails the run', () => {
  for (const result of ['failure', 'cancelled', 'skipped']) {
    assert.deepEqual(verdict(needs({ deck: 'skipped', contract: result }), planned), [
      `contract: ${result}, expected success`,
    ]);
  }
});

test('a job that ran although the plan skipped it fails the run', () => {
  assert.deepEqual(verdict(needs({ deck: 'success', contract: 'success' }), planned), [
    'deck: success, expected skipped',
  ]);
});

test('a plan that did not succeed fails the run whatever the jobs did', () => {
  assert.equal(verdict(needs({ plan: 'failure', deck: 'skipped', contract: 'skipped' }), null).length, 1);
  assert.equal(verdict(needs({ deck: 'skipped', contract: 'success' }), null).length, 1);
  assert.equal(verdict({}, planned).length, 1);
});

test('the workflow and the plan must name the same jobs', () => {
  assert.deepEqual(verdict(needs({ deck: 'skipped', contract: 'success', extra: 'success' }), planned), [
    'extra: in the workflow but not in scripts/ci/plan.mjs',
  ]);
  assert.deepEqual(verdict(needs({ deck: 'skipped' }), planned), [
    'contract: in scripts/ci/plan.mjs but the gate does not wait for it',
  ]);
});

test('the gate exits with a failure exactly when the run is not green', () => {
  const gate = fileURLToPath(new URL('../gate.mjs', import.meta.url));
  const run = (results) =>
    spawnSync(process.execPath, [gate], {
      encoding: 'utf8',
      env: {
        PATH: process.env.PATH,
        NEEDS: JSON.stringify({
          ...needs(results),
          plan: { result: 'success', outputs: { plan: JSON.stringify(planned) } },
        }),
      },
    });

  const green = run({ deck: 'skipped', contract: 'success' });
  assert.equal(green.status, 0, green.stdout + green.stderr);

  const red = run({ deck: 'skipped', contract: 'failure' });
  assert.equal(red.status, 1);
  assert.match(red.stdout, /::error::contract: failure, expected success/);

  const noPlan = spawnSync(process.execPath, [gate], {
    encoding: 'utf8',
    env: { PATH: process.env.PATH, NEEDS: JSON.stringify(needs({ plan: 'failure', deck: 'skipped', contract: 'skipped' })) },
  });
  assert.equal(noPlan.status, 1);
});
