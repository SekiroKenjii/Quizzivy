import { useEffect, useRef, useState, type FormEvent } from "react";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Link, useNavigate, useParams } from "react-router";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { format, isComplete, normalize } from "@/features/join/code";
import { joinClass } from "@/features/join/api";
import { ApiError } from "@/lib/api/errors";
import { useAuthStore } from "@/stores/auth";

/**
 * §6.2 steps one and two: `/join` (type a code) and `/join/:code` (a deep link
 * from a QR or a message, with the code filled in).
 */
export default function JoinPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { code: codeParam } = useParams();
  const isSignedIn = useAuthStore((s) => s.user !== null);
  const isBootstrapping = useAuthStore((s) => s.isBootstrapping);

  const [code, setCode] = useState(() => format(codeParam ?? ""));
  const [error, setError] = useState<string | null>(null);
  const started = useRef(false);
  const enrol = useMutation({
    mutationFn: (joinCode: string) => joinClass(joinCode),
    onSuccess: () => navigate("/app", { replace: true }),
    onError: (cause) =>
      setError(cause instanceof ApiError ? cause.message : t("join.failed")),
  });

  const enrolMutate = enrol.mutate;
  useEffect(() => {
    if (isBootstrapping || !isSignedIn || !codeParam || started.current) return;
    if (!isComplete(codeParam)) return;
    started.current = true;
    enrolMutate(normalize(codeParam));
  }, [isBootstrapping, isSignedIn, codeParam, enrolMutate]);

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
    <>
      <Card className="gap-0 p-5">
        <h1 className="text-xl font-semibold tracking-tight">{t("join.title")}</h1>
        <p className="text-muted-foreground mt-1.5 text-sm">{t("join.subtitle")}</p>

        <form onSubmit={onSubmit} className="mt-5" noValidate>
          <Label htmlFor="join-code">{t("join.codeLabel")}</Label>
          <Input
            id="join-code"
            className="mt-1.5 h-11 font-mono text-lg tracking-wide"
            value={code}
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
          <p id="join-code-hint" className="text-muted-foreground mt-1.5 text-xs">
            {t("join.codeHint")}
          </p>

          {error ? (
            <p role="alert" className="text-destructive mt-1.5 text-xs">
              {error}
            </p>
          ) : null}

          <Button
            type="submit"
            size="lg"
            className="mt-5 w-full"
            disabled={!isComplete(code)}
          >
            {t("join.continue")}
          </Button>
        </form>
      </Card>

      <p className="text-muted-foreground mt-5 text-center text-xs leading-relaxed">
        {t("join.haveAccount")}{" "}
        <Link to="/login" className="underline">
          {t("join.signIn")}
        </Link>
      </p>
    </>
  );
}
