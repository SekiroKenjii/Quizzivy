import { useRef, useState, type SyntheticEvent } from "react";
import { useTranslation } from "react-i18next";
import { CircleAlert, LoaderCircle } from "lucide-react";
import { useNavigate, useSearchParams } from "react-router";

import { PasswordRules } from "@/components/shared/PasswordRules";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { AuthLayout } from "@/features/auth/AuthLayout";
import { changePassword, fetchCurrentUser } from "@/features/auth/api";
import { destinationAfterSignIn } from "@/features/auth/home";
import { readJoinContext } from "@/features/join/context";
import { ApiError, failureMessage } from "@/lib/api/errors";
import { passwordRules } from "@/lib/password";
import { cn } from "@/lib/utils";
import { useAuthStore } from "@/stores/auth";

/**
 * ChangePasswordPage is the forced password change (§5.4) an account with a
 * temporary password meets on its first sign-in. It asks for no current
 * password: the server does not need one while the change is forced. Once
 * saved it goes on to `?next=`, the user's home, or a join waiting to finish.
 */
export default function ChangePasswordPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const setUser = useAuthStore((s) => s.setUser);
  const [password, setPassword] = useState("");
  const [again, setAgain] = useState("");
  const [revealed, setRevealed] = useState(false);
  const [refusal, setRefusal] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const passwordRef = useRef<HTMLInputElement>(null);
  const againRef = useRef<HTMLInputElement>(null);

  const rules = passwordRules(password);
  const valid = rules.length && rules.numberOrSymbol;
  const mismatch = again !== password && (again !== "" || revealed);
  const ready = valid && again === password;

  async function onSubmit(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    if (!ready) {
      setRevealed(true);
      (valid ? againRef : passwordRef).current?.focus();
      return;
    }
    setBusy(true);
    setError(null);
    setRefusal(null);
    try {
      await changePassword("", password);
      const user = await fetchCurrentUser();
      setUser(user);
      const joining = readJoinContext();
      await navigate(
        joining
          ? `/join/${joining.code}`
          : destinationAfterSignIn(params.get("next"), user),
        { replace: true },
      );
    } catch (cause) {
      if (cause instanceof ApiError && cause.code === "PASSWORD_UNCHANGED") {
        setRefusal(t("changePassword.errors.unchanged"));
        passwordRef.current?.focus();
      } else {
        setError(failureMessage(cause, t("api.failed")));
      }
      setBusy(false);
    }
  }

  return (
    <AuthLayout>
      <div>
        <h1 className="text-h1">{t("changePassword.title")}</h1>
        <p className="text-muted-fg mt-1 text-base leading-[1.55]">
          {t("changePassword.body")}
        </p>
      </div>

      <form
        onSubmit={(e) => void onSubmit(e)}
        className="flex flex-col gap-4.5"
        noValidate
      >
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="new-password">{t("changePassword.new")}</Label>
          <Input
            ref={passwordRef}
            id="new-password"
            type="password"
            size="xl"
            autoComplete="new-password"
            value={password}
            aria-invalid={(revealed && !valid) || refusal !== null || undefined}
            aria-describedby="new-password-rules"
            onChange={(e) => {
              setPassword(e.target.value);
              setRefusal(null);
            }}
          />
        </div>
        <PasswordRules
          id="new-password-rules"
          password={password}
          revealed={revealed}
          refusal={refusal}
          className="-mt-2.5"
        />

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="new-password-again">{t("changePassword.again")}</Label>
          <Input
            ref={againRef}
            id="new-password-again"
            type="password"
            size="xl"
            autoComplete="new-password"
            value={again}
            aria-invalid={mismatch || undefined}
            aria-describedby={mismatch ? "new-password-mismatch" : undefined}
            onChange={(e) => setAgain(e.target.value)}
          />
          {mismatch && (
            <p id="new-password-mismatch" className="text-danger-ink text-meta">
              {t("changePassword.errors.mismatch")}
            </p>
          )}
        </div>

        {error && (
          <Alert variant="danger" className="text-ui flex gap-2.5">
            <CircleAlert aria-hidden="true" className="mt-px size-4 shrink-0" />
            <span>{error}</span>
          </Alert>
        )}

        <Button
          type="submit"
          size="xl"
          className={cn("h-12", !ready && "opacity-50")}
          aria-disabled={!ready || undefined}
          aria-busy={busy || undefined}
        >
          {busy && (
            <LoaderCircle aria-hidden="true" className="size-[17px] animate-spin" />
          )}
          {t(busy ? "changePassword.saving" : "changePassword.save")}
        </Button>
      </form>
    </AuthLayout>
  );
}
