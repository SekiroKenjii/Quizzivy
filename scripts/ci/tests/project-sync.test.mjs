import assert from 'node:assert/strict';
import { test } from 'node:test';

import { fieldsFromBody, fieldsFromLabels, option, plan, taskRefs } from '../project-sync.mjs';

const options = [
  'Todo',
  'In Progress',
  'Blocked',
  'Done',
  'P0 blocker',
  'P1 high',
  'v0.14.0 (R7)',
  'v0.15.0 (R8)',
  'v1.0.0 (R11)',
].map((name, i) => ({ id: String(i), name }));

test('a label value names an option by its whole name or its first words', () => {
  assert.equal(option(options, 'in-progress')?.name, 'In Progress');
  assert.equal(option(options, 'p1')?.name, 'P1 high');
  assert.equal(option(options, 'v0.14.0')?.name, 'v0.14.0 (R7)');
  assert.equal(option(options, 'v1.0')?.name, 'v1.0.0 (R11)');
  assert.equal(option(options, 'v0'), undefined, 'two options start with it');
  assert.equal(option(options, 'progress'), undefined);
  assert.equal(option(options, ''), undefined);
});

test('prefixed labels set fields and other labels are ignored', () => {
  assert.deepEqual(
    fieldsFromLabels([{ name: 'status:blocked' }, { name: 'next: owner' }, { name: 'bug' }, 'priority:p2']),
    {
      Status: 'blocked',
      'Next step by': 'owner',
      Priority: 'p2',
    },
  );
});

test('Thread and Plan ID lines set the text fields', () => {
  assert.deepEqual(fieldsFromBody('Intro\nThread: Fix Safari export button\r\nplan id:  R7-03 \nThreads: no'), {
    Thread: 'Fix Safari export button',
    'Plan ID': 'R7-03',
  });
});

test('a pull request body names the issues it finishes and the ones it only links', () => {
  assert.deepEqual(taskRefs('Task: #12\nCloses #13, #14\n- Part of: #15 and #12\nRefs #99\nmentions #7 inline'), {
    finishes: [12, 13, 14],
    links: [15],
  });
});

test('an opened issue starts as Todo unless a label says otherwise, and a closed one is Done', () => {
  const issue = (labels, body = '') => ({ number: 5, labels: labels.map((name) => ({ name })), body });
  assert.deepEqual(plan('issues', { action: 'opened', issue: issue([]) }), [{ issue: 5, fields: { Status: 'Todo' } }]);
  assert.deepEqual(plan('issues', { action: 'opened', issue: issue(['status:blocked']) }), [
    { issue: 5, fields: { Status: 'blocked' } },
  ]);
  assert.deepEqual(plan('issues', { action: 'labeled', issue: issue(['release:v0.14.0'], 'Thread: T') }), [
    { issue: 5, fields: { Thread: 'T', Release: 'v0.14.0' } },
  ]);
  assert.deepEqual(plan('issues', { action: 'closed', issue: issue(['status:in-progress']) }), [
    { issue: 5, fields: { Status: 'Done' } },
  ]);
  assert.deepEqual(plan('issues', { action: 'opened', issue: { ...issue([]), pull_request: {} } }), []);
});

test('an open pull request moves its issues to In Progress and a merge finishes only its tasks', () => {
  const pr = (action, merged = false) => ({
    action,
    pull_request: { merged, body: 'Task: #3\nPart of: #4\nThread: Grading follow-ups' },
  });
  assert.deepEqual(plan('pull_request_target', pr('opened')), [
    { issue: 3, fields: { Thread: 'Grading follow-ups', Status: 'In Progress' } },
    { issue: 4, fields: { Thread: 'Grading follow-ups', Status: 'In Progress' } },
  ]);
  assert.deepEqual(plan('pull_request_target', pr('closed', true)), [
    { issue: 3, fields: { Thread: 'Grading follow-ups', Status: 'Done' } },
  ]);
  assert.deepEqual(plan('pull_request_target', pr('closed', false)), [], 'a closed, unmerged PR changes nothing');
  assert.deepEqual(plan('push', {}), []);
});
