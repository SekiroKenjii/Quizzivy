import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CircleCheck, Download } from "lucide-react";
import { Callout } from "@/components/shared/Callout";
import { ProgressBar } from "@/components/shared/stats/ProgressBar";
import { Button } from "@/components/ui/button";
import {
  downloadUserImportSheet,
  userImportQuery,
  type UserImportJob,
} from "@/features/users/api";
import { failureReason, isJobRunning } from "@/features/users/importText";
import { ApiError, failureMessage } from "@/lib/api/errors";
import type { Locale } from "@/lib/i18n";
import { clockTime } from "@/lib/i18n/datetime";

function SheetPart({ job, locale }: Readonly<{ job: UserImportJob; locale: Locale }>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const download = useMutation({
    mutationFn: () => downloadUserImportSheet(job.id),
    onSettled: () =>
      queryClient.invalidateQueries({ queryKey: userImportQuery(job.id).queryKey }),
  });
  const sheet = job.sheet;
  if (sheet === null) return null;
  const gone = download.error instanceof ApiError && download.error.status === 410;
  if (sheet.downloadedAt !== null)
    return (
      <p className="text-success-ink flex items-center gap-2 text-sm">
        <CircleCheck aria-hidden="true" className="size-4 flex-none" />
        {t("users.import.sheet.downloaded", {
          time: clockTime(sheet.downloadedAt, new Date(), locale),
        })}
      </p>
    );
  if (gone)
    return (
      <Callout tone="danger" size="md" announce>
        {t("users.import.sheet.gone")}
      </Callout>
    );
  if (!sheet.available)
    return (
      <Callout tone="neutral" size="md">
        {t(
          job.created === 0 ? "users.import.sheet.none" : "users.import.sheet.expired",
        )}
      </Callout>
    );
  return (
    <div className="flex flex-col gap-2.5">
      <Callout tone="warning" size="md">
        {t("users.import.sheet.warning", {
          time:
            sheet.expiresAt === null
              ? ""
              : clockTime(sheet.expiresAt, new Date(), locale),
        })}
      </Callout>
      {download.isError && (
        <p role="alert" className="text-danger-ink text-xs">
          {failureMessage(download.error, t("users.import.sheet.failed"))}
        </p>
      )}
      <Button
        type="button"
        className="aria-disabled:bg-primary/50 aria-disabled:hover:bg-primary/50 self-start"
        aria-disabled={download.isPending || undefined}
        onClick={() => {
          if (!download.isPending) download.mutate();
        }}
      >
        <Download aria-hidden="true" />
        {t("users.import.sheet.download")}
      </Button>
    </div>
  );
}

/**
 * ImportProgress follows one import: "Creating accounts… {done}/{total}" with
 * its bar while the worker runs, then how many accounts were made and skipped
 * and how the sign-in details reach people. A sheet is offered once, with a
 * warning before the button that it downloads once and when it is deleted;
 * afterwards the time it was taken, or why it can no longer be. A failed job
 * says why it stopped and that the accounts made before it stay; its sheet,
 * erased when it failed, is not offered.
 */
export function ImportProgress({
  job,
  locale,
}: Readonly<{ job: UserImportJob; locale: Locale }>) {
  const { t } = useTranslation();
  const summary = t("users.import.summary", {
    created: t("users.import.created", { count: job.created }),
    skipped: t("users.import.skipped", { count: job.skipped }),
  });
  if (isJobRunning(job))
    return (
      <div className="flex flex-col gap-2.5">
        <p className="text-ui font-medium tabular-nums">
          {t("users.import.running", { done: job.processed, total: job.total })}
        </p>
        <ProgressBar
          value={job.processed}
          max={job.total}
          label={t("users.import.progress")}
          size={8}
          cap="round"
        />
        <p className="text-muted-fg text-xs leading-[1.45]">
          {t("users.import.leave")}
        </p>
      </div>
    );
  if (job.status === "failed")
    return (
      <div role="status" className="flex flex-col gap-2.5">
        <Callout tone="danger" size="md" lead={t("users.import.failed.title")}>
          {failureReason(job, t)}
        </Callout>
        {job.created > 0 && (
          <p className="text-sm">
            {t("users.import.failed.kept", { count: job.created })}
          </p>
        )}
      </div>
    );
  return (
    <div className="flex flex-col gap-2.5">
      <div role="status" className="flex flex-col gap-1">
        <p className="text-ui flex items-center gap-2 font-medium">
          <CircleCheck
            aria-hidden="true"
            className="text-success-ink size-4 flex-none"
          />
          {summary}
        </p>
        {job.invalid > 0 && (
          <p className="text-muted-fg text-sm">
            {t("users.import.notImported", { count: job.invalid })}
          </p>
        )}
      </div>
      {job.delivery === "email_link" ? (
        job.created > 0 && <p className="text-sm">{t("users.import.emailSent")}</p>
      ) : (
        <SheetPart job={job} locale={locale} />
      )}
    </div>
  );
}
