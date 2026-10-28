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
  assert.deepEqual(plan('issues', { action: 'opened', issue: issue(['status:blocked'], 'Plan ID: R7-01') }), [
    { issue: 5, fields: { 'Plan ID': 'R7-01', Status: 'blocked' } },
  ]);
  assert.deepEqual(plan('issues', { action: 'closed', issue: issue(['status:in-progress']) }), [
    { issue: 5, fields: { Status: 'Done' } },
  ]);
  assert.deepEqual(plan('issues', { action: 'opened', issue: { ...issue([]), pull_request: {} } }), []);
});

test('a label or an edit writes only what it changed, so a stale label never undoes a move', () => {
  const issue = { number: 5, labels: [{ name: 'status:todo' }, { name: 'priority:p1' }], body: 'Thread: T' };
  assert.deepEqual(plan('issues', { action: 'labeled', label: { name: 'priority:p1' }, issue }), [
    { issue: 5, fields: { Priority: 'p1' } },
  ]);
  assert.deepEqual(plan('issues', { action: 'labeled', label: { name: 'bug' }, issue }), []);
  assert.deepEqual(plan('issues', { action: 'edited', issue }), [{ issue: 5, fields: { Thread: 'T' } }]);
  assert.deepEqual(plan('issues', { action: 'edited', issue: { ...issue, body: 'no fields' } }), []);
});

const pr = (action, { merged = false, association = 'OWNER', head = 'o/r' } = {}) => ({
  action,
  pull_request: {
    merged,
    author_association: association,
    head: { repo: { full_name: head } },
    base: { repo: { full_name: 'o/r' } },
    body: 'Task: #3\nPart of: #4\nThread: Grading follow-ups',
  },
});

test('an open pull request moves its issues to In Progress and a merge finishes only its tasks', () => {
  assert.deepEqual(plan('pull_request_target', pr('opened')), [
    { issue: 3, fields: { Thread: 'Grading follow-ups', Status: 'In Progress' } },
    { issue: 4, fields: { Thread: 'Grading follow-ups', Status: 'In Progress' } },
  ]);
  assert.deepEqual(plan('pull_request_target', pr('closed', { merged: true })), [
    { issue: 3, fields: { Thread: 'Grading follow-ups', Status: 'Done' } },
  ]);
  assert.deepEqual(plan('pull_request_target', pr('closed')), [], 'a closed, unmerged PR changes nothing');
  assert.deepEqual(plan('push', {}), []);
});

test('only a collaborator or a branch of this repository may move issues', () => {
  const fork = { association: 'CONTRIBUTOR', head: 'someone/r' };
  assert.deepEqual(plan('pull_request_target', pr('opened', fork)), []);
  assert.deepEqual(plan('pull_request_target', pr('closed', { ...fork, merged: true })), []);
  assert.equal(plan('pull_request_target', pr('opened', { association: 'COLLABORATOR', head: 'someone/r' })).length, 2);
  assert.equal(plan('pull_request_target', pr('opened', { association: 'NONE' })).length, 2, 'same repository');
  assert.deepEqual(plan('pull_request_target', pr('opened', { association: 'NONE', head: null })), []);
});
