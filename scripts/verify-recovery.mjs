import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';

try {
  assert.ok(process.env.RESTORE_DATABASE_URL, 'RESTORE_DATABASE_URL must identify the isolated recovery database');
  const connection = new URL(process.env.RESTORE_DATABASE_URL);
  assert.ok(['postgres:', 'postgresql:'].includes(connection.protocol), 'Expected a PostgreSQL URL');
  const databaseEnv = {
    PGHOST: connection.hostname,
    PGPORT: connection.port || '5432',
    PGDATABASE: decodeURIComponent(connection.pathname.slice(1)),
    PGUSER: decodeURIComponent(connection.username),
    PGPASSWORD: decodeURIComponent(connection.password),
    PGSSLMODE: connection.searchParams.get('sslmode') ?? 'require',
    PGOPTIONS: '-c default_transaction_read_only=on -c timezone=UTC -c statement_timeout=60000',
  };
  const output = execFileSync('psql', ['-X', '-qAt', '-v', 'ON_ERROR_STOP=1', '-f', new URL('./recovery-evidence.sql', import.meta.url).pathname], {
    env: { ...process.env, ...databaseEnv },
    encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'], maxBuffer: 32 * 1024 * 1024,
  });
  const evidence = JSON.parse(output);
  assert.equal(evidence.postgresMajor, 18, 'Recovery must use PostgreSQL 18');
  assert.equal(evidence.unvalidatedConstraints, 0, 'Recovery has unvalidated constraints');
  assert.equal(evidence.appCanRewriteLogs, false, 'Application role can rewrite logs');
  if (process.argv[2]) assert.deepEqual(evidence, JSON.parse(readFileSync(process.argv[2], 'utf8')), 'Recovered data differs from the known checkpoint');
  console.log(JSON.stringify(evidence, null, 2));
} catch (error) {
  console.error(error.stderr ? 'Recovery query failed; check database connectivity and migration compatibility.' : error.message);
  process.exitCode = 1;
}
