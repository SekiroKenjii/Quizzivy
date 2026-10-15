import assert from 'node:assert/strict';
import { chmodSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
import { test } from 'node:test';

const script = join(process.cwd(), 'scripts/provision-db.sh');

const stubPsql = `#!/usr/bin/env bash
log="$STUB_LOG"
role=""
password=""
command=""
while [ $# -gt 0 ]; do
  case "$1" in
    -v) case "$2" in role=*) role="\${2#role=}" ;; pw=*) password="\${2#pw=}" ;; esac; shift 2 ;;
    -tAc|-c) command="$2"; shift 2 ;;
    *) shift ;;
  esac
done
if [ -n "$role" ]; then
  echo "SET PASSWORD $role" >>"$log"
  exit 0
fi
case "$command" in
  *"FROM pg_roles"*) printf '%s' "$STUB_ROLES" | tr ',' '\\n'; echo "ROLES QUERIED" >>"$log" ;;
  *"FROM pg_database"*) echo 1; echo "DATABASE QUERIED" >>"$log" ;;
  "") echo "SQL $(cat)" >>"$log" ;;
  *) echo "COMMAND $command" >>"$log" ;;
esac
`;

function provision({ args = [], env = {}, roles = '' }) {
  const dir = mkdtempSync(join(tmpdir(), 'provision-'));
  try {
    const bin = join(dir, 'psql');
    writeFileSync(bin, stubPsql);
    chmodSync(bin, 0o755);
    const log = join(dir, 'log');
    writeFileSync(log, '');
    const result = spawnSync('bash', [script, ...args], {
      encoding: 'utf8',
      env: {
        PATH: `${dir}:${process.env.PATH}`,
        STUB_LOG: log,
        STUB_ROLES: roles,
        ...env,
      },
    });
    return { ...result, log: readFileSync(log, 'utf8') };
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

const managed = 'postgres://admin:secret@ep-quiet-sun.eu-central-1.aws.neon.tech/neondb?sslmode=require';
const existing = 'quizzivy_migrate,quizzivy_app';

test('adding the maintenance role to a managed database leaves the other two roles alone', () => {
  const run = provision({
    args: [managed],
    roles: existing,
    env: { QUIZZIVY_MAINTENANCE_PASSWORD: 'a-new-random-password', QUIZZIVY_MIGRATE_PASSWORD: 'migrate', QUIZZIVY_APP_PASSWORD: 'app' },
  });
  assert.equal(run.status, 0, run.stderr);
  assert.deepEqual(run.log.match(/SET PASSWORD \w+/g), ['SET PASSWORD quizzivy_maintenance']);
  assert.doesNotMatch(run.log, /COMMAND ALTER ROLE quizzivy_migrate CREATEDB/);
  assert.match(run.stderr, /quizzivy_app exists, so QUIZZIVY_APP_PASSWORD is ignored/);
  assert.match(run.stdout, /password of quizzivy_migrate left as it was/);
  assert.match(run.stdout, /password of quizzivy_app left as it was/);
});

test('a managed database is never given CREATEDB, even when every role is new', () => {
  const run = provision({
    args: [managed],
    env: { QUIZZIVY_MIGRATE_PASSWORD: 'm-1', QUIZZIVY_APP_PASSWORD: 'a-1', QUIZZIVY_MAINTENANCE_PASSWORD: 'x-1' },
  });
  assert.equal(run.status, 0, run.stderr);
  assert.deepEqual(run.log.match(/SET PASSWORD \w+/g), [
    'SET PASSWORD quizzivy_migrate',
    'SET PASSWORD quizzivy_app',
    'SET PASSWORD quizzivy_maintenance',
  ]);
  assert.doesNotMatch(run.log, /COMMAND ALTER ROLE quizzivy_migrate CREATEDB/);
});

test('a managed database refuses to create a role without its own password and changes nothing', () => {
  const run = provision({ args: [managed], roles: existing });
  assert.notEqual(run.status, 0);
  assert.match(run.stderr, /quizzivy_maintenance does not exist and QUIZZIVY_MAINTENANCE_PASSWORD is not set/);
  assert.doesNotMatch(run.log, /SET PASSWORD|SQL|COMMAND/);
});

test('a managed database refuses a development default password', () => {
  const run = provision({ args: [managed], roles: existing, env: { QUIZZIVY_MAINTENANCE_PASSWORD: 'maintenance' } });
  assert.notEqual(run.status, 0);
  assert.match(run.stderr, /QUIZZIVY_MAINTENANCE_PASSWORD is the development default/);
  assert.doesNotMatch(run.log, /SET PASSWORD|SQL|COMMAND/);
});

test('a managed database lets the owner rotate the maintenance password and only that one', () => {
  const run = provision({
    args: [managed],
    roles: `${existing},quizzivy_maintenance`,
    env: { QUIZZIVY_MAINTENANCE_PASSWORD: 'rotated-password' },
  });
  assert.equal(run.status, 0, run.stderr);
  assert.deepEqual(run.log.match(/SET PASSWORD \w+/g), ['SET PASSWORD quizzivy_maintenance']);
});

test('a re-run that supplies nothing changes no password on any database', () => {
  for (const args of [[], [managed]]) {
    const run = provision({ args, roles: `${existing},quizzivy_maintenance` });
    assert.equal(run.status, 0, run.stderr);
    assert.doesNotMatch(run.log, /SET PASSWORD/);
  }
});

test('a local run keeps the development defaults and gives the migrate role CREATEDB', () => {
  const run = provision({ env: { PGHOST: 'localhost' } });
  assert.equal(run.status, 0, run.stderr);
  assert.deepEqual(run.log.match(/SET PASSWORD \w+/g), [
    'SET PASSWORD quizzivy_migrate',
    'SET PASSWORD quizzivy_app',
    'SET PASSWORD quizzivy_maintenance',
  ]);
  assert.match(run.log, /COMMAND ALTER ROLE quizzivy_migrate CREATEDB/);
});

test('a libpq host that is not local is a managed database', () => {
  const run = provision({ env: { PGHOST: 'ep-quiet-sun.eu-central-1.aws.neon.tech' }, roles: existing });
  assert.notEqual(run.status, 0);
  assert.doesNotMatch(run.log, /COMMAND ALTER ROLE quizzivy_migrate CREATEDB|SET PASSWORD/);
});

test('a localhost DSN is a local run', () => {
  const run = provision({
    args: ['postgres://postgres:postgres@localhost:5432/postgres'],
    roles: `${existing},quizzivy_maintenance`,
  });
  assert.equal(run.status, 0, run.stderr);
  assert.match(run.log, /COMMAND ALTER ROLE quizzivy_migrate CREATEDB/);
  assert.doesNotMatch(run.log, /SET PASSWORD/);
});
