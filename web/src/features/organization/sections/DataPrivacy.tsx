import { useId, useRef, useState } from "react";
import { Controller, useFormContext } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { Download } from "lucide-react";
import { SettingsCard } from "@/components/shared/SettingsCard";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ExportDataDialog } from "@/features/organization/components/ExportDataDialog";
import { controlId } from "@/features/organization/controls";
import {
  FOREVER,
  retentionMonths,
  retentionValue,
} from "@/features/organization/retention";
import type { SettingsFormValues } from "@/features/organization/settingsFormSchema";
import { useAuthStore } from "@/stores/auth";

const ROWS = [
  {
    name: "keepAnswersMonths",
    label: "keepAnswers",
    hint: "keepAnswersHint",
    options: [12, 24, FOREVER],
    labels: ["year1", "year2", "forever"],
  },
  {
    name: "keepFocusEventsMonths",
    label: "keepFocus",
    hint: "keepFocusHint",
    options: [6, 12, 13, 24],
    labels: ["month6", "year1", "month13", "year2"],
  },
  {
    name: "anonymiseDisabledAfterMonths",
    label: "anonymise",
    hint: "anonymiseHint",
    options: [FOREVER, 12, 24],
    labels: ["never", "year1", "year2"],
  },
] as const;

function RetentionRow({
  row,
  saving,
}: Readonly<{ row: (typeof ROWS)[number]; saving: boolean }>) {
  const { t } = useTranslation();
  const { control } = useFormContext<SettingsFormValues>();
  const id = controlId(`retention.${row.name}`);
  return (
    <Controller
      control={control}
      name={`retention.${row.name}`}
      render={({ field }) => (
        <div className="flex flex-wrap items-center gap-x-3.5 gap-y-2 px-4.5 py-3">
          <span className="min-w-52 flex-1">
            <span
              id={`${id}-label`}
              className="text-ui block leading-normal font-medium"
            >
              {t(`systemSettings.data.${row.label}`)}
            </span>
            <span
              id={`${id}-hint`}
              className="text-muted-fg text-meta block leading-normal"
            >
              {t(`systemSettings.data.${row.hint}`)}
            </span>
          </span>
          <Select
            value={retentionValue(field.value)}
            onValueChange={(next) => {
              if (!saving) field.onChange(retentionMonths(next));
            }}
          >
            <SelectTrigger
              id={id}
              ref={field.ref}
              aria-labelledby={`${id}-label`}
              aria-describedby={`${id}-hint`}
              aria-disabled={saving || undefined}
              className="aria-disabled:text-foreground/50 w-44"
              onBlur={field.onBlur}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {row.options.map((option, index) => (
                <SelectItem key={option} value={String(option)}>
                  {t(`systemSettings.data.options.${row.labels[index]}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}
    />
  );
}

/**
 * DataPrivacySection is the Data & privacy cards of System settings: how long
 * the centre keeps attempt answers, focus-loss events and a disabled
 * account's name and email, as three selects that join the page's unsaved
 * changes, and Export all data, a danger-bordered card whose button opens the
 * request dialog and is not part of the form. Shortening a retention is
 * confirmed by the page before it is saved.
 */
export function DataPrivacySection({ saving }: Readonly<{ saving: boolean }>) {
  const { t } = useTranslation();
  const email = useAuthStore((state) => state.user?.email ?? "");
  const [open, setOpen] = useState(false);
  const opener = useRef<HTMLButtonElement>(null);
  const titleId = useId();
  return (
    <>
      <SettingsCard
        title={t("systemSettings.data.title")}
        description={t("systemSettings.data.hint")}
      >
        <div className="divide-y">
          {ROWS.map((row) => (
            <RetentionRow key={row.name} row={row} saving={saving} />
          ))}
        </div>
      </SettingsCard>
      <section
        aria-labelledby={titleId}
        className="bg-card shadow-card border-danger-soft flex flex-wrap items-center gap-x-3.5 gap-y-3 rounded-xl border px-4.5 py-4"
      >
        <div className="min-w-52 flex-1">
          <h2
            id={titleId}
            className="text-danger-ink text-md leading-normal font-semibold"
          >
            {t("systemSettings.export.title")}
          </h2>
          <p className="text-muted-fg text-sm leading-normal">
            {t("systemSettings.export.hint")}
          </p>
        </div>
        <Button
          ref={opener}
          type="button"
          variant="outline"
          className="h-8.5"
          onClick={() => setOpen(true)}
        >
          <Download aria-hidden="true" />
          {t("systemSettings.export.request")}
        </Button>
      </section>
      <ExportDataDialog
        open={open}
        email={email}
        onOpenChange={setOpen}
        returnFocus={opener}
      />
    </>
  );
}
