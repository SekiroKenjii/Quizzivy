import { useEffect, useRef, type RefObject } from "react";
import { useBlocker } from "react-router";
import { useTranslation } from "react-i18next";
import { KeyRound, Printer, type LucideIcon } from "lucide-react";
import { Callout } from "@/components/shared/Callout";
import { CopyButton } from "@/components/shared/CopyButton";
import {
  DialogShell,
  DialogShellBody,
  DialogShellFooter,
  DialogShellHeader,
} from "@/components/shared/form/DialogShell";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/sonner";
import type { UserPasswordReset } from "@/features/users/api";
import { failureText } from "@/features/users/userText";

/** BulkFailure is an account a bulk action did not reach, named for the reader. */
export type BulkFailure = Readonly<{ userId: string; fullName: string; code: string }>;

function LeaveGuard({ done }: Readonly<{ done: RefObject<HTMLButtonElement | null> }>) {
  const { t } = useTranslation();
  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      currentLocation.pathname !== nextLocation.pathname,
  );
  const blocked = blocker.state === "blocked";
  const reset = blocker.reset;
  useEffect(() => {
    if (!blocked) return;
    reset?.();
    done.current?.focus();
    toast(t("users.credential.leaveFirst"));
  }, [blocked, reset, done, t]);
  useEffect(() => {
    const warn = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, []);
  return null;
}

/**
 * CredentialResult ends a bulk action on the Users screen: the temporary
 * passwords the server returned once, each with Copy, with Copy all and
 * Print; `sent` links, as one line; and every account the action did not
 * reach, with the reason. It closes only through Done or its close button,
 * never on Escape or a click outside, so a password is not lost by accident;
 * Escape moves focus to Done. While it shows a password, leaving the page is
 * refused, in the app with a toast and focus on Done, and by the browser's
 * own prompt on a reload or a closed tab. The caller drops the passwords when
 * it closes.
 */
export function CredentialResult({
  open,
  title,
  icon = KeyRound,
  credentials = [],
  sent = 0,
  failures,
  returnFocus,
  onDone,
}: Readonly<{
  open: boolean;
  title: string;
  icon?: LucideIcon;
  credentials?: readonly UserPasswordReset[];
  sent?: number;
  failures: readonly BulkFailure[];
  returnFocus?: RefObject<HTMLElement | null> | undefined;
  onDone: () => void;
}>) {
  const { t } = useTranslation();
  const done = useRef<HTMLButtonElement>(null);
  const shown = credentials.filter((one) => one.temporaryPassword !== undefined);
  const all = shown
    .map((one) => `${one.fullName}\t${one.email}\t${one.temporaryPassword ?? ""}`)
    .join("\n");
  return (
    <DialogShell
      open={open}
      width={520}
      data-print="credentials"
      returnFocus={returnFocus}
      onOpenChange={(next) => {
        if (!next) onDone();
      }}
      onEscapeKeyDown={(event) => {
        event.preventDefault();
        done.current?.focus();
      }}
      onPointerDownOutside={(event) => event.preventDefault()}
      onInteractOutside={(event) => event.preventDefault()}
    >
      {open && shown.length > 0 ? <LeaveGuard done={done} /> : null}
      <DialogShellHeader
        title={title}
        icon={icon}
        description={shown.length > 0 ? t("users.credential.mustChange") : undefined}
      />
      <DialogShellBody>
        {shown.length === 0 ? null : (
          <>
            <Callout tone="warning" size="md" className="print:hidden">
              {t("users.credential.shownOnce")}
            </Callout>
            <ul
              aria-label={t("users.credential.passwords")}
              className="divide-y rounded-lg border"
            >
              {shown.map((one) => (
                <li
                  key={one.userId}
                  className="flex flex-wrap items-center gap-x-3 gap-y-1 px-3.5 py-2.5"
                >
                  <span className="min-w-0 flex-[1_1_12rem]">
                    <span className="text-ui block leading-normal font-medium break-words">
                      {one.fullName}
                    </span>
                    <span className="text-muted-fg text-meta block leading-normal break-all">
                      {one.email}
                    </span>
                  </span>
                  <code className="text-ui font-mono font-medium select-all">
                    {one.temporaryPassword}
                  </code>
                  <span className="print:hidden">
                    <CopyButton
                      value={one.temporaryPassword ?? ""}
                      label={t("users.credential.copy")}
                      failedMessage={t("users.credential.copyFailed")}
                    />
                  </span>
                </li>
              ))}
            </ul>
          </>
        )}
        {sent > 0 ? (
          <p className="text-ui">{t("users.credential.linksSent", { count: sent })}</p>
        ) : null}
        {failures.length === 0 ? null : (
          <section aria-labelledby="bulk-failures" className="flex flex-col gap-1.5">
            <h3 id="bulk-failures" className="text-ui font-medium">
              {t("users.credential.notReached", { count: failures.length })}
            </h3>
            <ul className="flex flex-col gap-1">
              {failures.map((failure) => (
                <li key={failure.userId} className="text-sm leading-normal">
                  <span className="font-medium">{failure.fullName}</span>
                  {" · "}
                  <span className="text-muted-fg">{failureText(failure.code, t)}</span>
                </li>
              ))}
            </ul>
          </section>
        )}
      </DialogShellBody>
      <DialogShellFooter className="print:hidden">
        {shown.length > 1 ? (
          <CopyButton
            value={all}
            label={t("users.credential.copyAll")}
            failedMessage={t("users.credential.copyFailed")}
          />
        ) : null}
        {shown.length === 0 ? null : (
          <Button type="button" variant="outline" onClick={() => window.print()}>
            <Printer aria-hidden="true" />
            {t("users.credential.print")}
          </Button>
        )}
        <Button ref={done} type="button" onClick={onDone}>
          {t("users.transfer.done")}
        </Button>
      </DialogShellFooter>
    </DialogShell>
  );
}
