#!/usr/bin/env bash
# T-0.3 — verify the R2 bucket and credentials end to end.
#
# Reads .env. Never prints secrets. Exits non-zero on failure.
# Uses the minio/mc image the compose stack already pulls, so there is nothing
# extra to install.
#
# Checks the things that actually matter for §11.2: the credentials work, the
# bucket is writable, a signed URL is readable, and — the important one — an
# UNSIGNED request is refused. A publicly readable media bucket would leak every
# listening file to anyone with a URL.
set -uo pipefail
cd "$(dirname "$0")/.."
[ -f .env ] && set -a && . ./.env && set +a

fail=0
ok()   { printf '  \033[32m✓\033[0m %s\n' "$1"; }
bad()  { printf '  \033[31m✗\033[0m %s\n' "$1"; fail=1; }
warn() { printf '  \033[33m!\033[0m %s\n' "$1"; }
mask() { local s="$1"; [ ${#s} -le 12 ] && echo "***" || echo "${s:0:6}…${s: -4}"; }

BUCKET="${S3_BUCKET:-quizzivy-media}"
echo "T-0.3 — Cloudflare R2 (bucket: ${BUCKET})"
echo

for v in R2_ACCOUNT_ID R2_ACCESS_KEY_ID R2_SECRET_ACCESS_KEY; do
  if [ -z "${!v:-}" ]; then bad "$v is not set in .env"; else ok "$v present: $(mask "${!v}")"; fi
done
[ $fail -eq 1 ] && { echo; echo "Fill these in .env, then re-run. See docs/setup/r2.md"; exit 1; }

ENDPOINT="https://${R2_ACCOUNT_ID}.r2.cloudflarestorage.com"
ok "endpoint: ${ENDPOINT}"
echo

KEY="_quizzivy-verify-$$.txt"
mcrun() {
  docker run --rm -i \
    -e MC_HOST_r2="https://${R2_ACCESS_KEY_ID}:${R2_SECRET_ACCESS_KEY}@${R2_ACCOUNT_ID}.r2.cloudflarestorage.com" \
    --entrypoint /bin/sh minio/mc:latest -c "$1" 2>&1
}

# 1. credentials + bucket exist
out=$(mcrun "mc ls r2/${BUCKET} --json >/dev/null && echo LISTED")
if echo "$out" | grep -q LISTED; then
  ok "credentials accepted; bucket '${BUCKET}' is reachable"
else
  bad "could not list the bucket"
  echo "$out" | head -4 | sed 's/^/      /'
  echo
  echo "  Common causes: the token is not scoped to this bucket, the bucket name"
  echo "  differs, or the account id is wrong."
  exit 1
fi

# 2. writable
out=$(mcrun "echo quizzivy-verification | mc pipe r2/${BUCKET}/${KEY} && echo WROTE")
echo "$out" | grep -q WROTE && ok "object write succeeded" || { bad "object write failed"; echo "$out" | head -4 | sed 's/^/      /'; }

# 3. readable back
out=$(mcrun "mc cat r2/${BUCKET}/${KEY}")
echo "$out" | grep -q quizzivy-verification && ok "object read succeeded" || bad "object read failed"

# 4. presigned GET works (this is how §11.2 serves audio)
#
#    Note if you ever retarget this at MinIO to test the script: mc mints the
#    URL with the hostname IT sees, and SigV4 binds the signature to the host
#    header -- so rewriting the host to reach it from outside the container
#    yields SignatureDoesNotMatch. Use `--network host`. Against real R2 the
#    hostname is public and this does not arise.
url=$(mcrun "mc share download --expire 10m r2/${BUCKET}/${KEY} --json" | jq -r '.share // empty' 2>/dev/null)
if [ -n "$url" ]; then
  code=$(curl -s -o /dev/null -w '%{http_code}' "$url")
  [ "$code" = "200" ] && ok "presigned GET returns 200 (signed-URL delivery works)" \
                      || bad "presigned GET returned HTTP ${code}"
else
  warn "could not mint a presigned URL via mc — verify in code during T-2.4"
fi

# 5. THE IMPORTANT ONE: unsigned access must be refused
code=$(curl -s -o /dev/null -w '%{http_code}' "${ENDPOINT}/${BUCKET}/${KEY}")
if [ "$code" = "401" ] || [ "$code" = "403" ]; then
  ok "unsigned request refused (HTTP ${code}) — bucket is private, per §11.2"
else
  bad "unsigned request returned HTTP ${code} — THE BUCKET MAY BE PUBLIC"
  echo "      §11.2 requires a private bucket: no public listing, no public read."
  echo "      Check R2 > ${BUCKET} > Settings > Public access, and remove any r2.dev"
  echo "      subdomain or custom domain binding."
fi

# 6. clean up
mcrun "mc rm r2/${BUCKET}/${KEY}" >/dev/null && ok "test object removed" || warn "could not remove ${KEY} — delete it by hand"

echo
if [ $fail -eq 0 ]; then
  echo "T-0.3 verified. Phase 2 can deploy against real R2."
  echo "Local development still uses MinIO; only the endpoint and keys differ."
else
  echo "T-0.3 not yet satisfied — see docs/setup/r2.md"
fi
exit $fail
