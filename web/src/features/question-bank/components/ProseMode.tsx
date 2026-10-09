import {
  useEffect,
  useId,
  useRef,
  useState,
  type KeyboardEvent,
  type ReactNode,
} from "react";
import { useTranslation } from "react-i18next";
import { FileCode, Info, Type } from "lucide-react";
import { Segmented } from "@/components/ui/segmented";
import { ContentView } from "@/components/shared/content/ContentView";
import type { QuestionPromptContent } from "@/components/shared/content/questionContent";
import type { ProseMode } from "../proseMode";

/**
 * ProseModeHeader is the field's header: its label, an optional hint, and
 * "Rich text | Markdown" on the right. `markdownBlocked` disables "Markdown"
 * and says why under the header.
 */
export function ProseModeHeader({
  id,
  label,
  hint,
  mode,
  onMode,
  markdownBlocked,
}: Readonly<{
  id: string;
  label: string;
  hint?: string | undefined;
  mode: ProseMode;
  onMode: (mode: ProseMode) => void;
  markdownBlocked?: string | undefined;
}>) {
  const { t } = useTranslation();
  const reason = `${id}-markdown-blocked`;
  return (
    <div className="mb-1.5 flex flex-col gap-1">
      <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-2">
        <label className="text-[12.5px] font-medium" htmlFor={id}>
          {label}
          {hint && (
            <>
              {" "}
              <span className="text-muted-fg font-normal">{hint}</span>
            </>
          )}
        </label>
        <Segmented
          size="xs"
          label={t("proseMode.label", { label })}
          value={mode}
          onChange={(value) => onMode(value as ProseMode)}
          options={[
            { value: "rich", label: t("proseMode.rich"), icon: Type },
            {
              value: "markdown",
              label: t("proseMode.markdown"),
              icon: FileCode,
              disabled: markdownBlocked !== undefined && mode === "rich",
              describedBy: markdownBlocked === undefined ? undefined : reason,
            },
          ]}
        />
      </div>
      {markdownBlocked !== undefined && mode === "rich" && (
        <p id={reason} className="text-muted-fg text-xs">
          {markdownBlocked}
        </p>
      )}
    </div>
  );
}

const ACTION = "h-8 rounded-[8px] px-3 text-[13px] font-medium whitespace-nowrap";
const SECONDARY = `${ACTION} bg-card hover:bg-hover border`;
const PRIMARY = `${ACTION} bg-primary text-primary-fg font-semibold hover:opacity-90`;

/**
 * SwitchToMarkdown asks, in place under the toolbar, before a rich field is
 * written as Markdown. Escape and "Cancel" call `onCancel`; it takes focus
 * when it opens.
 */
export function SwitchToMarkdown({
  onCancel,
  onConfirm,
}: Readonly<{ onCancel: () => void; onConfirm: () => void }>) {
  const { t } = useTranslation();
  const message = useId();
  const cancel = useRef<HTMLButtonElement>(null);
  useEffect(() => cancel.current?.focus(), []);
  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key !== "Escape") return;
    event.stopPropagation();
    onCancel();
  };
  return (
    <div
      role="alertdialog"
      aria-labelledby={message}
      aria-modal="false"
      className="bg-warning-soft flex flex-col gap-2.5 border-b px-3.5 py-3"
    >
      <span id={message} className="text-fg text-[13px] leading-[1.55] text-pretty">
        {t("proseMode.switchWarning")}
      </span>
      <div className="flex flex-wrap justify-end gap-2">
        <button
          ref={cancel}
          type="button"
          className={SECONDARY}
          onClick={onCancel}
          onKeyDown={onKeyDown}
        >
          {t("common.cancel")}
        </button>
        <button
          type="button"
          className={PRIMARY}
          onClick={onConfirm}
          onKeyDown={onKeyDown}
        >
          {t("proseMode.switchToMarkdown")}
        </button>
      </div>
    </div>
  );
}

/** Conversion is a Markdown field converted to rich content, with whatever else the conversion bound. */
export type Conversion = { content: QuestionPromptContent };

/**
 * ConversionPanel shows a Markdown field converted to rich content in place
 * of its body, with "Keep Markdown" and "Apply conversion". A text the
 * converter refuses shows `blocked` and offers "Keep Markdown" only.
 */
export function ConversionPanel<T extends Conversion>({
  convert,
  blocked,
  onKeep,
  onApply,
}: Readonly<{
  convert: () => T | null;
  blocked: string;
  onKeep: () => void;
  onApply: (conversion: T) => void;
}>) {
  const { t } = useTranslation();
  const [conversion] = useState(convert);
  const keep = useRef<HTMLButtonElement>(null);
  useEffect(() => keep.current?.focus(), []);
  let body: ReactNode = (
    <div
      role="alert"
      className="bg-warning-soft text-fg rounded-[8px] px-3 py-2.5 text-[13px] leading-normal"
    >
      {blocked}
    </div>
  );
  if (conversion)
    body = (
      <>
        <div className="bg-info-soft flex items-start gap-2 rounded-[8px] px-3 py-2.5 text-[13px] leading-normal">
          <Info
            size={15}
            aria-hidden="true"
            className="text-info-ink mt-0.5 flex-none"
          />
          <span>{t("questionEditor.proseConversionPreview")}</span>
        </div>
        <ContentView
          document={conversion.content}
          className="max-h-80 overflow-auto rounded-[8px] border px-3.5 py-3 [font-size:var(--content-editor-font-size,inherit)] leading-[1.6]"
        />
      </>
    );
  return (
    <div className="flex flex-col gap-2.5 px-3.5 py-3">
      {body}
      <div className="flex flex-wrap justify-end gap-2">
        <button ref={keep} type="button" className={SECONDARY} onClick={onKeep}>
          {t("questionEditor.keepMarkdown")}
        </button>
        {conversion && (
          <button type="button" className={PRIMARY} onClick={() => onApply(conversion)}>
            {t("questionEditor.applyProseConversion")}
          </button>
        )}
      </div>
    </div>
  );
}
