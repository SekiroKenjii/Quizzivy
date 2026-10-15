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

function periodFor(filters: AuditFilters): AuditPeriod {
  if (filters.range === "today" || filters.range === "7d") return "7d";
  return filters.range === "30d" ? "30d" : "year";
}

/**
 * ExportAuditDialog asks which period and which file to download. A period is
 * the export's own, so a range or a custom day is not sent: the control for
 * "current filters only" narrows by the category and the search, and shows
 * only while one of them is set, on. A period of more than 100,000 entries
 * asks for a shorter one.
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
      onSubmit={(values) => download.mutate(values)}
    />
  );
}
