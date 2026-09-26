import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowLeft, CircleAlert, LoaderCircle } from "lucide-react";
import { Link, useNavigate, useSearchParams } from "react-router";
import { api } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { callbackUrl, statesMatch, takePending } from "@/features/auth/google/pkce";
import { destinationAfterSignIn, preloadStudentHome } from "@/features/auth/home";
import { useAuthStore } from "@/stores/auth";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { AuthLayout } from "@/features/auth/AuthLayout";

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
      <AuthLayout>
        <div>
          <h1 className="text-h1">{t("login.googleFailedTitle")}</h1>
        </div>
        <Alert variant="danger" className="text-ui flex gap-2.5">
          <CircleAlert aria-hidden="true" className="mt-px size-4 shrink-0" />
          <span>{error}</span>
        </Alert>
        <Button
          asChild
          variant="outline"
          size="xl"
          className="bg-card hover:bg-muted w-full font-medium"
        >
          <Link to="/login" replace>
            <ArrowLeft aria-hidden="true" className="size-[17px]" />
            {t("login.backToSignIn")}
          </Link>
        </Button>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout>
      <p role="status" className="text-muted-fg flex items-center gap-2.5">
        <LoaderCircle aria-hidden="true" className="size-[17px] animate-spin" />
        {t("login.completingGoogle")}
      </p>
    </AuthLayout>
  );
}
