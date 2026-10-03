import assert from 'node:assert/strict';
import { test } from 'node:test';

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
