import { useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { CircleAlert, CircleCheck, GraduationCap, TriangleAlert } from "lucide-react";
import { Callout } from "@/components/shared/Callout";
import { InfoField } from "@/components/shared/form/fields/InfoField";
import type { FormInfoRow } from "@/components/shared/form/fields/types";
import { Button } from "@/components/ui/button";
import type { UserImportIssue, UserImportPreview } from "@/features/users/api";
import { IMPORT_ISSUES_SHOWN, issueReason } from "@/features/users/importText";
import type { Locale } from "@/lib/i18n";

function IssueTable({
  id,
  issues,
}: Readonly<{ id: string; issues: readonly UserImportIssue[] }>) {
  const { t } = useTranslation();
  return (
    <div id={id} className="flex flex-col gap-1.5">
      <div className="overflow-hidden rounded-lg border">
        <table className="w-full text-left text-xs leading-[1.45]">
          <caption className="sr-only">{t("users.import.issues")}</caption>
          <thead className="bg-muted text-muted-fg">
            <tr>
              <th scope="col" className="px-2.5 py-1.5 font-medium whitespace-nowrap">
                {t("users.import.issueRow")}
              </th>
              <th scope="col" className="px-2.5 py-1.5 font-medium whitespace-nowrap">
                {t("users.import.issueColumn")}
              </th>
              <th scope="col" className="w-full px-2.5 py-1.5 font-medium">
                {t("users.import.issueReason")}
              </th>
            </tr>
          </thead>
          <tbody>
            {issues.map((issue, index) => (
              <tr key={index} className="border-t align-top">
                <td className="px-2.5 py-1.5 font-medium whitespace-nowrap tabular-nums">
                  {issue.row}
                </td>
                <td className="px-2.5 py-1.5 whitespace-nowrap">
                  <code className="font-mono">{issue.field}</code>
                </td>
                <td className="px-2.5 py-1.5">{issueReason(issue.code, t)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="text-muted-fg text-xs leading-[1.45]">
        {issues.length >= IMPORT_ISSUES_SHOWN
          ? t("users.import.issuesCapped", { count: IMPORT_ISSUES_SHOWN })
          : t("users.import.issuesHint")}
      </p>
    </div>
  );
}

/**
 * ImportPreview draws what a file would come to: the accounts it would create,
 * the addresses that already have one and will be skipped (with up to five of
 * them), the classes it adds people to, and the rows refused, whose cells open
 * under a disclosure with their row, column and reason. A file that would
 * create no account says so, since Import then stays paused. Under email
 * delivery, a file with more new accounts than today's sending budget says the
 * rest go out on the following days.
 */
export function ImportPreview({
  preview,
  locale,
}: Readonly<{ preview: UserImportPreview; locale: Locale }>) {
  const { t } = useTranslation();
  const issuesId = useId();
  const [showIssues, setShowIssues] = useState(false);
  const list = new Intl.ListFormat(locale, { type: "conjunction" });
  const existing = preview.existingEmails;
  const rows: FormInfoRow[] = [
    {
      icon: CircleCheck,
      tone: "success",
      text: t("users.import.newAccounts", { count: preview.newAccounts }),
    },
  ];
  if (existing.count > 0)
    rows.push({
      icon: CircleAlert,
      tone: "warning",
      text: (
        <>
          {t("users.import.existing", { count: existing.count })}
          {existing.sample.length > 0 && (
            <span className="text-muted-fg block text-xs break-all">
              {t("users.import.existingSample", {
                emails:
                  existing.sample.join(", ") +
                  (existing.count > existing.sample.length ? ", …" : ""),
              })}
            </span>
          )}
        </>
      ),
    });
  if (preview.classes.length > 0)
    rows.push({
      icon: GraduationCap,
      text: t("users.import.classes", {
        classes: list.format(preview.classes.map((one) => one.name)),
      }),
    });
  if (preview.invalidCount > 0)
    rows.push({
      icon: TriangleAlert,
      tone: "danger",
      text: t("users.import.invalid", { count: preview.invalidCount }),
      ...(preview.invalid.length > 0 && {
        action: {
          label: t(showIssues ? "users.import.hideIssues" : "users.import.showIssues"),
          onAction: () => setShowIssues((shown) => !shown),
          expanded: showIssues,
          controls: issuesId,
        },
      }),
    });
  const budget = preview.emailBudget;
  return (
    <div className="flex flex-col gap-2">
      <InfoField rows={rows} />
      {showIssues && preview.invalid.length > 0 && (
        <IssueTable id={issuesId} issues={preview.invalid} />
      )}
      {preview.newAccounts === 0 && (
        <Callout tone="neutral" size="sm">
          {t("users.import.refusal.nothingToCreate")}
        </Callout>
      )}
      {budget !== null && preview.newAccounts > budget.remainingToday && (
        <Callout tone="warning" size="sm">
          {t("users.import.budget", { count: budget.remainingToday })}
        </Callout>
      )}
    </div>
  );
}

/** ImportPreviewSkeleton stands in for the preview while the file is read. */
export function ImportPreviewSkeleton() {
  const { t } = useTranslation();
  return (
    <p className="text-muted-fg flex items-center gap-2 rounded-lg border px-3 py-2.5 text-sm">
      <span
        className="bg-muted size-3.75 flex-none animate-pulse rounded-full"
        aria-hidden="true"
      />
      {t("users.import.reading")}
    </p>
  );
}

/** ImportPreviewError says why a file was refused, with Retry when the same file may yet be read. */
export function ImportPreviewError({
  message,
  onRetry,
}: Readonly<{ message: string; onRetry?: (() => void) | undefined }>) {
  const { t } = useTranslation();
  return (
    <Callout
      tone="danger"
      size="md"
      announce
      actions={
        onRetry === undefined ? undefined : (
          <Button type="button" variant="outline" size="sm" onClick={onRetry}>
            {t("common.retry")}
          </Button>
        )
      }
    >
      {message}
    </Callout>
  );
}
