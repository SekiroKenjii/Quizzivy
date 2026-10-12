import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import {
  CircleAlert,
  ImageOff,
  Info,
  TriangleAlert,
  type LucideIcon,
} from "lucide-react";
import { useLocale } from "@/lib/i18n/useLocale";
import { cn } from "@/lib/utils";
import type { PasteScan } from "../paste";
import type { PasteState } from "../usePastedText";

const SHOWN_ITEMS = 8;

type Tone = "danger" | "muted";

interface Note {
  id: string;
  icon: LucideIcon;
  tone: Tone;
  text: string;
}

interface Item {
  id: string;
  label: string;
  detail: string;
}

const NOTE_TONES: Record<Tone, string> = {
  danger: "bg-danger-soft [&>svg]:text-danger-ink",
  muted: "bg-muted [&>svg]:text-muted-fg",
};

function notesOf(scan: PasteScan, over: boolean, max: string, t: TFunction): Note[] {
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
  if (scan.questions > 0)
    notes.push({
      id: "media",
      icon: ImageOff,
      tone: "muted",
      text: t("imports.paste.mediaNote"),
    });
  return notes;
}

function itemsOf(scan: PasteScan, t: TFunction): Item[] {
  return [
    ...scan.missing.map((label) => ({
      id: `missing-${label}`,
      label: t("imports.workspace.questionLabel", { label }),
      detail: t("imports.workspace.noAnswer"),
    })),
    ...scan.duplicates.map((number) => ({
      id: `duplicate-${number}`,
      label: t("imports.workspace.questionLabel", { label: String(number) }),
      detail: t("imports.workspace.repeated"),
    })),
  ];
}

/**
 * PasteFindings is the "Read as" part of a pasted import's status pane: the
 * quick count of `scan` as "{n} questions recognised · {m} need checking",
 * then only what needs checking, each unanswered question and each number
 * used twice in a part, the first eight of them, and the notes on a text over
 * the limit, without numbered questions, or without its media. The server's
 * recognition stays the authority, in review.
 */
export function PasteFindings({
  scan,
  state,
  max,
}: Readonly<{ scan: PasteScan; state: PasteState; max: number }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  if (state === "empty")
    return (
      <p className="text-muted-fg px-4.5 py-3.5 text-sm">
        {t("imports.workspace.pasteEmpty")}
      </p>
    );
  const numbers = new Intl.NumberFormat(locale);
  const items = itemsOf(scan, t);
  const shown = items.slice(0, SHOWN_ITEMS);
  const rest = items.length - shown.length;
  return (
    <div className="flex flex-col">
      <div className="flex flex-col gap-0.5 border-b px-4.5 py-3">
        <p className="text-[14px] font-semibold">
          {t("imports.workspace.pasteLead", {
            count: scan.questions,
            questions: numbers.format(scan.questions),
            check: numbers.format(items.length),
          })}
        </p>
        <p className="text-muted-fg text-[12.5px]">
          {t("imports.workspace.pasteHint")}
        </p>
      </div>
      {shown.length === 0 ? null : (
        <ul aria-label={t("imports.workspace.needsCheckList")} className="divide-y">
          {shown.map((item) => (
            <li
              key={item.id}
              className="flex items-center gap-3 px-4.5 py-2.75 text-[13px]"
            >
              <span className="min-w-0 flex-1">
                <span className="block font-medium">{item.label}</span>
                <span className="text-muted-fg block text-xs">{item.detail}</span>
              </span>
              <span className="bg-warning-soft text-warning-ink inline-flex h-5.5 shrink-0 items-center gap-1.25 rounded-full px-2 text-xs font-medium">
                <TriangleAlert aria-hidden="true" className="size-3" />
                {t("imports.workspace.needsCheck")}
              </span>
            </li>
          ))}
          {rest > 0 ? (
            <li className="text-muted-fg px-4.5 py-2.75 text-[13px]">
              {t("imports.paste.more", { count: rest })}
            </li>
          ) : null}
        </ul>
      )}
      <div className="flex flex-col gap-2.5 border-t px-4.5 py-3">
        {notesOf(scan, state === "over", numbers.format(max), t).map((note) => (
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
        <p className="text-muted-fg text-xs leading-normal">
          {t("imports.paste.footnote")}
        </p>
      </div>
    </div>
  );
}
