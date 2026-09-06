import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Link, useNavigate, useParams } from "react-router";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { GoogleMark } from "@/features/auth/components/GoogleMark";
import { previewJoinCode, joinClass, type JoinPreview } from "@/features/join/api";
import { normalize } from "@/features/join/code";
import { useGoogleSignIn } from "@/features/auth/google/useGoogleSignIn";
import { ApiError } from "@/lib/api/errors";
import { useAuthStore } from "@/stores/auth";
import type { TFunction } from "i18next";

/**
 * §6.2's confirm step, and the reason it exists: "the student sees WHICH class
 * they are joining before authenticating. Never create an account and enrol in
 * one blind tap."
 */
export default function JoinConfirmPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { code = "" } = useParams();
  const user = useAuthStore((s) => s.user);
  const google = useGoogleSignIn();

  const [joinError, setJoinError] = useState<string | null>(null);
  const [joining, setJoining] = useState(false);
  const {
    data: preview,
    error: previewError,
    isPending,
  } = useQuery<JoinPreview>({
    queryKey: ["join-preview", normalize(code)],
    queryFn: ({ signal }) => previewJoinCode(normalize(code), signal),
    retry: false,
    staleTime: 30_000,
  });
  const error = joinError ?? previewMessage(previewError, t);

  async function onConfirm() {
    if (!user) {
      await google.start({ joinCode: normalize(code) });
      return;
    }
    setJoining(true);
    try {
      await joinClass(normalize(code));
      await navigate("/app", { replace: true });
    } catch (cause) {
      setJoinError(cause instanceof ApiError ? cause.message : t("join.failed"));
      setJoining(false);
    }
  }

  if (error) {
    return (
      <Card className="gap-0 p-5">
        <h1 className="text-lg font-semibold tracking-tight">{t("join.cannotJoin")}</h1>
        <p role="alert" className="text-muted-foreground mt-2 text-sm leading-relaxed">
          {error}
        </p>
        <Button asChild className="mt-4 w-full">
          <Link to="/join">{t("join.tryAnotherCode")}</Link>
        </Button>
      </Card>
    );
  }

  if (isPending || !preview) {
    return (
      <p className="text-muted-foreground text-sm" role="status" aria-live="polite">
        {t("common.loading")}
      </p>
    );
  }

  return (
    <>
      <Card className="gap-0 p-5 text-center">
        <p className="text-muted-foreground text-xs tracking-wide uppercase">
          {t("join.youAreJoining")}
        </p>
        <h1 className="mt-2 text-2xl leading-snug font-semibold tracking-tight text-balance">
          {preview.className}
        </h1>
        <p className="text-muted-foreground mt-2 text-sm">
          {t("join.teacher", { name: preview.teacherName })}
        </p>

        <Separator className="my-5" />

        {google.error ? (
          <p role="alert" className="text-destructive mb-3 text-sm">
            {google.error}
          </p>
        ) : null}

        <Button
          variant={user ? "default" : "outline"}
          size="lg"
          className="w-full"
          disabled={joining || google.pending}
          onClick={() => void onConfirm()}
        >
          {user ? (
            t("join.confirmSignedIn")
          ) : (
            <>
              <GoogleMark />
              {t("join.confirmWithGoogle")}
            </>
          )}
        </Button>

        {user ? null : (
          <p className="text-muted-foreground mt-3 text-xs leading-relaxed">
            {t("join.googleExplainer")}
          </p>
        )}
      </Card>

      <Button asChild variant="ghost" className="text-muted-foreground mt-3 w-full">
        <Link to="/join">
          <ArrowLeft aria-hidden="true" />
          {t("join.anotherCode")}
        </Link>
      </Button>
    </>
  );
}

function previewMessage(error: unknown, t: TFunction): string | null {
  if (error instanceof ApiError) return error.message;
  return error ? t("join.failed") : null;
}
