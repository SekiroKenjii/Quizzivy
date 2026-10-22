import { useId, useRef, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { Upload } from "lucide-react";
import {
  DialogShell,
  DialogShellBody,
  DialogShellFooter,
  DialogShellHeader,
} from "@/components/shared/form/DialogShell";
import { FileDrop } from "@/components/shared/form/FileDrop";
import { SegField } from "@/components/shared/form/fields/SegField";
import { ToggleField } from "@/components/shared/form/fields/ToggleField";
import { Button } from "@/components/ui/button";
import type { UserImportDefaultRole } from "@/features/users/api";
import { ImportProgress } from "@/features/users/components/ImportProgress";
import {
  ImportPreview,
  ImportPreviewError,
  ImportPreviewSkeleton,
} from "@/features/users/components/ImportPreview";
import {
  IMPORT_ACCEPT,
  isJobRunning,
  isRetryable,
  refusalMessage,
} from "@/features/users/importText";
import { useUserImport, type UserImport } from "@/features/users/useUserImport";
import { useLocale } from "@/lib/i18n/useLocale";
import { cn } from "@/lib/utils";

const FOOTER_BUTTON = "text-ui leading-normal shadow-none h-9 px-3.5";

function ChooseBody({ run }: Readonly<{ run: UserImport }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const id = useId();
  const { preview } = run;
  let shown = null;
  if (run.tooLarge)
    shown = <ImportPreviewError message={t("users.import.refusal.tooLarge")} />;
  else if (run.start.isError)
    shown = (
      <ImportPreviewError
        message={refusalMessage(run.start.error, t)}
        onRetry={isRetryable(run.start.error) ? run.begin : undefined}
      />
    );
  else if (preview.isError)
    shown = (
      <ImportPreviewError
        message={refusalMessage(preview.error, t)}
        onRetry={isRetryable(preview.error) ? () => void preview.refetch() : undefined}
      />
    );
  else if (preview.data !== undefined)
    shown = <ImportPreview preview={preview.data} locale={locale} />;
  else if (run.file !== null) shown = <ImportPreviewSkeleton />;
  const locked = run.start.isPending;
  return (
    <>
      <div className="flex flex-col gap-1.5">
        <label htmlFor={`${id}-file`} className="text-meta font-medium">
          {t("users.import.file")}
        </label>
        <FileDrop
          id={`${id}-file`}
          value={run.file}
          onChange={run.choose}
          accept={IMPORT_ACCEPT}
          placeholder={t("users.import.choose")}
          limits={t("users.import.limits")}
          invalid={run.tooLarge}
          disabled={locked}
        />
      </div>
      {run.file !== null && (
        <section
          aria-label={t("users.import.preview")}
          aria-live="polite"
          className="flex flex-col gap-1.5"
        >
          <h3 className="text-meta font-medium">{t("users.import.preview")}</h3>
          {shown}
        </section>
      )}
      <div className="flex flex-col gap-1.5">
        <span className="text-meta font-medium">{t("users.import.role")}</span>
        <SegField
          id={`${id}-role-field`}
          label={t("users.import.role")}
          value={run.defaultRole}
          onChange={(value) => run.setDefaultRole(value as UserImportDefaultRole)}
          disabled={locked}
          options={[
            { value: "student", label: t("users.import.roleStudent") },
            { value: "teacher", label: t("users.import.roleTeacher") },
          ]}
        />
      </div>
      <ToggleField
        id={`${id}-delivery`}
        label={t("users.import.deliveryText")}
        text={t("users.import.deliveryText")}
        sub={t("users.import.deliverySub")}
        value={run.delivery === "email_link"}
        onChange={(email) => run.setDelivery(email ? "email_link" : "sheet")}
        disabled={locked}
      />
    </>
  );
}

/**
 * ImportUsersDialog is the deck's "Import users" (T-R5.23), 520px: a CSV or
 * XLSX file, the role for rows that name none, and whether sign-in details go
 * by email or on a sheet to hand out (DG-05). Choosing a file, or changing
 * either choice, reads the preview; "Import {n} people" starts the job, and
 * stays paused while there is nothing to create. The dialog then follows the
 * job to its end and offers the sheet once. It may be closed at any point:
 * the job carries on, the Users list is read again when it settles, and
 * opening the dialog again shows it while it runs or its sheet can still be
 * taken. The page keeps it mounted so that this holds while it is closed.
 */
export function ImportUsersDialog({
  open,
  onOpenChange,
  returnFocus,
}: Readonly<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  returnFocus?: RefObject<HTMLElement | null> | undefined;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const run = useUserImport(open);
  const body = useRef<HTMLDivElement>(null);
  const job = run.job.data;
  const preview = run.preview.data;
  const ready =
    preview !== undefined &&
    !run.preview.isFetching &&
    !run.preview.isError &&
    preview.newAccounts > 0;
  const paused = !ready || run.start.isPending;

  const close = (next: boolean) => {
    onOpenChange(next);
    if (!next) run.dismiss();
  };

  const choosing = run.jobId === null;
  const settled = job !== undefined && !isJobRunning(job);
  let primaryLabel = settled ? t("users.import.done") : t("common.close");
  if (choosing)
    primaryLabel = ready
      ? t("users.import.submitCount", { count: preview.newAccounts })
      : t("users.import.submit");
  let secondary = null;
  if (choosing)
    secondary = (
      <Button
        type="button"
        variant="outline"
        className={cn("bg-card", FOOTER_BUTTON)}
        onClick={() => close(false)}
      >
        {t("common.cancel")}
      </Button>
    );
  else if (settled)
    secondary = (
      <Button
        type="button"
        variant="outline"
        className={cn("bg-card", FOOTER_BUTTON)}
        onClick={run.again}
      >
        {t("users.import.again")}
      </Button>
    );

  const readFailed = run.job.isError && !run.job.isFetching && (
    <ImportPreviewError
      message={t("users.import.progressFailed")}
      onRetry={() => void run.job.refetch()}
    />
  );
  let content;
  if (choosing) content = <ChooseBody run={run} />;
  else if (job !== undefined)
    content = (
      <>
        <ImportProgress
          job={job}
          locale={locale}
          sheetSaved={run.sheetSaved}
          onSheetSaved={run.saveSheet}
        />
        {readFailed}
      </>
    );
  else content = readFailed || <ImportPreviewSkeleton />;

  return (
    <DialogShell
      open={open}
      onOpenChange={close}
      width={520}
      returnFocus={returnFocus}
      onOpenAutoFocus={(event) => {
        const first = body.current?.querySelector<HTMLElement>("button:not(:disabled)");
        if (!first) return;
        event.preventDefault();
        first.focus();
      }}
    >
      <DialogShellHeader
        title={t("users.import.title")}
        description={t("users.import.columns")}
        icon={Upload}
      />
      <DialogShellBody ref={body}>{content}</DialogShellBody>
      <DialogShellFooter>
        {secondary}
        <Button
          type="button"
          aria-disabled={(choosing && paused) || undefined}
          className={cn(
            FOOTER_BUTTON,
            "aria-disabled:bg-primary/55 aria-disabled:hover:bg-primary/55 px-4",
          )}
          onClick={() => {
            if (!choosing) close(false);
            else if (!paused) run.begin();
          }}
        >
          {primaryLabel}
        </Button>
      </DialogShellFooter>
    </DialogShell>
  );
}
