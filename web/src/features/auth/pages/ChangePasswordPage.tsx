import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { changePassword, fetchCurrentUser } from "@/features/auth/api";
import { ApiError } from "@/lib/api/errors";
import { homePathFor } from "@/features/auth/home";
import { useAuthStore } from "@/stores/auth";

/**
 * The forced password change (§5.4).
 *
 * Reached from every route while `mustChangePassword` is set, which is how a
 * teacher-issued temporary password stops being a shared secret. The page has
 * no way out on purpose: navigating anywhere else lands back here.
 *
 * A wrong current password is a 400, not a 401 -- see the server handler. If it
 * were a 401 the API client would refresh, retry, and sign the user out for a
 * typo.
 */
export default function ChangePasswordPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const setUser = useAuthStore((s) => s.setUser);

  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    setPending(true);
    try {
      await changePassword(current, next);
      // Re-read rather than assume: the server clears mustChangePassword, and
      // the guard reads it from the store. Guessing here would leave the user
      // on this page after a successful change.
      const user = await fetchCurrentUser();
      setUser(user);
      await navigate(homePathFor(user), { replace: true });
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : t("error.body"));
    } finally {
      setPending(false);
    }
  }

  return (
    <main className="flex min-h-svh items-center justify-center p-6">
      <div className="w-full max-w-sm">
        <h1 className="text-2xl font-semibold tracking-tight">
          {t("changePassword.title")}
        </h1>
        <p className="text-muted-foreground mt-2 text-sm">{t("changePassword.body")}</p>

        <form onSubmit={onSubmit} className="mt-6 space-y-4" noValidate>
          <div className="space-y-2">
            <Label htmlFor="current-password">{t("changePassword.current")}</Label>
            <Input
              id="current-password"
              type="password"
              autoComplete="current-password"
              required
              value={current}
              onChange={(e) => setCurrent(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="new-password">{t("changePassword.new")}</Label>
            <Input
              id="new-password"
              type="password"
              autoComplete="new-password"
              minLength={8}
              required
              value={next}
              onChange={(e) => setNext(e.target.value)}
              aria-describedby="new-password-hint"
            />
            <p id="new-password-hint" className="text-muted-foreground text-xs">
              {t("changePassword.hint")}
            </p>
          </div>

          {error ? (
            <p role="alert" className="text-destructive text-sm">
              {error}
            </p>
          ) : null}

          <Button type="submit" className="w-full" disabled={pending}>
            {pending ? t("common.loading") : t("changePassword.submit")}
          </Button>
        </form>
      </div>
    </main>
  );
}
