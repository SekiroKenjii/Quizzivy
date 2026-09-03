import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useSearchParams } from "react-router";
import { api } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { callbackUrl, statesMatch, takePending } from "@/features/auth/google/pkce";
import { destinationAfterSignIn, preloadStudentHome } from "@/features/auth/home";
import { useAuthStore } from "@/stores/auth";
import { Button } from "@/components/ui/button";

/** Where Google sends the browser back (§5.3 step 2). */
export default function GoogleCallbackPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const setSession = useAuthStore((s) => s.setSession);
  const setUser = useAuthStore((s) => s.setUser);
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
        await navigate("/login", { replace: true });
        return;
      }
      if (!pending || !code || !state || !statesMatch(pending.state, state)) {
        setError(t("login.googleFailed"));
        return;
      }

      try {
        if (pending.mode === "link") {
          const linked = await api("post", "/auth/google/link", {
            body: { code, codeVerifier: pending.verifier, redirectUri: callbackUrl() },
          });
          setUser(linked);
          await navigate(destinationAfterSignIn(pending.next, linked), {
            replace: true,
          });
          return;
        }

        // A visitor holding a class code is a student, whatever the exchange says next.
        if (pending.joinCode) preloadStudentHome();

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
  }, [params, navigate, setSession, setUser, t]);

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
