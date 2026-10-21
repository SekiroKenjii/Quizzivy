import { useRef } from "react";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { formatInTimeZone } from "date-fns-tz";
import { Download } from "lucide-react";
import { FormDialog, type FormField } from "@/components/shared/form/FormDialog";
import { ApiError, failureMessage } from "@/lib/api/errors";
import { useDisplayTimeZone } from "@/lib/i18n/datetime";
import { notify } from "@/lib/toast";
import {
  exportAuditLog,
  type AuditFilters,
  type AuditFormat,
  type AuditPeriod,
} from "../api";
import { hasNarrowing } from "../filters";

type ExportValues = { period: AuditPeriod; format: AuditFormat; filtersOnly: boolean };

const PERIODS: readonly AuditPeriod[] = ["7d", "30d", "year"];
const FORMATS: readonly AuditFormat[] = ["csv", "json"];

const DAY_MS = 86_400_000;

function shift(day: string, days: number): string {
  return new Date(Date.parse(day) + days * DAY_MS).toISOString().slice(0, 10);
}

function startOf({ range, from }: AuditFilters, today: string): string | null {
  if (range === "today") return today;
  if (range === "7d") return shift(today, -6);
  if (range === "30d") return shift(today, -29);
  if (range === "custom") return from;
  return null;
}

function reaches(period: AuditPeriod, start: string | null, today: string): boolean {
  if (start === null) return false;
  if (period === "7d") return start >= shift(today, -6);
  if (period === "30d") return start >= shift(today, -29);
  return start >= `${today.slice(0, 4)}-01-01`;
}

/**
 * ExportAuditDialog asks which period and which file to download. A period is
 * the export's own, so a range or a custom day is not sent: the control for
 * "current filters only" narrows by the category and the search, and shows
 * only while one of them is set, on. A period ends today, so it opens on the
 * shortest one that reaches back to the first day of the range in force,
 * counted in the viewer's zone, or on this year, and says so under the
 * periods when the one chosen does not reach that day. A period of more than
 * 100,000 entries asks for a shorter one. One export runs at a time: while it
 * runs the submit says so, and closing the dialog by any means cancels it.
 */
export function ExportAuditDialog({
  open,
  filters,
  onOpenChange,
}: Readonly<{
  open: boolean;
  filters: AuditFilters;
  onOpenChange: (open: boolean) => void;
}>) {
  const { t } = useTranslation();
  const zone = useDisplayTimeZone();
  const today = formatInTimeZone(new Date(), zone, "yyyy-MM-dd");
  const start = startOf(filters, today);
  const narrowing = hasNarrowing(filters);
  const running = useRef<AbortController | null>(null);
  const download = useMutation({
    mutationFn: ({ values, signal }: { values: ExportValues; signal: AbortSignal }) =>
      exportAuditLog(
        {
          period: values.period,
          format: values.format,
          category: values.filtersOnly ? filters.category : null,
          q: values.filtersOnly ? filters.q.trim() : "",
        },
        signal,
      ),
    onSuccess: () => {
      onOpenChange(false);
      notify.success(t("audit.exportDialog.done"));
    },
    onSettled: (_data, _error, { signal }) => {
      if (running.current?.signal === signal) running.current = null;
    },
  });
  const fields: readonly FormField<ExportValues>[] = [
    {
      kind: "seg",
      name: "period",
      label: t("audit.exportDialog.period"),
      options: PERIODS.map((value) => ({
        value,
        label: t(`audit.exportDialog.periods.${value}`),
      })),
      hint: (values) =>
        reaches(values.period, start, today)
          ? undefined
          : t("audit.exportDialog.beforePeriod"),
    },
    {
      kind: "seg",
      name: "format",
      label: t("audit.exportDialog.format"),
      options: FORMATS.map((value) => ({
        value,
        label: t(`audit.exportDialog.formats.${value}`),
      })),
    },
    {
      kind: "toggle",
      name: "filtersOnly",
      text: t("audit.exportDialog.filtersOnly"),
      sub: t("audit.exportDialog.filtersOnlyHint"),
      when: () => narrowing,
    },
  ];
  const problem = download.error;
  const tooLarge = problem instanceof ApiError && problem.code === "EXPORT_TOO_LARGE";
  const message = tooLarge
    ? t("audit.exportDialog.tooLarge")
    : failureMessage(problem, t("audit.exportDialog.failed"));
  return (
    <FormDialog<ExportValues>
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          running.current?.abort();
          running.current = null;
        }
        download.reset();
        onOpenChange(next);
      }}
      title={t("audit.exportDialog.title")}
      description={t("audit.exportDialog.description")}
      icon={Download}
      initial={{
        period: PERIODS.find((period) => reaches(period, start, today)) ?? "year",
        format: "csv",
        filtersOnly: narrowing,
      }}
      fields={fields}
      submitLabel={
        download.isPending
          ? t("audit.exportDialog.running")
          : t("audit.exportDialog.submit")
      }
      pending={download.isPending}
      error={download.isError ? message : null}
      onSubmit={(values) => {
        if (running.current !== null) return;
        const controller = new AbortController();
        running.current = controller;
        download.mutate({ values, signal: controller.signal });
      }}
    />
  );
}
