#!/usr/bin/env bash
# T-0.2 — verify the Google OAuth client without any browser interaction.
#
# Reads .env. Never prints secrets. Exits non-zero on failure.
#
# The trick in step 3: we POST a deliberately invalid authorization code to
# Google's token endpoint using the real client credentials. Google validates
# the CLIENT before it validates the CODE, so the error it returns tells us
# whether our credentials are right:
#
#   invalid_client  -> client_id or client_secret is wrong
#   invalid_grant   -> credentials are GOOD; only the dummy code was rejected
#
# That is a complete credential check with no user, no browser, no consent.
set -uo pipefail
cd "$(dirname "$0")/.."
[ -f .env ] && set -a && . ./.env && set +a

fail=0
ok()   { printf '  \033[32m✓\033[0m %s\n' "$1"; }
bad()  { printf '  \033[31m✗\033[0m %s\n' "$1"; fail=1; }
warn() { printf '  \033[33m!\033[0m %s\n' "$1"; }
mask() { local s="$1"; [ ${#s} -le 12 ] && echo "***" || echo "${s:0:8}…${s: -4}"; }

echo "T-0.2 — Google OAuth client"
echo

# 1. present?
if [ -z "${VITE_GOOGLE_CLIENT_ID:-}" ]; then
  bad "VITE_GOOGLE_CLIENT_ID is not set in .env"
else
  ok "client id present: $(mask "$VITE_GOOGLE_CLIENT_ID")"
  case "$VITE_GOOGLE_CLIENT_ID" in
    *.apps.googleusercontent.com) ok "client id has the expected form" ;;
    *) bad "client id should end in .apps.googleusercontent.com — is this the right value?" ;;
  esac
fi
if [ -z "${GOOGLE_CLIENT_SECRET:-}" ]; then
  bad "GOOGLE_CLIENT_SECRET is not set in .env"
else
  ok "client secret present: $(mask "$GOOGLE_CLIENT_SECRET")"
  case "$GOOGLE_CLIENT_SECRET" in
    GOCSPX-*) ok "client secret has the expected form" ;;
    *) warn "client secret does not start with GOCSPX- (older clients may differ)" ;;
  esac
fi
[ $fail -eq 1 ] && { echo; echo "Fill these in .env, then re-run."; exit 1; }

# 2. Google's server still advertises what §5.3 depends on
echo
disc=$(curl -fsS https://accounts.google.com/.well-known/openid-configuration 2>/dev/null)
if [ -z "$disc" ]; then
  bad "could not reach Google's discovery document"
else
  ok "discovery reachable"
  echo "$disc" | jq -e '.code_challenge_methods_supported | index("S256")' >/dev/null \
    && ok "PKCE S256 supported (spec §5.3 depends on this)" \
    || bad "S256 not advertised — §5.3's PKCE assumption no longer holds"
  echo "$disc" | jq -e '.token_endpoint_auth_methods_supported | index("client_secret_post")' >/dev/null \
    && ok "client_secret_post supported" || warn "client_secret_post not advertised"
fi

# 3. do the credentials actually work?
echo
redirect="${GOOGLE_REDIRECT_URI:-http://localhost:5173/auth/google/callback}"
resp=$(curl -fsS -X POST https://oauth2.googleapis.com/token \
  -d grant_type=authorization_code \
  -d "code=quizzivy-verification-probe-not-a-real-code" \
  -d "client_id=${VITE_GOOGLE_CLIENT_ID}" \
  -d "client_secret=${GOOGLE_CLIENT_SECRET}" \
  --data-urlencode "redirect_uri=${redirect}" 2>/dev/null \
  || curl -sS -X POST https://oauth2.googleapis.com/token \
       -d grant_type=authorization_code \
       -d "code=quizzivy-verification-probe-not-a-real-code" \
       -d "client_id=${VITE_GOOGLE_CLIENT_ID}" \
       -d "client_secret=${GOOGLE_CLIENT_SECRET}" \
       --data-urlencode "redirect_uri=${redirect}" 2>/dev/null)

err=$(echo "$resp" | jq -r '.error // empty' 2>/dev/null)
desc=$(echo "$resp" | jq -r '.error_description // empty' 2>/dev/null)

case "$err" in
  invalid_grant)
    ok "credentials accepted by Google (the dummy code was rejected, which is correct)"
    ok "redirect_uri accepted: ${redirect}"
    ;;
  invalid_client)
    bad "Google rejected the client — client_id or client_secret is wrong"
    [ -n "$desc" ] && echo "      Google said: $desc"
    ;;
  redirect_uri_mismatch)
    bad "redirect_uri '${redirect}' is not registered on this OAuth client"
    echo "      Add it under Authorized redirect URIs in the console."
    ;;
  "")
    bad "unexpected response from Google's token endpoint"
    echo "$resp" | head -5 | sed 's/^/      /'
    ;;
  *)
    warn "Google returned '${err}'"
    [ -n "$desc" ] && echo "      $desc"
    warn "not a definitive pass — check the client configuration in the console"
    ;;
esac

echo
if [ $fail -eq 0 ]; then
  echo "T-0.2 verified. Phase 1's Google path is unblocked."
else
  echo "T-0.2 not yet satisfied — see docs/setup/google-oauth.md"
fi
exit $fail
