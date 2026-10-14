import { useQueryClient } from "@tanstack/react-query";
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ReactNode,
  type Ref,
  type SyntheticEvent,
} from "react";
import { useTranslation } from "react-i18next";
import { CircleAlert, LinkIcon, LoaderCircle, RotateCw } from "lucide-react";
import { Link, useNavigate } from "react-router";

import { PasswordInput } from "@/components/shared/PasswordInput";
import { PasswordRules } from "@/components/shared/PasswordRules";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { AuthLayout } from "@/features/auth/AuthLayout";
import { accountDepartureNeedsRetry } from "@/features/auth/accountDeparture";
import { signInAccount } from "@/features/auth/accountPreferences";
import {
  checkSetPasswordLink,
  setPasswordWithLink,
  type SetPasswordLinkInfo,
} from "@/features/auth/api";
import { GoogleMark } from "@/features/auth/components/GoogleMark";
import { homePathFor } from "@/features/auth/home";
import {
  MAX_LINK_TOKEN_LENGTH,
  setPasswordSchema,
} from "@/features/auth/setPasswordSchema";
import { TransitionPendingError, transitionStatus } from "@/lib/api/authTransition";
import { ApiError } from "@/lib/api/errors";
import { cn } from "@/lib/utils";
import { useAppState } from "@/stores/appState";
import { authStore } from "@/stores/auth";

type Phase =
  | { kind: "checking" }
  | { kind: "invalid" }
  | { kind: "unavailable"; cause: unknown }
  | { kind: "ready"; link: SetPasswordLinkInfo }
  | { kind: "passwordsOff" };

const NEW_FIELD = "set-password-new";
const AGAIN_FIELD = "set-password-again";
const TOO_LONG = "setPassword.errors.tooLong";

function linkTokenIn(hash: string): string {
  return new URLSearchParams(hash.replace(/^#/, "")).get("token") ?? "";
}

function usable(token: string): boolean {
  return token.length > 0 && token.length <= MAX_LINK_TOKEN_LENGTH;
}

function takeLinkToken(): string {
  const token = linkTokenIn(window.location.hash);
  if (window.location.hash !== "") {
    window.history.replaceState(
      window.history.state,
      "",
      window.location.pathname + window.location.search,
    );
  }
  return token;
}

function firstPhase(): Phase {
  return usable(linkTokenIn(window.location.hash))
    ? { kind: "checking" }
    : { kind: "invalid" };
}

function afterCheckFailed(cause: unknown): Phase {
  if (cause instanceof ApiError && (cause.status === 410 || cause.status === 400))
    return { kind: "invalid" };
  return { kind: "unavailable", cause };
}

/**
 * SetPasswordPage is where an emailed set-password link lands (D15, DG-05).
 * It takes the token from the URL's fragment and removes the fragment from the
 * address bar before its first request, holds the token in memory only, and
 * checks it once. A valid link asks for a new password twice; saving it signs
 * the account in as `login` does and goes to the account's home. A link that
 * is unknown, used, expired or revoked, a centre with password sign-in off,
 * and a failed check each get their own state.
 */
export default function SetPasswordPage() {
  const token = useRef<string | null>(null);
  const check = useRef<Promise<SetPasswordLinkInfo> | null>(null);
  const [phase, setPhase] = useState<Phase>(firstPhase);
  const [attempt, setAttempt] = useState(0);
  const heading = useRef<HTMLHeadingElement>(null);
  const shownKind = useRef(phase.kind);

  useEffect(() => {
    token.current ??= takeLinkToken();
    const current = token.current;
    if (!usable(current)) return;
    check.current ??= checkSetPasswordLink(current);
    let live = true;
    check.current.then(
      (link) => {
        if (live) setPhase({ kind: "ready", link });
      },
      (cause: unknown) => {
        if (!live) return;
        check.current = null;
        setPhase(afterCheckFailed(cause));
      },
    );
    return () => {
      live = false;
    };
  }, [attempt]);

  useEffect(() => {
    if (shownKind.current === phase.kind) return;
    shownKind.current = phase.kind;
    heading.current?.focus();
  }, [phase.kind]);

  const readToken = useCallback(() => token.current ?? "", []);
  const retry = useCallback(() => {
    setPhase({ kind: "checking" });
    setAttempt((n) => n + 1);
  }, []);
  const onLinkInvalid = useCallback(() => setPhase({ kind: "invalid" }), []);
  const onPasswordsOff = useCallback(() => setPhase({ kind: "passwordsOff" }), []);

  return (
    <AuthLayout>
      {phase.kind === "checking" && <CheckingLink />}
      {phase.kind === "ready" && (
        <SetPasswordForm
          link={phase.link}
          headingRef={heading}
          readToken={readToken}
          onLinkInvalid={onLinkInvalid}
          onPasswordsOff={onPasswordsOff}
        />
      )}
      {phase.kind === "invalid" && <InvalidLink headingRef={heading} />}
      {phase.kind === "passwordsOff" && <PasswordsOff headingRef={heading} />}
      {phase.kind === "unavailable" && (
        <CheckFailed headingRef={heading} cause={phase.cause} onRetry={retry} />
      )}
    </AuthLayout>
  );
}

function CheckingLink() {
  const { t } = useTranslation();
  return (
    <div aria-busy="true" className="flex flex-col gap-5">
      <p role="status" className="sr-only">
        {t("setPassword.checking")}
      </p>
      <div className="flex flex-col gap-2">
        <Skeleton className="h-7.5 w-4/5" />
        <Skeleton className="h-5 w-full" />
        <Skeleton className="h-5 w-3/5" />
      </div>
      <div className="flex flex-col gap-1.5">
        <Skeleton className="h-4 w-28" />
        <Skeleton className="h-12 w-full rounded-lg" />
      </div>
      <div className="flex flex-col gap-1.5">
        <Skeleton className="h-4 w-36" />
        <Skeleton className="h-12 w-full rounded-lg" />
      </div>
      <Skeleton className="h-12 w-full rounded-lg" />
    </div>
  );
}

function SetPasswordForm({
  link,
  headingRef,
  readToken,
  onLinkInvalid,
  onPasswordsOff,
}: Readonly<{
  link: SetPasswordLinkInfo;
  headingRef: Ref<HTMLHeadingElement>;
  readToken: () => string;
  onLinkInvalid: () => void;
  onPasswordsOff: () => void;
}>) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  const [password, setPassword] = useState("");
  const [again, setAgain] = useState("");
  const [revealed, setRevealed] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const passwordRef = useRef<HTMLInputElement>(null);
  const againRef = useRef<HTMLInputElement>(null);

  const verdict = setPasswordSchema.shape.newPassword.safeParse(password);
  const tooLong = verdict.error?.issues.some((issue) => issue.message === TOO_LONG);
  const valid = verdict.success;
  const mismatch = again !== password && (again !== "" || revealed);
  const ready = valid && again === password;

  async function onSubmit(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    const body = setPasswordSchema.safeParse({
      token: readToken(),
      newPassword: password,
    });
    if (!ready || !body.success) {
      setRevealed(true);
      (valid ? againRef : passwordRef).current?.focus();
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const operation = signInAccount(
        "login",
        (ticket) => setPasswordWithLink(body.data.token, body.data.newPassword, ticket),
        queryClient,
        () => mounted.current,
      );
      const failedLease = authStore.captureActor();
      const result = await operation.catch((cause: unknown) => {
        if (!authStore.isCurrent(failedLease)) {
          throw new DOMException("Superseded", "AbortError");
        }
        throw cause;
      });
      if (!mounted.current || !authStore.isCurrent(result.actor)) return;
      useAppState.getState().setBootPhase("ready");
      await navigate(homePathFor(result.user), { replace: true });
    } catch (cause) {
      if (
        !mounted.current ||
        (cause instanceof DOMException && cause.name === "AbortError")
      )
        return;
      setBusy(false);
      if (cause instanceof ApiError && cause.code === "SET_PASSWORD_LINK_INVALID") {
        onLinkInvalid();
        return;
      }
      if (cause instanceof ApiError && cause.code === "PASSWORD_SIGN_IN_DISABLED") {
        onPasswordsOff();
        return;
      }
      setError(failureFor(cause));
    }
  }

  function failureFor(cause: unknown): string {
    if (cause instanceof TransitionPendingError) {
      if (cause.phase === "cleanup" && accountDepartureNeedsRetry())
        return t("auth.transition.cleanupFailed");
      return t(
        cause.phase === "cleanup"
          ? "auth.transition.cleanupPending"
          : "auth.transition.accountPending",
      );
    }
    if (transitionStatus().phase === "cleanup")
      return t("auth.transition.cleanupFailed");
    return cause instanceof ApiError ? cause.message : t("api.unavailable");
  }

  return (
    <>
      <div>
        <h1
          ref={headingRef}
          tabIndex={-1}
          className="text-h1 [overflow-wrap:anywhere] outline-none!"
        >
          {t("setPassword.title", { email: link.email })}
        </h1>
        <p className="text-muted-fg mt-1 text-base leading-[1.55]">
          {t(`setPassword.body.${link.purpose}`, { name: link.fullName })}
        </p>
      </div>

      <form
        onSubmit={(e) => void onSubmit(e)}
        className="flex flex-col gap-4.5"
        noValidate
      >
        <input
          type="email"
          name="username"
          autoComplete="username"
          value={link.email}
          readOnly
          hidden
          aria-hidden="true"
          tabIndex={-1}
        />
        <div className="flex flex-col gap-1.5">
          <Label htmlFor={NEW_FIELD}>{t("changePassword.new")}</Label>
          <PasswordInput
            ref={passwordRef}
            id={NEW_FIELD}
            size="xl"
            autoComplete="new-password"
            value={password}
            aria-invalid={(revealed && !valid) || undefined}
            aria-describedby={`${NEW_FIELD}-rules`}
            onChange={(e) => {
              setPassword(e.target.value);
              setError(null);
            }}
          />
        </div>
        <PasswordRules
          id={`${NEW_FIELD}-rules`}
          password={password}
          revealed={revealed}
          refusal={tooLong ? t(TOO_LONG) : null}
          replacing={false}
          className="-mt-2.5"
        />

        <div className="flex flex-col gap-1.5">
          <Label htmlFor={AGAIN_FIELD}>{t("changePassword.again")}</Label>
          <PasswordInput
            ref={againRef}
            id={AGAIN_FIELD}
            size="xl"
            autoComplete="new-password"
            value={again}
            aria-invalid={mismatch || undefined}
            aria-describedby={mismatch ? `${AGAIN_FIELD}-mismatch` : undefined}
            onChange={(e) => {
              setAgain(e.target.value);
              setError(null);
            }}
          />
          {mismatch && (
            <p id={`${AGAIN_FIELD}-mismatch`} className="text-danger-ink text-meta">
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
          className={cn("group/set-busy h-12", !ready && "opacity-50")}
          aria-disabled={!ready || undefined}
          aria-busy={busy || undefined}
        >
          {busy && (
            <LoaderCircle
              aria-hidden="true"
              className="size-[17px] animate-spin group-focus-within/set-busy:[animation-play-state:paused] group-hover/set-busy:[animation-play-state:paused] motion-reduce:animate-none"
            />
          )}
          {t(busy ? "setPassword.submitting" : "setPassword.submit")}
        </Button>
      </form>
    </>
  );
}

function StateNotice({
  icon,
  headingRef,
  title,
  children,
  action,
}: Readonly<{
  icon: ReactNode;
  headingRef: Ref<HTMLHeadingElement>;
  title: string;
  children: ReactNode;
  action: ReactNode;
}>) {
  return (
    <div className="flex flex-col gap-[18px]">
      <span className="bg-muted grid size-11 place-items-center rounded-xl">
        {icon}
      </span>
      <div>
        <h1 ref={headingRef} tabIndex={-1} className="text-h1 outline-none!">
          {title}
        </h1>
        <div className="text-muted-fg text-body mt-1.5 text-pretty">{children}</div>
      </div>
      {action}
    </div>
  );
}

function SignInLink({ primary = false }: Readonly<{ primary?: boolean }>) {
  const { t } = useTranslation();
  return (
    <Button
      asChild
      variant={primary ? "default" : "outline"}
      size="xl"
      className={cn("w-full font-medium", !primary && "bg-card hover:bg-muted")}
    >
      <Link to="/login">{t("setPassword.toSignIn")}</Link>
    </Button>
  );
}

function InvalidLink({
  headingRef,
}: Readonly<{ headingRef: Ref<HTMLHeadingElement> }>) {
  const { t } = useTranslation();
  return (
    <StateNotice
      icon={<LinkIcon aria-hidden="true" className="size-5" />}
      headingRef={headingRef}
      title={t("setPassword.invalid.title")}
      action={<SignInLink primary />}
    >
      <p>{t("setPassword.invalid.body")}</p>
    </StateNotice>
  );
}

function PasswordsOff({
  headingRef,
}: Readonly<{ headingRef: Ref<HTMLHeadingElement> }>) {
  const { t } = useTranslation();
  return (
    <StateNotice
      icon={<GoogleMark className="size-5" />}
      headingRef={headingRef}
      title={t("setPassword.passwordsOff.title")}
      action={<SignInLink primary />}
    >
      <p>{t("setPassword.passwordsOff.body")}</p>
    </StateNotice>
  );
}

function CheckFailed({
  headingRef,
  cause,
  onRetry,
}: Readonly<{
  headingRef: Ref<HTMLHeadingElement>;
  cause: unknown;
  onRetry: () => void;
}>) {
  const { t } = useTranslation();
  return (
    <StateNotice
      icon={<CircleAlert aria-hidden="true" className="size-5" />}
      headingRef={headingRef}
      title={t("setPassword.unavailable.title")}
      action={
        <div className="flex flex-col gap-3">
          <Button type="button" size="xl" className="w-full" onClick={onRetry}>
            <RotateCw aria-hidden="true" className="size-[17px]" />
            {t("common.retry")}
          </Button>
          <SignInLink />
        </div>
      }
    >
      <p role="alert">
        {cause instanceof ApiError ? cause.message : t("api.unavailable")}
      </p>
    </StateNotice>
  );
}
