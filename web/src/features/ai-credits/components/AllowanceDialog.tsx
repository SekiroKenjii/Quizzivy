import { useMemo, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { Sparkles } from "lucide-react";
import { FormDialog, type FormField } from "@/components/shared/form/FormDialog";
import type { AiAllowanceInput, AiTeacherMonth } from "@/features/ai-credits/api";
import {
  MAX_ALLOWANCE,
  MIN_ALLOWANCE,
  allowanceBody,
  baseAllowance,
  type AllowanceValues,
} from "@/features/ai-credits/allowance";
import { useSaveAllowance } from "@/features/ai-credits/mutations";
import { credits } from "@/features/ai/credits";
import { failureMessage } from "@/lib/api/errors";
import { useLocale } from "@/lib/i18n/useLocale";
import { notify } from "@/lib/toast";

/**
 * AllowanceDialog is the deck's "AI allowance" for one teacher, open while
 * `teacher` is set: the default allowance or a custom figure of at least
 * 1,000, and a switch that pauses their AI. It sends only what the Admin
 * changed, so a dialog closed without a change sends nothing, and a figure
 * below what the teacher has used is the server's refusal, shown in the
 * dialog.
 */
export function AllowanceDialog({
  teacher,
  defaultAllowance,
  onClose,
  returnFocus,
}: Readonly<{
  teacher: AiTeacherMonth | null;
  defaultAllowance: number;
  onClose: () => void;
  returnFocus?: RefObject<HTMLElement | null> | undefined;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const save = useSaveAllowance();
  const bounds = {
    min: credits(MIN_ALLOWANCE, locale),
    max: credits(MAX_ALLOWANCE, locale),
  };

  const fields = useMemo<readonly FormField<AllowanceValues>[]>(
    () => [
      {
        kind: "seg",
        name: "mode",
        label: t("aiCredits.allowance.mode"),
        options: [
          {
            value: "default",
            label: t("aiCredits.allowance.default", {
              n: credits(defaultAllowance, locale),
            }),
          },
          { value: "custom", label: t("aiCredits.allowance.custom") },
        ],
      },
      {
        kind: "number",
        name: "credits",
        label: t("aiCredits.allowance.credits"),
        hint: t("aiCredits.allowance.creditsHint"),
        inputMode: "numeric",
        autoComplete: "off",
        when: (values) => values.mode === "custom",
      },
      {
        kind: "toggle",
        name: "paused",
        text: t("aiCredits.allowance.paused"),
        sub: t("aiCredits.allowance.pausedHint"),
      },
    ],
    [t, locale, defaultAllowance],
  );

  const initial = useMemo<AllowanceValues>(
    () => ({
      mode: teacher?.custom === true ? "custom" : "default",
      credits: String(
        teacher?.custom === true ? baseAllowance(teacher) : defaultAllowance,
      ),
      paused: teacher?.paused ?? false,
    }),
    [teacher, defaultAllowance],
  );

  function validate(values: AllowanceValues) {
    if (values.mode !== "custom") return null;
    const figure = Number(values.credits.trim());
    if (Number.isNaN(figure) || figure < MIN_ALLOWANCE)
      return { credits: t("aiCredits.allowance.min", bounds) };
    if (!Number.isInteger(figure)) return { credits: t("aiCredits.allowance.whole") };
    if (figure > MAX_ALLOWANCE)
      return { credits: t("aiCredits.allowance.max", bounds) };
    return null;
  }

  function submit(values: AllowanceValues) {
    if (teacher === null || save.isPending) return;
    const body: AiAllowanceInput | null = allowanceBody(teacher, values);
    if (body === null) {
      onClose();
      return;
    }
    save.mutate(
      { userId: teacher.userId, body },
      {
        onSuccess: () => {
          onClose();
          notify.success(t("aiCredits.allowance.saved", { name: teacher.name }));
        },
      },
    );
  }

  return (
    <FormDialog
      open={teacher !== null}
      onOpenChange={(next) => {
        if (next) return;
        save.reset();
        onClose();
      }}
      title={t("aiCredits.allowance.title", { name: teacher?.name ?? "" })}
      description={t("aiCredits.allowance.used", {
        used: credits(teacher?.used ?? 0, locale),
      })}
      icon={Sparkles}
      initial={initial}
      fields={fields}
      validate={validate}
      submitLabel={t("common.save")}
      pending={save.isPending}
      error={
        save.isError
          ? failureMessage(save.error, t("aiCredits.allowance.failed"))
          : null
      }
      returnFocus={returnFocus}
      onSubmit={submit}
    />
  );
}
