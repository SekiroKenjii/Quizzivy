import { useId, useState, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { FormDialog, type FormField } from "@/components/shared/form/FormDialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "@/components/ui/sonner";
import { useRequestDataExport } from "@/features/exports/api";
import { ApiError, failureMessage } from "@/lib/api/errors";

type Values = { includeAnswers: boolean };

const INITIAL: Values = { includeAnswers: false };

function refusal(cause: unknown, t: TFunction): string {
  if (!(cause instanceof ApiError)) return failureMessage(cause, t("api.failed"));
  if (cause.code === "EXPORT_IN_PROGRESS") return t("systemSettings.export.inProgress");
  if (cause.code === "EXPORT_OFF") return t("systemSettings.export.off");
  if (cause.status === 429) {
    const minutes = Math.max(1, Math.ceil((cause.retryAfterSeconds ?? 60) / 60));
    return t("systemSettings.export.rateLimited", { count: minutes });
  }
  return cause.message;
}

/**
 * ExportDataDialog is "Export all data?": the address the link goes to, which
 * is the requester's own and cannot be changed, and whether the file holds
 * attempt answers, off by default. A request that is made closes the dialog
 * with a toast naming the address. An export already under way, the hourly
 * limit and a deployment without export storage each say why nothing was made.
 */
export function ExportDataDialog({
  open,
  email,
  onOpenChange,
  returnFocus,
}: Readonly<{
  open: boolean;
  email: string;
  onOpenChange: (open: boolean) => void;
  returnFocus?: RefObject<HTMLElement | null>;
}>) {
  const { t } = useTranslation();
  const request = useRequestDataExport();
  const [error, setError] = useState<string | null>(null);
  const addressId = useId();
  const fields: readonly FormField<Values>[] = [
    {
      kind: "toggle",
      name: "includeAnswers",
      text: t("systemSettings.export.includeAnswers"),
      sub: t("systemSettings.export.includeAnswersHint"),
    },
  ];

  async function submit(values: Values) {
    setError(null);
    try {
      await request.mutateAsync(values.includeAnswers);
      onOpenChange(false);
      toast(t("systemSettings.export.requested", { email }));
    } catch (cause) {
      setError(refusal(cause, t));
    }
  }

  return (
    <FormDialog
      open={open}
      onOpenChange={(next) => {
        if (!next) setError(null);
        onOpenChange(next);
      }}
      title={t("systemSettings.export.dialogTitle")}
      description={t("systemSettings.export.dialogHint")}
      initial={INITIAL}
      fields={fields}
      submitLabel={t("systemSettings.export.submit")}
      pending={request.isPending}
      error={error}
      returnFocus={returnFocus}
      onSubmit={(values) => void submit(values)}
    >
      <div className="mt-3.5 flex flex-col gap-1.5">
        <Label htmlFor={addressId} className="text-meta font-medium">
          {t("systemSettings.export.sendTo")}
        </Label>
        <Input id={addressId} readOnly value={email} />
      </div>
    </FormDialog>
  );
}
