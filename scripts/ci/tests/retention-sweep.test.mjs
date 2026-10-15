import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';

import { SETS } from '../plan.mjs';

const workflow = readFileSync('.github/workflows/retention-sweep.yml', 'utf8');
const lines = workflow.split('\n');

function runScripts() {
  const scripts = [];
  for (let i = 0; i < lines.length; i++) {
    const match = /^(\s*)run:\s*(.*)$/.exec(lines[i]);
    if (!match) continue;
    const indent = match[1].length;
    if (match[2] !== '|') {
      scripts.push(match[2]);
      continue;
    }
    const body = [];
    while (i + 1 < lines.length && (lines[i + 1].trim() === '' || lines[i + 1].search(/\S/) > indent)) {
      body.push(lines[++i]);
    }
    scripts.push(body.join('\n'));
  }
  return scripts;
}

test('the sweep runs at 03:00 in Ho Chi Minh City and on request, and on no pull request', () => {
  assert.match(workflow, /schedule:\n\s+- cron: '0 20 \* \* \*'/);
  assert.match(workflow, /workflow_dispatch:/);
  assert.doesNotMatch(workflow, /pull_request|push:/);
});

test('one sweep runs at a time and a run is never cut short by the next', () => {
  assert.match(workflow, /concurrency:\n\s+group: retention-sweep\n\s+cancel-in-progress: false/);
  assert.match(workflow, /timeout-minutes: 15\b/);
});

test('the credential is the production-maintenance environment secret, set on the step that sweeps and no other', () => {
  assert.match(workflow, /environment: production-maintenance/);
  assert.deepEqual(workflow.match(/\$\{\{\s*secrets\.[A-Za-z_]+\s*\}\}/g), ['${{ secrets.MAINTENANCE_DATABASE_URL }}']);
  const sweep = workflow.slice(workflow.indexOf('- name: Sweep'));
  assert.ok(sweep.includes('MAINTENANCE_DATABASE_URL: ${{ secrets.MAINTENANCE_DATABASE_URL }}'));
  assert.match(workflow, /permissions:\n\s+contents: read\n/);
});

test('the workflow runs only GitHub\'s own actions, each pinned to a commit with its tag in a comment, since this job holds a credential', () => {
  const uses = [...workflow.matchAll(/^\s*-?\s*uses:\s*(\S+)(?:\s+#\s*(\S+))?/gm)].map((match) => [match[1], match[2]]);
  assert.ok(uses.length >= 2);
  for (const [action, tag] of uses) {
    assert.match(action, /^actions\/[a-z-]+@[0-9a-f]{40}$/);
    assert.match(tag ?? '', /^v\d+(\.\d+){0,2}$/);
  }
});

test('the job keeps no repository token on disk and restores no cache while it holds the credential', () => {
  assert.match(workflow, /actions\/checkout@[0-9a-f]{40} # v\d+\n\s+with:\n\s+persist-credentials: false\n/);
  assert.match(workflow, /actions\/setup-go@[0-9a-f]{40} # v\d+\n\s+with:\n(\s+[a-z-]+: .*\n)*?\s+cache: false\n/);
  assert.doesNotMatch(workflow, /cache-dependency-path/);
});

test('no run script interpolates an expression, so an input cannot inject a command', () => {
  const scripts = runScripts();
  assert.ok(scripts.length >= 2);
  for (const script of scripts) {
    assert.doesNotMatch(script, /\$\{\{/);
  }
});

test('a scheduled run writes only when the repository variable says true, a manual run only when asked', () => {
  assert.match(workflow, /ENABLED: \$\{\{ vars\.RETENTION_SWEEP_ENABLED \}\}/);
  assert.match(workflow, /REQUESTED: \$\{\{ inputs\.apply \}\}/);
  assert.match(
    workflow,
    /if \[ "\$EVENT" = schedule \]; then\n\s+if \[ "\$ENABLED" = true \]; then apply=true; fi\n\s+elif \[ "\$REQUESTED" = true \]; then\n\s+apply=true\n\s+fi/,
  );
  assert.equal(workflow.match(/-apply/g).length, 1);
  assert.match(workflow, /flags=\(-budget 5m\)/);
});

test('a change to the workflow alone runs no code job', () => {
  assert.ok(SETS.code.exclude.includes('.github/workflows/retention-sweep.yml'));
});
