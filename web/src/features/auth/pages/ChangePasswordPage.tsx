import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { changePassword, fetchCurrentUser } from "@/features/auth/api";
import {
  changePasswordSchema,
  type ChangePasswordValues,
} from "@/features/auth/changePasswordSchema";
import { ApiError } from "@/lib/api/errors";
import { homePathFor } from "@/features/auth/home";
import { useAuthStore } from "@/stores/auth";

/** The forced password change (§5.4). */
export default function ChangePasswordPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const setUser = useAuthStore((s) => s.setUser);

  const [error, setError] = useState<string | null>(null);
  const form = useForm<ChangePasswordValues>({
    resolver: zodResolver(changePasswordSchema),
    defaultValues: { currentPassword: "", newPassword: "" },
    mode: "onTouched",
  });
  const newPasswordError = form.formState.errors.newPassword;

  const onSubmit = form.handleSubmit(async (values) => {
    setError(null);
    try {
      await changePassword(values.currentPassword, values.newPassword);
      const user = await fetchCurrentUser();
      setUser(user);
      await navigate(homePathFor(user), { replace: true });
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : t("error.body"));
    }
  });

  return (
    <main className="min-h-svh p-4 pt-14">
      <Card className="mx-auto w-full max-w-sm gap-0 p-5">
        <h1 className="text-xl font-semibold tracking-tight">
          {t("changePassword.title")}
        </h1>
        <p className="text-muted-foreground mt-1.5 text-sm leading-relaxed">
          {t("changePassword.body")}
        </p>

        <form onSubmit={(e) => void onSubmit(e)} className="mt-5" noValidate>
          <div className="space-y-3">
            {/* Not required, and that is the point. */}
            <div className="space-y-1.5">
              <Label htmlFor="current-password">{t("changePassword.current")}</Label>
              <Input
                id="current-password"
                type="password"
                className="h-11"
                autoComplete="current-password"
                aria-describedby="current-password-hint"
                {...form.register("currentPassword")}
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
                aria-invalid={newPasswordError ? true : undefined}
                aria-describedby={
                  newPasswordError ? "new-password-error" : "new-password-hint"
                }
                {...form.register("newPassword")}
              />
              {newPasswordError ? (
                <p id="new-password-error" className="text-destructive mt-1.5 text-sm">
                  {t(newPasswordError.message ?? "changePassword.errors.tooShort")}
                </p>
              ) : (
                <p
                  id="new-password-hint"
                  className="text-muted-foreground mt-1.5 text-xs"
                >
                  {t("changePassword.hint")}
                </p>
              )}
            </div>
          </div>

          {error ? (
            <p role="alert" className="text-destructive mt-3 text-sm">
              {error}
            </p>
          ) : null}

          <Button
            type="submit"
            size="lg"
            className="mt-5 w-full"
            disabled={form.formState.isSubmitting}
          >
            {form.formState.isSubmitting
              ? t("common.loading")
              : t("changePassword.submit")}
          </Button>
        </form>
      </Card>
    </main>
  );
}
