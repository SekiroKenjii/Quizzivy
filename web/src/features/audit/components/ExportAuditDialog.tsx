import { useRef } from "react";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Download } from "lucide-react";
import { FormDialog, type FormField } from "@/components/shared/form/FormDialog";
import { ApiError, failureMessage } from "@/lib/api/errors";
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

function periodFor({ range, from, to }: AuditFilters): AuditPeriod {
  if (range === "today" || range === "7d") return "7d";
  if (range === "30d") return "30d";
  if (range === "custom" && from !== null && to !== null) {
    const days = (Date.parse(to) - Date.parse(from)) / DAY_MS + 1;
    if (days <= 7) return "7d";
    if (days <= 30) return "30d";
  }
  return "year";
}

/**
 * ExportAuditDialog asks which period and which file to download. A period is
 * the export's own, so a range or a custom day is not sent: the control for
 * "current filters only" narrows by the category and the search, and shows
 * only while one of them is set, on. It opens on the shortest period that
 * covers the range in force. A period of more than 100,000 entries asks for a
 * shorter one. One export runs at a time, and the dialog stays open until it
 * ends.
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
  const narrowing = hasNarrowing(filters);
  const running = useRef(false);
  const download = useMutation({
    mutationFn: (values: ExportValues) =>
      exportAuditLog({
        period: values.period,
        format: values.format,
        category: values.filtersOnly ? filters.category : null,
        q: values.filtersOnly ? filters.q.trim() : "",
      }),
    onSuccess: () => {
      onOpenChange(false);
      notify.success(t("audit.exportDialog.done"));
    },
    onSettled: () => {
      running.current = false;
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
        if (!next && running.current) return;
        download.reset();
        onOpenChange(next);
      }}
      title={t("audit.exportDialog.title")}
      description={t("audit.exportDialog.description")}
      icon={Download}
      initial={{ period: periodFor(filters), format: "csv", filtersOnly: narrowing }}
      fields={fields}
      submitLabel={t("audit.exportDialog.submit")}
      pending={download.isPending}
      error={download.isError ? message : null}
      onSubmit={(values) => {
        if (running.current) return;
        running.current = true;
        download.mutate(values);
      }}
    />
  );
}
