import { useMemo, type RefObject } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Sparkles } from "lucide-react";
import { FormDialog, type FormField } from "@/components/shared/form/FormDialog";
import {
  adminCreditsQuery,
  type AiAllowanceInput,
  type AiTeacherMonth,
} from "@/features/ai-credits/api";
import {
  MAX_ALLOWANCE,
  MIN_ALLOWANCE,
  allowanceBody,
  baseAllowance,
  type AllowanceValues,
} from "@/features/ai-credits/allowance";
import { useLatch, useSaveAllowance } from "@/features/ai-credits/mutations";
import { credits } from "@/features/ai/credits";
import { failureMessage } from "@/lib/api/errors";
import { useLocale } from "@/lib/i18n/useLocale";
import { notify } from "@/lib/toast";

/**
 * AllowanceDialog is the deck's "AI allowance" for one teacher, open while
 * `teacher` is set: the default allowance or a custom figure of at least
 * 1,000, and a switch that pauses their AI. It sends only what the Admin
 * changed; a save that changes nothing against the row it opened on reads
 * the month again and sends what differs from the teacher's row there, so a
 * change another Admin made meanwhile is not kept by mistake. A figure
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
  const latch = useLatch();
  const client = useQueryClient();
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

  function send(target: AiTeacherMonth, body: AiAllowanceInput | null) {
    if (body === null) {
      latch.release();
      onClose();
      return;
    }
    save.mutate(
      { userId: target.userId, body },
      {
        onSuccess: () => {
          onClose();
          notify.success(t("aiCredits.allowance.saved", { name: target.name }));
        },
        onSettled: latch.release,
      },
    );
  }

  function submit(values: AllowanceValues) {
    if (teacher === null || !latch.take()) return;
    const body = allowanceBody(teacher, values);
    if (body !== null) {
      send(teacher, body);
      return;
    }
    client
      .query({ ...adminCreditsQuery(), staleTime: 0 })
      .then((view) => view.teachers.find((row) => row.userId === teacher.userId))
      .catch(() => undefined)
      .then((fresh) => send(teacher, fresh ? allowanceBody(fresh, values) : null));
  }

  return (
    <FormDialog
      open={teacher !== null}
      onOpenChange={(next) => {
        if (next) return;
        save.reset();
        latch.release();
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
