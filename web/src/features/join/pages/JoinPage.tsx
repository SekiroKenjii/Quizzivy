import { useEffect, useRef, useState, type FormEvent } from "react";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { format, isComplete, normalize } from "@/features/join/code";
import { joinClass } from "@/features/join/api";
import { ApiError } from "@/lib/api/errors";
import { useAuthStore } from "@/stores/auth";

/**
 * §6.2 steps one and two: `/join` (type a code) and `/join/:code` (a deep link
 * from a QR or a message, with the code filled in).
 *
 * One component for both, because they differ only in where the initial value
 * comes from -- and the prefilled case still SHOWS the code rather than acting
 * on it, so a student who scanned the wrong poster can see that before going
 * any further.
 *
 * A student who is already signed in skips both steps: §6.2 sends them straight
 * to enrolment, because the confirm screen exists to show an ANONYMOUS visitor
 * what they are about to create an account for.
 */
export default function JoinPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { code: codeParam } = useParams();
  const user = useAuthStore((s) => s.user);
  const isBootstrapping = useAuthStore((s) => s.isBootstrapping);

  const [code, setCode] = useState(() => format(codeParam ?? ""));
  const [error, setError] = useState<string | null>(null);
  // POST /app/classes/join is idempotent, but firing it twice under StrictMode
  // would still spend two of §6.5's per-code allowance.
  const started = useRef(false);

  // A mutation rather than hand-rolled state: `isPending` replaces a
  // setState-in-effect, which is both the lint rule and the reason for it.
  const enrol = useMutation({
    mutationFn: (joinCode: string) => joinClass(joinCode),
    onSuccess: () => navigate("/app", { replace: true }),
    onError: (cause) =>
      setError(cause instanceof ApiError ? cause.message : t("join.failed")),
  });

  useEffect(() => {
    if (isBootstrapping || !user || !codeParam || started.current) return;
    if (!isComplete(codeParam)) return;
    started.current = true;
    enrol.mutate(normalize(codeParam));
  }, [isBootstrapping, user, codeParam, enrol]);

  function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (!isComplete(code)) {
      setError(t("join.errors.incomplete"));
      return;
    }
    // The confirm step is mandatory (§6.2): nothing creates an account here.
    void navigate(`/join/${normalize(code)}/confirm`);
  }

  if (enrol.isPending) {
    return (
      <p className="text-muted-foreground text-sm" role="status" aria-live="polite">
        {t("join.enrolling")}
      </p>
    );
  }

  return (
    <div className="w-full max-w-sm rounded-lg border p-6">
      <h1 className="text-xl font-semibold tracking-tight">{t("join.title")}</h1>
      <p className="text-muted-foreground mt-2 text-sm">{t("join.subtitle")}</p>

      <form onSubmit={onSubmit} className="mt-6 space-y-4" noValidate>
        <div className="space-y-2">
          <Label htmlFor="join-code">{t("join.codeLabel")}</Label>
          <Input
            id="join-code"
            value={code}
            // Normalized on every keystroke (§6.1): the field only ever holds
            // characters the alphabet contains, upper case, grouped. A student
            // reading a code aloud cannot type it into an invalid state.
            onChange={(e) => {
              setCode(format(e.target.value));
              setError(null);
            }}
            autoCapitalize="characters"
            autoComplete="off"
            spellCheck={false}
            inputMode="text"
            maxLength={9}
            aria-describedby="join-code-hint"
            aria-invalid={error ? true : undefined}
          />
          <p id="join-code-hint" className="text-muted-foreground text-xs">
            {t("join.codeHint")}
          </p>
        </div>

        {error ? (
          <p role="alert" className="text-destructive text-sm">
            {error}
          </p>
        ) : null}

        <Button type="submit" className="w-full" disabled={!isComplete(code)}>
          {t("join.continue")}
        </Button>
      </form>
    </div>
  );
}
