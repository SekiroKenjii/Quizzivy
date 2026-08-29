import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Link, useNavigate, useParams } from "react-router";
import { Button } from "@/components/ui/button";
import { GoogleMark } from "@/features/auth/components/GoogleMark";
import { previewJoinCode, joinClass, type JoinPreview } from "@/features/join/api";
import { format, normalize } from "@/features/join/code";
import { useGoogleSignIn } from "@/features/auth/google/useGoogleSignIn";
import { ApiError } from "@/lib/api/errors";
import { useAuthStore } from "@/stores/auth";

/**
 * §6.2's confirm step, and the reason it exists: "the student sees WHICH class
 * they are joining before authenticating. Never create an account and enrol in
 * one blind tap."
 *
 * So the preview happens first and unauthenticated, and the primary button is
 * the only thing that starts an account. §12 asks for this shape specifically --
 * "single centered card, class name large, one primary button... calm and
 * legitimate, not a marketing page" -- because it is the first Quizzivy screen
 * a new student ever sees.
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
  const error =
    joinError ??
    (previewError instanceof ApiError
      ? previewError.message
      : previewError
        ? t("join.failed")
        : null);

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
      <div className="w-full max-w-sm rounded-lg border p-6 text-center">
        <h1 className="text-xl font-semibold tracking-tight">{t("join.cannotJoin")}</h1>
        <p role="alert" className="text-muted-foreground mt-3 text-sm leading-relaxed">
          {error}
        </p>
        <Button asChild variant="outline" className="mt-6 w-full">
          <Link to="/join">{t("join.tryAnotherCode")}</Link>
        </Button>
      </div>
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
    <div className="w-full max-w-sm rounded-lg border p-6 text-center">
      <p className="text-muted-foreground text-sm">{t("join.youAreJoining")}</p>
      <h1 className="mt-2 text-2xl font-semibold tracking-tight text-balance">
        {preview.className}
      </h1>
      <p className="text-muted-foreground mt-2 text-sm">
        {t("join.teacher", { name: preview.teacherName })}
      </p>
      <p className="text-muted-foreground mt-1 font-mono text-xs">{format(code)}</p>

      {google.error ? (
        <p role="alert" className="text-destructive mt-4 text-sm">
          {google.error}
        </p>
      ) : null}

      <Button
        className="mt-6 w-full"
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

      {!user ? (
        <p className="text-muted-foreground mt-4 text-xs leading-relaxed">
          {t("join.googleExplainer")}
        </p>
      ) : null}
    </div>
  );
}
