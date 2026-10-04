import { useTranslation } from "react-i18next";
import { Lock } from "lucide-react";

/**
 * LockNoticeProps is what a LockNotice takes: the sentence that says why the
 * thing is locked, and the word shown in place of "Locked".
 */
export type LockNoticeProps = Readonly<{
  reason: string;
  label?: string | undefined;
}>;

/**
 * LockNotice is the deck's marker on a card whose fields cannot be changed
 * for now: a small lock and the word "Locked", muted, on one line. The
 * reason is its `title`, for a pointer, and a visually hidden sentence after
 * the word, for a keyboard and a screen reader, which never reach a `title`.
 * It is text, not a control: it takes no tab stop.
 */
export function LockNotice({ reason, label }: LockNoticeProps) {
  const { t } = useTranslation();
  return (
    <span
      title={reason}
      className="text-muted-fg relative inline-flex items-center gap-1.25 text-xs leading-normal whitespace-nowrap"
    >
      <Lock aria-hidden="true" className="size-[0.8125rem] shrink-0" />
      {label ?? t("display.locked")} <span className="sr-only">{reason}</span>
    </span>
  );
}
