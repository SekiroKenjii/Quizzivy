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
import { ApiError } from "@/lib/api/errors";
import type { Locale } from "@/lib/i18n";
import { clockTime } from "@/lib/i18n/datetime";

type SheetOutcome = "saved" | "gone" | "lost" | "failed";

function SheetPart({ job, locale }: Readonly<{ job: UserImportJob; locale: Locale }>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const query = userImportQuery(job.id);
  const download = useMutation({
    mutationFn: async (): Promise<SheetOutcome> => {
      try {
        await downloadUserImportSheet(job.id);
        return "saved";
      } catch (cause) {
        if (cause instanceof ApiError && cause.status === 410) return "gone";
        const now = await queryClient
          .query({ ...query, staleTime: 0 })
          .catch(() => undefined);
        return now?.sheet?.downloadedAt == null ? "failed" : "lost";
      }
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: query.queryKey }),
  });
  const sheet = job.sheet;
  if (sheet === null) return null;
  const outcome = download.data;
  if (outcome === "saved")
    return (
      <p role="status" className="text-success-ink flex items-center gap-2 text-sm">
        <CircleCheck aria-hidden="true" className="size-4 flex-none" />
        {t("users.import.sheet.downloaded", {
          time: clockTime(sheet.downloadedAt ?? new Date(), new Date(), locale),
        })}
      </p>
    );
  if (outcome === "gone" || outcome === "lost")
    return (
      <Callout tone="danger" size="md" announce>
        {t(outcome === "gone" ? "users.import.sheet.gone" : "users.import.sheet.lost")}
      </Callout>
    );
  if (sheet.downloadedAt !== null)
    return (
      <Callout tone="neutral" size="md">
        {t("users.import.sheet.taken", {
          time: clockTime(sheet.downloadedAt, new Date(), locale),
        })}
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
      {outcome === "failed" && (
        <p role="alert" className="text-danger-ink text-xs leading-[1.45]">
          {t("users.import.sheet.retry")}
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
 * warning before the button that it downloads once and when it is deleted.
 * It reads as downloaded only once the whole file has arrived and been handed
 * to the browser; a download that fails after the server erased the sheet
 * says so and that the accounts need a password reset, as does a sheet taken
 * elsewhere. A failed job says why it stopped and that the accounts made
 * before it stay; under a sheet, those accounts have no password, because the
 * sheet is erased when the job fails, and it says to reset them.
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
          {t(
            job.delivery === "sheet" ? "users.import.leaveSheet" : "users.import.leave",
          )}
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
            {t(
              job.delivery === "sheet"
                ? "users.import.failed.keptNoPassword"
                : "users.import.failed.kept",
              { count: job.created },
            )}
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
