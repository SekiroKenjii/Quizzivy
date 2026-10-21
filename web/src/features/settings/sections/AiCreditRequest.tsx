import { useEffect, useRef, useState, type ReactNode } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { Hourglass } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "@/components/ui/sonner";
import { Textarea } from "@/components/ui/textarea";
import type { AiCreditRequest as Request, AiMonthUsage } from "@/features/ai/api";
import { credits } from "@/features/ai/credits";
import { useRequestAiCredits } from "@/features/settings/aiApi";
import {
  AI_REASON_MAX,
  AI_REQUEST_MAX,
  AI_REQUEST_MIN,
  aiCreditRequestSchema,
  type AiCreditRequestOutput,
  type AiCreditRequestValues,
} from "@/features/settings/aiCreditRequestSchema";
import { useUnsavedSection } from "@/features/settings/unsavedSection";
import { failureMessage } from "@/lib/api/errors";
import { dayMonthYear } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";

const GRID =
  "grid grid-cols-[repeat(auto-fit,minmax(max(min(260px,100%),calc((100%_-_2.5rem)/3)),1fr))] gap-x-5 gap-y-3.5";
const EMPTY: AiCreditRequestValues = { amount: "", reason: "" };

function Field({
  id,
  label,
  hint,
  error,
  children,
}: Readonly<{
  id: string;
  label: string;
  hint: string;
  error: string | undefined;
  children: ReactNode;
}>) {
  return (
    <div className="flex max-w-(--field-max) min-w-0 flex-col gap-1.5">
      <Label htmlFor={id} className="text-meta leading-normal font-medium">
        {label}
      </Label>
      {children}
      {error === undefined ? (
        <p id={`${id}-hint`} className="text-muted-fg text-xs leading-normal">
          {hint}
        </p>
      ) : (
        <p id={`${id}-error`} className="text-danger-ink text-xs leading-normal">
          {error}
        </p>
      )}
    </div>
  );
}

function Pending({ request, focus }: Readonly<{ request: Request; focus: boolean }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const note = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (focus) note.current?.focus();
  }, [focus]);
  return (
    <div
      ref={note}
      tabIndex={-1}
      data-testid="ai-request-pending"
      className="flex items-start gap-3 border-t px-4.5 py-3.5 -outline-offset-2"
    >
      <Hourglass
        aria-hidden="true"
        className="text-muted-fg mt-0.5 size-[15px] flex-none"
      />
      <div className="min-w-0 flex-1">
        <p className="text-ui leading-normal font-medium">
          {t("settings.ai.request.pending", {
            amount: credits(request.amount, locale),
          })}
        </p>
        <p className="text-muted-fg text-meta leading-normal">
          {t("settings.ai.request.pendingMeta", {
            date: dayMonthYear(request.createdAt, locale),
          })}
        </p>
        <p className="text-muted-fg text-meta mt-1 leading-normal break-words whitespace-pre-line">
          {request.reason}
        </p>
      </div>
    </div>
  );
}

function RequestForm({
  onClose,
  onSent,
}: Readonly<{ onClose: () => void; onSent: () => void }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const send = useRequestAiCredits();
  const sending = useRef(false);
  const [error, setError] = useState<string | null>(null);
  const form = useForm<AiCreditRequestValues, unknown, AiCreditRequestOutput>({
    resolver: zodResolver(aiCreditRequestSchema),
    defaultValues: EMPTY,
    mode: "onTouched",
  });
  const { errors, isDirty } = form.formState;
  useUnsavedSection("ai", isDirty);
  const { setFocus } = form;
  useEffect(() => {
    setFocus("amount");
  }, [setFocus]);

  const bounds = {
    min: credits(AI_REQUEST_MIN, locale),
    max: credits(AI_REQUEST_MAX, locale),
  };
  const amountError = errors.amount?.message;
  const reasonError = errors.reason?.message;

  function submit(values: AiCreditRequestOutput) {
    if (sending.current) return;
    sending.current = true;
    setError(null);
    send.mutate(values, {
      onSuccess: () => {
        toast(t("settings.ai.request.sent"));
        onSent();
      },
      onError: (cause) => setError(failureMessage(cause, t("api.failed"))),
      onSettled: () => {
        sending.current = false;
      },
    });
  }

  return (
    <form
      noValidate
      onSubmit={(event) => void form.handleSubmit(submit)(event)}
      aria-labelledby="ai-request-title"
      className="flex flex-col gap-3.5 border-t px-4.5 py-4"
    >
      <div>
        <h3 id="ai-request-title" className="text-ui leading-normal font-semibold">
          {t("settings.ai.request.open")}
        </h3>
        <p className="text-muted-fg text-meta leading-normal">
          {t("settings.ai.request.hint")}
        </p>
      </div>
      <div className={GRID}>
        <Field
          id="ai-request-amount"
          label={t("settings.ai.request.amount")}
          hint={t("settings.ai.request.amountHint", bounds)}
          error={amountError === undefined ? undefined : t(amountError, bounds)}
        >
          <Input
            id="ai-request-amount"
            className="bg-bg h-9.5"
            inputMode="numeric"
            autoComplete="off"
            aria-invalid={errors.amount ? true : undefined}
            aria-describedby={
              errors.amount ? "ai-request-amount-error" : "ai-request-amount-hint"
            }
            {...form.register("amount")}
          />
        </Field>
        <Field
          id="ai-request-reason"
          label={t("settings.ai.request.reason")}
          hint={t("settings.ai.request.reasonHint", { max: AI_REASON_MAX })}
          error={
            reasonError === undefined
              ? undefined
              : t(reasonError, { max: AI_REASON_MAX })
          }
        >
          <Textarea
            id="ai-request-reason"
            rows={3}
            maxLength={AI_REASON_MAX}
            className="bg-bg min-h-19"
            aria-invalid={errors.reason ? true : undefined}
            aria-describedby={
              errors.reason ? "ai-request-reason-error" : "ai-request-reason-hint"
            }
            {...form.register("reason")}
          />
        </Field>
      </div>
      {error === null ? null : (
        <p role="alert" className="text-danger-ink text-meta leading-normal">
          {error}
        </p>
      )}
      <div className="flex flex-wrap justify-end gap-2">
        <Button type="button" variant="outline" onClick={onClose}>
          {t("common.cancel")}
        </Button>
        <Button type="submit" aria-disabled={send.isPending || undefined}>
          {send.isPending
            ? t("settings.ai.request.sending")
            : t("settings.ai.request.send")}
        </Button>
      </div>
    </form>
  );
}

/**
 * AiCreditRequest is "Ask for more credits" under the usage card (DG-200):
 * while the centre takes requests and none is waiting, a button that opens
 * a form of the amount (1,000 to 100,000) and a reason, sent once however
 * often it is pressed; then the waiting request, until an admin decides it.
 * Nothing is drawn while the centre's rule is to pause a teacher who runs
 * out.
 */
export function AiCreditRequest({ usage }: Readonly<{ usage: AiMonthUsage }>) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [sent, setSent] = useState(false);
  const opener = useRef<HTMLButtonElement>(null);
  const refocus = useRef(false);
  useEffect(() => {
    if (open || !refocus.current) return;
    refocus.current = false;
    opener.current?.focus();
  }, [open]);
  const close = () => {
    refocus.current = true;
    setOpen(false);
  };

  if (usage.pending !== null) return <Pending request={usage.pending} focus={sent} />;
  if (!usage.requests) return null;
  if (open)
    return (
      <RequestForm
        onClose={close}
        onSent={() => {
          setSent(true);
          setOpen(false);
        }}
      />
    );
  return (
    <div className="flex flex-wrap items-center justify-between gap-2.5 border-t px-4.5 py-3.5">
      <span className="text-muted-fg text-sm leading-normal">
        {t("settings.ai.request.prompt")}
      </span>
      <Button
        ref={opener}
        type="button"
        variant="outline"
        onClick={() => setOpen(true)}
      >
        {t("settings.ai.request.open")}
      </Button>
    </div>
  );
}
