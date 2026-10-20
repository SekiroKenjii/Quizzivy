import { useId, useRef, useState, type KeyboardEvent } from "react";
import { useTranslation } from "react-i18next";
import { ChevronDown, ChevronRight, ClipboardPaste, Eraser, Type } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { CopyButton } from "@/components/shared/CopyButton";
import { useLocale } from "@/lib/i18n/useLocale";
import { notify } from "@/lib/toast";
import { cn } from "@/lib/utils";
import { PASTE_EXAMPLE } from "../paste";

const TOOL_BUTTON =
  "h-8 gap-1.5 rounded-[8px] px-[11px] leading-[normal] [&_svg:not([class*='size-'])]:size-3.5";

function startOnShortcut(event: KeyboardEvent<HTMLTextAreaElement>) {
  if (event.key !== "Enter" || !(event.ctrlKey || event.metaKey)) return;
  if (event.nativeEvent.isComposing) return;
  event.preventDefault();
  event.currentTarget.form?.requestSubmit();
}

/**
 * PasteField is the "Test content" part of a pasted import's Sources pane:
 * the box with its character count against `max`, the clipboard and clear
 * actions, and how to lay the text out, which stays open while the box is
 * empty. What the quick count finds is PasteFindings, in the status pane.
 * `length` describes a deferred copy of `text`. Ctrl or ⌘+Enter in the box
 * submits its form.
 */
export function PasteField({
  text,
  length,
  max,
  busy,
  onTextChange,
}: Readonly<{
  text: string;
  length: number;
  max: number;
  busy: boolean;
  onTextChange: (next: string) => void;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const headingId = useId();
  const counterId = useId();
  const tipsId = useId();
  const box = useRef<HTMLTextAreaElement>(null);
  const [tipsWanted, setTipsWanted] = useState(false);
  const hasText = text !== "";
  const over = length > max;
  const tipsOpen = tipsWanted || !hasText;
  const numbers = new Intl.NumberFormat(locale);
  const maxLabel = numbers.format(max);

  async function fromClipboard() {
    let value: string;
    try {
      value = await navigator.clipboard.readText();
    } catch {
      notify.warning(t("imports.paste.clipboardBlocked"));
      return;
    }
    if (value === "") {
      notify.info(t("imports.paste.clipboardEmpty"));
      return;
    }
    onTextChange(value);
    box.current?.focus();
  }

  const tips = [
    [t("imports.paste.tip1Head"), t("imports.paste.tip1Body")],
    [t("imports.paste.tip2Head"), t("imports.paste.tip2Body")],
    [t("imports.paste.tip3Head"), t("imports.paste.tip3Body")],
    [t("imports.paste.tip4Head"), t("imports.paste.tip4Body")],
  ] as const;

  return (
    <section aria-labelledby={headingId} className="@container/paste flex flex-col">
      <div className="flex flex-wrap items-baseline justify-between gap-2.5 pb-1">
        <h3 id={headingId} className="text-ui font-semibold">
          {t("imports.paste.contentHeading")}
        </h3>
        <span
          id={counterId}
          className={cn(
            "text-[12.5px] tabular-nums",
            over ? "text-danger-ink" : "text-muted-fg",
          )}
        >
          {t("imports.paste.counter", { n: numbers.format(length), max: maxLabel })}
        </span>
      </div>
      <div className="flex flex-col gap-3 pt-2">
        <Textarea
          ref={box}
          aria-labelledby={headingId}
          aria-describedby={counterId}
          aria-invalid={over || undefined}
          spellCheck={false}
          value={text}
          readOnly={busy}
          placeholder={t("imports.paste.placeholder")}
          onChange={(event) => onTextChange(event.target.value)}
          onKeyDown={startOnShortcut}
          className={cn(
            "bg-bg border-border min-h-[340px] rounded-[10px] border-[1.5px] px-4 py-3.5 font-mono text-[13.5px] leading-[1.7] [tab-size:4] lg:text-[13.5px]",
            over && "border-danger focus-visible:border-danger",
          )}
        />
        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={busy}
            className={TOOL_BUTTON}
            onClick={() => void fromClipboard()}
          >
            <ClipboardPaste aria-hidden="true" />
            {t("imports.paste.fromClipboard")}
          </Button>
          {hasText ? (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              disabled={busy}
              className={cn(
                TOOL_BUTTON,
                "text-muted-fg hover:bg-hover hover:text-fg border border-transparent",
              )}
              onClick={() => {
                onTextChange("");
                box.current?.focus();
              }}
            >
              <Eraser aria-hidden="true" />
              {t("imports.paste.clear")}
            </Button>
          ) : null}
          {hasText ? (
            <span className="bg-muted text-muted-fg ml-auto inline-flex h-6.5 max-w-full min-w-0 items-center gap-1.5 rounded-full px-2.5 text-xs font-medium">
              <Type aria-hidden="true" className="size-3.25 shrink-0" />
              <span className="truncate">{t("imports.paste.kind")}</span>
            </span>
          ) : null}
        </div>
        <button
          type="button"
          aria-expanded={tipsOpen}
          aria-controls={tipsId}
          onClick={() => setTipsWanted(!tipsOpen)}
          className="inline-flex items-center gap-1.5 self-start rounded-sm text-[13px] leading-4 font-medium"
        >
          {tipsOpen ? (
            <ChevronDown aria-hidden="true" className="size-3.75" />
          ) : (
            <ChevronRight aria-hidden="true" className="size-3.75" />
          )}
          {t("imports.paste.tipsToggle")}
        </button>
        <div
          id={tipsId}
          hidden={!tipsOpen}
          className="bg-muted grid grid-cols-1 gap-3 rounded-[10px] p-3 @[594px]/paste:grid-cols-2"
        >
          <ol className="flex min-w-0 flex-col gap-2">
            {tips.map(([head, body], index) => (
              <li
                key={head}
                className="flex items-start gap-2.5 text-[13px] leading-normal"
              >
                <span
                  aria-hidden="true"
                  className="bg-primary text-primary-fg grid size-5.5 shrink-0 place-items-center rounded-full text-[11px] font-semibold"
                >
                  {index + 1}
                </span>
                <span>
                  <b className="font-semibold">{head}</b>{" "}
                  <span className="text-muted-fg">{body}</span>
                </span>
              </li>
            ))}
          </ol>
          <div className="flex min-w-0 flex-col items-start gap-1.5">
            <pre className="bg-card w-full rounded-[8px] border px-3 py-2.5 font-mono text-xs leading-[1.6] [overflow-wrap:anywhere] whitespace-pre-wrap">
              {PASTE_EXAMPLE}
            </pre>
            <CopyButton
              value={PASTE_EXAMPLE}
              label={t("imports.paste.copyExample")}
              failedMessage={t("imports.paste.copyFailed")}
              className="bg-card h-7 gap-1.5 rounded-[8px] border px-[11px] text-xs"
            />
          </div>
        </div>
      </div>
    </section>
  );
}
