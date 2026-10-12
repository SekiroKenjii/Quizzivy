# Real API E2E fixtures

`go test -tags e2e ./tests/...` uses `TEST_DATABASE_URL` only as a control
connection. Its test role needs PostgreSQL 18 and `CREATEDB`, as the existing
CI provisioning supplies. Each test-binary invocation creates a fresh database,
applies repository migrations and routes every test connection to it. Multiple
application boots within that invocation retain the same records.

Configured media/import stores need credentials which can create, list and
delete private buckets and their objects. The harness creates uniquely named
buckets and never uploads into or empties the caller's buckets. Unconfigured
stores remain optional. Import wake requests go to an inert local server.

The harness drains its HTTP servers and closes application/fixture pools before
emptying owned buckets and dropping its database. Setup, test and cleanup failures
exit nonzero; cleanup errors are reported. Resource names are logged without
credentials. SIGKILL or host loss can interrupt cleanup: use the exact logged
names for manual recovery; the harness never sweeps names from earlier runs.

Without `TEST_DATABASE_URL`, existing database-dependent tests explicitly skip.
A configured provisioning failure is an error, with no fallback to shared writes.
