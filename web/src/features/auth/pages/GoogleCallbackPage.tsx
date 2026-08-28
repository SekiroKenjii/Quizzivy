import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useSearchParams } from "react-router";
import { api } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { callbackUrl, statesMatch, takePending } from "@/features/auth/google/pkce";
import { destinationAfterSignIn } from "@/features/auth/home";
import { useAuthStore } from "@/stores/auth";
import { Button } from "@/components/ui/button";

/**
 * Where Google sends the browser back (§5.3 step 2).
 *
 * The authorization code is exchanged SERVER-side -- the client secret never
 * reaches a browser -- so all this page does is hand `code` and `codeVerifier`
 * to `POST /auth/google` and act on the answer.
 *
 * `state` is checked before anything else. It is the only thing standing
 * between this endpoint and a code planted by someone else's authorization
 * request, and the pending record is consumed on read, so a replay of the same
 * callback URL finds nothing.
 */
export default function GoogleCallbackPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const setSession = useAuthStore((s) => s.setSession);
  const [error, setError] = useState<string | null>(null);
  // Effects run twice under StrictMode, and this one redeems a single-use code.
  const started = useRef(false);

  useEffect(() => {
    if (started.current) return;
    started.current = true;

    void (async () => {
      const pending = takePending();
      const code = params.get("code");
      const state = params.get("state");

      if (params.get("error")) {
        // The user pressed cancel on Google's consent screen. Not an error to
        // apologise for -- just take them back.
        await navigate("/login", { replace: true });
        return;
      }
      if (!pending || !code || !state || !statesMatch(pending.state, state)) {
        setError(t("login.googleFailed"));
        return;
      }

      try {
        const result = await api("post", "/auth/google", {
          body: {
            code,
            codeVerifier: pending.verifier,
            redirectUri: callbackUrl(),
            ...(pending.joinCode ? { joinCode: pending.joinCode } : {}),
          },
        });
        setSession(result.accessToken, result.user);
        await navigate(destinationAfterSignIn(pending.next, result.user), {
          replace: true,
        });
      } catch (cause) {
        setError(cause instanceof ApiError ? cause.message : t("login.googleFailed"));
      }
    })();
  }, [params, navigate, setSession, t]);

  if (error) {
    return (
      <main className="flex min-h-svh flex-col items-center justify-center gap-4 p-6 text-center">
        <p role="alert" className="text-sm">
          {error}
        </p>
        <Button onClick={() => void navigate("/login", { replace: true })}>
          {t("login.backToSignIn")}
        </Button>
      </main>
    );
  }

  return (
    <main
      className="text-muted-foreground flex min-h-svh items-center justify-center text-sm"
      role="status"
      aria-live="polite"
    >
      {t("login.completingGoogle")}
    </main>
  );
}
