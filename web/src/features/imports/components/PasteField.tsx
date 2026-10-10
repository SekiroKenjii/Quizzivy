import { useId, useRef, useState, type KeyboardEvent } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import {
  ChevronDown,
  ChevronRight,
  CircleAlert,
  ClipboardPaste,
  Copy,
  Eraser,
  ImageOff,
  Info,
  KeyRound,
  Type,
  type LucideIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { CopyButton } from "@/components/shared/CopyButton";
import { useLocale } from "@/lib/i18n/useLocale";
import { notify } from "@/lib/toast";
import { cn } from "@/lib/utils";
import { PASTE_EXAMPLE, type PasteScan } from "../paste";

const SHOWN_NUMBERS = 6;
const TOOL_BUTTON =
  "h-8 gap-1.5 rounded-[8px] px-[11px] text-[13px] in-data-[scale=deck]:h-8 in-data-[scale=deck]:rounded-[8px] [&_svg:not([class*='size-'])]:size-3.5";

type Tone = "danger" | "warning" | "muted";

interface Note {
  id: string;
  icon: LucideIcon;
  tone: Tone;
  text: string;
}

const NOTE_TONES: Record<Tone, string> = {
  danger: "bg-danger-soft [&>svg]:text-danger-ink",
  warning: "bg-warning-soft [&>svg]:text-warning-ink",
  muted: "bg-muted [&>svg]:text-muted-fg",
};

function numberList(items: readonly string[], t: TFunction): string {
  const shown = items.slice(0, SHOWN_NUMBERS);
  const rest = items.length - shown.length;
  const parts = rest > 0 ? [...shown, t("imports.paste.more", { count: rest })] : shown;
  if (parts.length === 1) return parts[0]!;
  return t("imports.paste.listAnd", {
    head: parts.slice(0, -1).join(", "),
    last: parts[parts.length - 1],
  });
}

function scanNotes(scan: PasteScan, over: boolean, max: string, t: TFunction): Note[] {
  const notes: Note[] = [];
  if (over)
    notes.push({
      id: "over",
      icon: CircleAlert,
      tone: "danger",
      text: t("imports.paste.overNote", { max }),
    });
  if (scan.questions === 0)
    notes.push({
      id: "none",
      icon: Info,
      tone: "muted",
      text: t("imports.paste.noQuestionsNote"),
    });
  if (scan.missing.length > 0)
    notes.push({
      id: "missing",
      icon: KeyRound,
      tone: "warning",
      text: t("imports.paste.missingNote", {
        count: scan.missing.length,
        list: numberList(scan.missing, t),
      }),
    });
  if (scan.duplicates.length > 0)
    notes.push({
      id: "duplicates",
      icon: Copy,
      tone: "warning",
      text: t("imports.paste.duplicateNote", {
        count: scan.duplicates.length,
        list: numberList(scan.duplicates.map(String), t),
      }),
    });
  if (scan.questions > 0)
    notes.push({
      id: "media",
      icon: ImageOff,
      tone: "muted",
      text: t("imports.paste.mediaNote"),
    });
  return notes;
}

function startOnShortcut(event: KeyboardEvent<HTMLTextAreaElement>) {
  if (event.key !== "Enter" || !(event.ctrlKey || event.metaKey)) return;
  if (event.nativeEvent.isComposing) return;
  event.preventDefault();
  event.currentTarget.form?.requestSubmit();
}

/**
 * PasteField is the "Test content" card of a pasted import: the box with its
 * character count against `max`, the clipboard and clear actions, what the
 * quick count in `scan` found, and how to lay the text out, which stays open
 * while the box is empty. `length` and `scan` describe a deferred copy of
 * `text`. Ctrl or ⌘+Enter in the box submits its form.
 */
export function PasteField({
  text,
  length,
  max,
  scan,
  busy,
  onTextChange,
}: Readonly<{
  text: string;
  length: number;
  max: number;
  scan: PasteScan;
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
  const stats = [
    { id: "sections", label: t("imports.paste.sections"), value: scan.sections },
    { id: "questions", label: t("imports.paste.questions"), value: scan.questions },
    { id: "answered", label: t("imports.paste.answered"), value: scan.answered },
    {
      id: "missing",
      label: t("imports.paste.needAnswer"),
      value: scan.missing.length,
      warn: scan.missing.length > 0,
    },
  ];

  return (
    <section
      aria-labelledby={headingId}
      className="bg-card text-card-foreground shadow-card @container/paste rounded-xl border"
    >
      <div className="flex flex-wrap items-baseline justify-between gap-2.5 px-4.5 pt-4 pb-1">
        <h2 id={headingId} className="text-[15px] font-semibold">
          {t("imports.paste.contentHeading")}
        </h2>
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
      <div className="flex flex-col gap-3 px-4.5 pt-2.5 pb-4.5">
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
            "bg-bg border-border min-h-[340px] rounded-[10px] border-[1.5px] px-4 py-3.5 font-mono text-[13.5px] leading-[1.7] [tab-size:4] in-data-[scale=deck]:px-4 lg:text-[13.5px] in-data-[scale=deck]:lg:text-[13.5px]",
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
              className={cn(TOOL_BUTTON, "text-muted-fg hover:bg-hover hover:text-fg")}
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
        {hasText ? (
          <div className="border-border flex flex-col gap-2.5 border-t pt-3">
            <h3 className="text-[13px] font-semibold">{t("imports.paste.found")}</h3>
            <dl className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,140px),1fr))] gap-2">
              {stats.map((stat) => (
                <div
                  key={stat.id}
                  className={cn(
                    "rounded-[10px] border px-3 py-2.5",
                    stat.warn ? "bg-warning-soft text-warning-ink" : "bg-card",
                  )}
                >
                  <dt className="text-muted-fg text-xs">{stat.label}</dt>
                  <dd className="text-xl font-semibold tabular-nums">
                    {numbers.format(stat.value)}
                  </dd>
                </div>
              ))}
            </dl>
            {scanNotes(scan, over, maxLabel, t).map((note) => (
              <p
                key={note.id}
                className={cn(
                  "flex items-start gap-2.5 rounded-[9px] px-3 py-2.5 text-[13px] leading-normal",
                  NOTE_TONES[note.tone],
                )}
              >
                <note.icon aria-hidden="true" className="mt-0.5 size-3.75 shrink-0" />
                <span className="min-w-0 flex-1">{note.text}</span>
              </p>
            ))}
            <p className="text-muted-fg text-xs">{t("imports.paste.footnote")}</p>
          </div>
        ) : null}
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
            <pre className="bg-card w-full rounded-lg border px-3 py-2.5 font-mono text-xs leading-[1.6] [overflow-wrap:anywhere] whitespace-pre-wrap">
              {PASTE_EXAMPLE}
            </pre>
            <CopyButton
              value={PASTE_EXAMPLE}
              label={t("imports.paste.copyExample")}
              failedMessage={t("imports.paste.copyFailed")}
              className="bg-card h-7 border"
            />
          </div>
        </div>
      </div>
    </section>
  );
}
