import { useRef, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { KeyRound, Printer } from "lucide-react";
import { Callout } from "@/components/shared/Callout";
import { CopyButton } from "@/components/shared/CopyButton";
import {
  DialogShell,
  DialogShellBody,
  DialogShellFooter,
  DialogShellHeader,
} from "@/components/shared/form/DialogShell";
import { Button } from "@/components/ui/button";
import {
  failureKey,
  type OneTimeCredential,
  type ResetFailure,
} from "@/features/students/credentials";

/**
 * ResetSheet shows temporary passwords the server returned once: each
 * student's name, email and password with its own Copy, a warning that they
 * are shown only now, and Print, which prints this sheet alone. Where the
 * sheet is too narrow for a row on one line, the name and email take a line
 * of their own above the password and Copy. Students a
 * reset did not reach are listed under it with the reason. It closes only
 * through Done or its close button, never on Escape or a click outside, so a
 * password is not lost by accident; Escape moves focus to Done instead, one
 * Enter from the way out. The caller drops the passwords when it closes.
 */
export function ResetSheet({
  open,
  title,
  credentials,
  failures = [],
  returnFocus,
  onDone,
}: Readonly<{
  open: boolean;
  title: string;
  credentials: readonly OneTimeCredential[];
  failures?: readonly ResetFailure[] | undefined;
  returnFocus?: RefObject<HTMLElement | null> | undefined;
  onDone: () => void;
}>) {
  const { t } = useTranslation();
  const shown = credentials.length > 0;
  const done = useRef<HTMLButtonElement>(null);
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
      <DialogShellHeader
        title={title}
        icon={KeyRound}
        description={shown ? t("students.reset.mustChange") : undefined}
      />
      <DialogShellBody>
        {shown ? (
          <>
            <Callout tone="warning" size="md" className="print:hidden">
              {t("students.reset.shownOnce")}
            </Callout>
            <ul
              aria-label={t("students.reset.passwords")}
              className="divide-y rounded-lg border"
            >
              {credentials.map((credential) => (
                <li
                  key={credential.studentId}
                  className="flex flex-wrap items-center gap-x-3 gap-y-1 px-3.5 py-2.5"
                >
                  <span className="min-w-0 flex-[1_1_12rem]">
                    <span className="text-ui block leading-normal font-medium break-words">
                      {credential.fullName}
                    </span>
                    <span className="text-muted-fg text-meta block leading-normal break-all">
                      {credential.email}
                    </span>
                  </span>
                  <code className="text-ui font-mono font-medium select-all">
                    {credential.temporaryPassword}
                  </code>
                  <span className="print:hidden">
                    <CopyButton
                      value={credential.temporaryPassword}
                      label={t("students.copy")}
                      failedMessage={t("students.copyFailed")}
                    />
                  </span>
                </li>
              ))}
            </ul>
            <p className="text-muted-fg text-meta leading-normal print:hidden">
              {t("students.tempWarning")}
            </p>
          </>
        ) : null}
        {failures.length === 0 ? null : (
          <section aria-labelledby="reset-failures" className="flex flex-col gap-1.5">
            <h3 id="reset-failures" className="text-ui font-medium">
              {t("students.reset.notReset", { count: failures.length })}
            </h3>
            <ul className="flex flex-col gap-1">
              {failures.map((failure) => (
                <li key={failure.studentId} className="text-sm leading-normal">
                  <span className="font-medium">{failure.fullName}</span>
                  {" · "}
                  <span className="text-muted-fg">{t(failureKey(failure.code))}</span>
                </li>
              ))}
            </ul>
          </section>
        )}
      </DialogShellBody>
      <DialogShellFooter className="print:hidden">
        {shown ? (
          <Button type="button" variant="outline" onClick={() => window.print()}>
            <Printer aria-hidden="true" />
            {t("students.reset.print")}
          </Button>
        ) : null}
        <Button ref={done} type="button" onClick={onDone}>
          {t("students.done")}
        </Button>
      </DialogShellFooter>
    </DialogShell>
  );
}
