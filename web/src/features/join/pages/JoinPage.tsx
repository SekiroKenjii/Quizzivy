import { useEffect, useRef, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { LoaderCircle } from "lucide-react";
import { Link, Navigate, useLocation, useNavigate, useParams } from "react-router";
import type { TFunction } from "i18next";

import { Button } from "@/components/ui/button";
import { AuthLayout } from "@/features/auth/AuthLayout";
import { homePathFor } from "@/features/auth/home";
import { joinClass, previewJoinCode } from "@/features/join/api";
import { clean, CODE_LENGTH, hasExcluded, normalize } from "@/features/join/code";
import {
  clearJoinContext,
  joinOutcomeState,
  readJoinContext,
  readJoinOutcome,
  saveJoinContext,
  type JoinContext,
  type JoinOutcome,
} from "@/features/join/context";
import { ClassPreviewCard } from "@/features/join/components/ClassPreviewCard";
import { JoinCodeField } from "@/features/join/components/JoinCodeField";
import { JoinedState } from "@/features/join/components/JoinedState";
import { ApiError, failureMessage } from "@/lib/api/errors";
import { useAuthStore } from "@/stores/auth";

const LOOKUP_DELAY_MS = 250;
const ERROR_ID = "join-code-error";

/**
 * JoinPage is §6.2's join: `/join` to type a code, `/join/:code` from a link
 * or a QR code with the code filled in. A valid code shows its class before
 * anything authenticates; joining sends a signed-out visitor to sign in with
 * the class remembered, and the sign-in comes back here to finish. The outcome
 * arrives in the location state and replaces the form.
 */
export default function JoinPage() {
  const location = useLocation();
  const { code = "" } = useParams();
  const outcome = readJoinOutcome(location.state);
  if (outcome) {
    return (
      <AuthLayout panel={false}>
        <JoinedState outcome={outcome} />
      </AuthLayout>
    );
  }
  return <JoinForm key={code} initial={code} />;
}

function JoinForm({ initial }: Readonly<{ initial: string }>) {
  const user = useAuthStore((s) => s.user);
  const isBootstrapping = useAuthStore((s) => s.isBootstrapping);
  const [resumable] = useState(() => {
    const context = readJoinContext();
    return context && context.code === normalize(initial) ? context : null;
  });
  if (resumable && (isBootstrapping || user?.role === "student")) {
    return <ResumeJoin context={resumable} />;
  }
  return <CodeEntry initial={initial} />;
}

function ResumeJoin({ context }: Readonly<{ context: JoinContext }>) {
  const { t } = useTranslation();
  const isStudent = useAuthStore((s) => s.user?.role === "student");
  const mustChangePassword = useAuthStore((s) => s.user?.mustChangePassword === true);
  const { mutate } = useEnrol();
  const started = useRef(false);
  useEffect(() => {
    if (!isStudent || mustChangePassword || started.current) return;
    started.current = true;
    mutate(context);
  }, [context, isStudent, mustChangePassword, mutate]);

  if (isStudent && mustChangePassword)
    return <Navigate to="/change-password" replace />;
  return (
    <AuthLayout panel={false}>
      <p
        role="status"
        className="text-muted-fg flex items-center justify-center gap-2.5"
      >
        <LoaderCircle aria-hidden="true" className="size-[17px] animate-spin" />
        {t("join.joining", { className: context.className })}
      </p>
    </AuthLayout>
  );
}

function CodeEntry({ initial }: Readonly<{ initial: string }>) {
  const { t } = useTranslation();
  const signedIn = useAuthStore((s) => s.user !== null);
  const [code, setCode] = useState(() => clean(initial));
  const excluded = hasExcluded(code);
  const lookup = useSettled(code.length === CODE_LENGTH && !excluded ? code : null);
  const preview = useQuery({
    queryKey: ["join-preview", lookup],
    queryFn: ({ signal }) => previewJoinCode(lookup ?? "", signal),
    enabled: lookup !== null,
    retry: false,
    staleTime: 30_000,
  });

  const found = lookup !== null ? preview.data : undefined;
  let message: string | null = null;
  if (excluded) message = t("join.excluded");
  else if (lookup !== null && preview.isError)
    message = lookupMessage(preview.error, t);

  return (
    <AuthLayout panel={false}>
      <div className="flex flex-col gap-4.5">
        <div>
          <h1 className="text-h1">{t("join.title")}</h1>
          <p className="text-muted-fg mt-1 text-base">{t("join.subtitle")}</p>
        </div>

        <JoinCodeField
          value={code}
          onChange={setCode}
          tone={toneFor(message, found !== undefined)}
          errorId={message ? ERROR_ID : undefined}
        />
        {message && (
          <p id={ERROR_ID} role="alert" className="text-danger-ink text-ui -mt-2">
            {message}
          </p>
        )}
        <p role="status" className="sr-only">
          {lookup !== null && preview.isFetching ? t("join.checking") : ""}
        </p>

        {found && (
          <FoundClass
            context={{
              code,
              className: found.className,
              teacherName: found.teacherName,
            }}
          />
        )}

        {!signedIn && (
          <p className="text-muted-fg text-ui text-center">
            {t("join.alreadyInClass")}{" "}
            <Link
              to="/login"
              onClick={clearJoinContext}
              className="text-fg font-medium underline underline-offset-[3px]"
            >
              {t("join.signIn")}
            </Link>
          </p>
        )}
      </div>
    </AuthLayout>
  );
}

function FoundClass({ context }: Readonly<{ context: JoinContext }>) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const user = useAuthStore((s) => s.user);
  const enrol = useEnrol();

  function onJoin() {
    if (enrol.isPending) return;
    if (user) {
      enrol.mutate(context);
      return;
    }
    saveJoinContext(context);
    void navigate("/login");
  }

  return (
    <>
      <ClassPreviewCard name={context.className} teacherName={context.teacherName} />
      {user && user.role !== "student" ? (
        <div className="flex flex-col gap-3">
          <p className="text-muted-fg text-ui text-center">{t("join.studentsOnly")}</p>
          <Button
            asChild
            variant="outline"
            size="xl"
            className="bg-card hover:bg-muted"
          >
            <Link to={homePathFor(user)}>{t("join.goHome")}</Link>
          </Button>
        </div>
      ) : (
        <Button
          type="button"
          size="xl"
          className="h-12"
          aria-busy={enrol.isPending || undefined}
          onClick={onJoin}
        >
          {enrol.isPending && (
            <LoaderCircle aria-hidden="true" className="size-[17px] animate-spin" />
          )}
          <span className="min-w-0 truncate">
            {t("join.joinClass", { className: context.className })}
          </span>
        </Button>
      )}
      {!user && (
        <p className="text-muted-fg text-meta -mt-2 text-center">
          {t("join.nextStep")}
        </p>
      )}
    </>
  );
}

function useSettled(code: string | null): string | null {
  const [settled, setSettled] = useState(code);
  useEffect(() => {
    if (code === null) return;
    const timer = setTimeout(() => setSettled(code), LOOKUP_DELAY_MS);
    return () => clearTimeout(timer);
  }, [code]);
  return code !== null && settled === code ? code : null;
}

function useEnrol() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  function finish(code: string, outcome: JoinOutcome) {
    clearJoinContext();
    void navigate(`/join/${code}`, { replace: true, state: joinOutcomeState(outcome) });
  }
  return useMutation({
    mutationFn: (context: JoinContext) => joinClass(context.code),
    onSuccess: (joined, context) =>
      finish(context.code, {
        kind: "joined",
        className: joined.name,
        teacherName: context.teacherName,
      }),
    onError: (cause, context) =>
      finish(
        context.code,
        cause instanceof ApiError && cause.code === "ALREADY_ENROLLED"
          ? {
              kind: "joined",
              className: context.className,
              teacherName: context.teacherName,
            }
          : {
              kind: "failed",
              className: context.className,
              message: failureMessage(cause, t("join.failed")),
            },
      ),
  });
}

function lookupMessage(error: unknown, t: TFunction): string {
  if (error instanceof ApiError && error.isRateLimited) return error.message;
  if (error instanceof ApiError && error.status === 404) return t("join.notFound");
  return t("join.lookupFailed");
}

function toneFor(message: string | null, found: boolean) {
  if (message) return "danger";
  return found ? "success" : "idle";
}
