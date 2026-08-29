import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
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
    <main className="min-h-svh p-4 pt-14">
      <Card className="mx-auto w-full max-w-sm gap-0 p-5">
        <h1 className="text-xl font-semibold tracking-tight">
          {t("changePassword.title")}
        </h1>
        <p className="text-muted-foreground mt-1.5 text-sm leading-relaxed">
          {t("changePassword.body")}
        </p>

        <form onSubmit={onSubmit} className="mt-5" noValidate>
          <div className="space-y-3">
            {/* Not required, and that is the point. This page is reached only
              while `mustChangePassword` is set, which happens after a teacher
              resets the password — including on a Google-only account, whose
              owner may sign in with Google and land here having never held the
              temporary one. Demanding it would strand exactly the person the
              reset was for. */}
            <div className="space-y-1.5">
              <Label htmlFor="current-password">{t("changePassword.current")}</Label>
              <Input
                id="current-password"
                type="password"
                className="h-11"
                autoComplete="current-password"
                value={current}
                onChange={(e) => setCurrent(e.target.value)}
                aria-describedby="current-password-hint"
              />
              <p id="current-password-hint" className="text-muted-foreground text-xs">
                {t("changePassword.currentHint")}
              </p>
            </div>
            <div>
              <Label htmlFor="new-password">{t("changePassword.new")}</Label>
              <Input
                id="new-password"
                type="password"
                className="mt-1.5 h-11"
                autoComplete="new-password"
                minLength={8}
                required
                value={next}
                onChange={(e) => setNext(e.target.value)}
                aria-describedby="new-password-hint"
              />
              <p
                id="new-password-hint"
                className="text-muted-foreground mt-1.5 text-xs"
              >
                {t("changePassword.hint")}
              </p>
            </div>
          </div>

          {error ? (
            <p role="alert" className="text-destructive mt-3 text-sm">
              {error}
            </p>
          ) : null}

          <Button type="submit" size="lg" className="mt-5 w-full" disabled={pending}>
            {pending ? t("common.loading") : t("changePassword.submit")}
          </Button>
        </form>
      </Card>
    </main>
  );
}
