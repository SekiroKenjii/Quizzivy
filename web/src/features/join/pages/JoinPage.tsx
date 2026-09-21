import { useEffect, useRef, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Link, useNavigate, useParams } from "react-router";
import { Card } from "@/components/ui/card";
import { isComplete, normalize } from "@/features/join/code";
import { JoinCodeForm } from "@/features/join/components/JoinCodeForm";
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

        {error ? (
          <p role="alert" className="text-destructive mt-3 text-xs">
            {error}
          </p>
        ) : null}
        <div className="mt-5">
          {/* The confirm step is mandatory (§6.2): nothing creates an account here. */}
          <JoinCodeForm
            id="join-code"
            initial={codeParam ?? ""}
            onContinue={(code) => void navigate(`/join/${code}/confirm`)}
          />
        </div>
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
