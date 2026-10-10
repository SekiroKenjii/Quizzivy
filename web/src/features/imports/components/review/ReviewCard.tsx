import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import {
  ChevronDown,
  ChevronUp,
  CircleAlert,
  CircleCheck,
  CircleHelp,
  Info,
  type LucideIcon,
} from "lucide-react";
import { cn } from "@/lib/utils";
import type { ImportFinding } from "../../api";
import { isUnresolved } from "../../findings";

type PillTone = "blocking" | "review" | "info";

const PILL: Record<PillTone, { icon: LucideIcon; tone: string; label: string }> = {
  blocking: {
    icon: CircleAlert,
    tone: "bg-danger-soft text-danger-ink",
    label: "imports.severity.blocking",
  },
  review: {
    icon: CircleHelp,
    tone: "bg-warning-soft text-warning-ink",
    label: "imports.severity.review",
  },
  info: {
    icon: Info,
    tone: "bg-info-soft text-info-ink",
    label: "imports.severity.info",
  },
};

/**
 * OpenPill is a card's summary of its open findings: the most urgent
 * severity, or "{n} open" when there are several, and nothing when none is
 * open.
 */
export function OpenPill({
  findings,
}: Readonly<{ findings: readonly ImportFinding[] }>) {
  const { t } = useTranslation();
  const open = findings.filter(isUnresolved);
  if (open.length === 0) return null;
  const tone: PillTone = open.some((finding) => finding.severity === "blocking")
    ? "blocking"
    : "review";
  const { icon: Icon, tone: colour, label } = PILL[tone];
  return (
    <span
      className={cn(
        "text-caption inline-flex h-5.5 flex-none items-center gap-1.25 rounded-full px-2 font-medium whitespace-nowrap",
        colour,
      )}
    >
      <Icon aria-hidden="true" className="size-3 flex-none" />
      {open.length > 1
        ? t("imports.review.openCount", { count: open.length })
        : t(label)}
    </span>
  );
}

/**
 * ReviewCard is one card of the review's test pane, as the deck draws it: a
 * header button that opens and closes it (`aria-expanded`) and, while open,
 * its body under a rule. `headerId` is the header button's
 * `data-question-id`, the anchor the workspace scrolls and focuses. A card
 * with nothing open shows a check; `dimmed` fades a closed excluded card.
 */
export function ReviewCard({
  cardId,
  headerId,
  open,
  dimmed = false,
  done,
  badge,
  headline,
  strike = false,
  meta,
  findings,
  onToggle,
  children,
}: Readonly<{
  cardId: string;
  headerId?: string;
  open: boolean;
  dimmed?: boolean;
  done: boolean;
  badge: ReactNode;
  headline: string;
  strike?: boolean;
  meta?: string;
  findings: readonly ImportFinding[];
  onToggle: () => void;
  children?: ReactNode;
}>) {
  const { t } = useTranslation();
  const Chevron = open ? ChevronUp : ChevronDown;
  return (
    <div
      data-card={cardId}
      className={cn(
        "bg-card rounded-xl border transition-colors duration-150",
        open ? "border-ring shadow-lg" : "shadow-card",
        dimmed && !open && "opacity-60",
      )}
    >
      <button
        type="button"
        data-question-id={headerId}
        aria-expanded={open}
        onClick={onToggle}
        className="flex w-full cursor-pointer items-center gap-2.5 rounded-xl px-3 py-2.5 text-left"
      >
        <span className="bg-muted grid h-6.5 min-w-7 flex-none place-items-center rounded-[7px] px-1.5 text-xs font-semibold tabular-nums">
          {badge}
        </span>
        <span
          className={cn("text-ui min-w-12 flex-1 truncate", strike && "line-through")}
        >
          {headline}
        </span>
        {meta ? (
          <span className="text-muted-fg hidden flex-none text-xs whitespace-nowrap min-[480px]:inline">
            {meta}
          </span>
        ) : null}
        <OpenPill findings={findings} />
        {done ? (
          <CircleCheck
            role="img"
            aria-label={t("imports.review.nothingOpenHere")}
            className="text-success size-3.75 flex-none"
          />
        ) : null}
        <Chevron aria-hidden="true" className="text-muted-fg size-3.75 flex-none" />
      </button>
      {open ? children : null}
    </div>
  );
}
