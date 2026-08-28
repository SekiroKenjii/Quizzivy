#!/bin/bash
# Runs once, as superuser, on first container start.
#
# Delegates to scripts/provision-db.sh so local, CI and production share one
# definition of the roles rather than three copies that drift.
set -euo pipefail
export PGUSER="${POSTGRES_USER}"
exec /provision/provision-db.sh
