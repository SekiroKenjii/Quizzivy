import { useTranslation } from "react-i18next";
import { Check, CircleAlert, CloudCheck, LoaderCircle } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { AutosaveStatus } from "@/features/tests/useAutosave";
import { formatTime, useDisplayTimeZone } from "@/lib/i18n/datetime";

const DECK_STATES = {
  idle: { label: "builder.savedPlain", Icon: CloudCheck },
  saved: { label: "builder.savedPlain", Icon: CloudCheck },
  saving: { label: "builder.saving", Icon: LoaderCircle },
  dirty: { label: "builder.dirty", Icon: null },
} as const;
type DeckState = keyof typeof DECK_STATES;

function isDeckState(kind: AutosaveStatus["kind"]): kind is DeckState {
  return kind in DECK_STATES;
}

function DeckStatusLabel({ kind }: Readonly<{ kind: DeckState }>) {
  const { t } = useTranslation();
  const { label, Icon } = DECK_STATES[kind];
  return (
    <span
      role="status"
      aria-live="polite"
      data-state={kind}
      className="text-muted-fg flex shrink-0 items-center gap-1.25 text-[12.5px] whitespace-nowrap"
    >
      {Icon ? <Icon aria-hidden="true" className="size-3.5" /> : null}
      {t(label)}
    </span>
  );
}

/**
 * AutosaveStatusLabel reports pending, acknowledged, stale and failed saves in
 * words. In its `deck` form, the builder's, it says "Saved" from load and
 * after each save without a time, "Saving…" while a write is out, and
 * "Not saved" while an edit waits; `data-state` carries the status kind.
 */
export function AutosaveStatusLabel({
  status,
  onRetry,
  staleLabel,
  deck = false,
}: Readonly<{
  status: AutosaveStatus;
  onRetry?: () => void;
  staleLabel?: string;
  deck?: boolean;
}>) {
  useDisplayTimeZone();
  const { t } = useTranslation();

  if (deck && isDeckState(status.kind)) return <DeckStatusLabel kind={status.kind} />;

  if (status.kind === "idle") return null;

  if (status.kind === "dirty") {
    return (
      <span role="status" aria-live="polite" className="text-muted-foreground text-xs">
        {t("builder.dirty")}
      </span>
    );
  }

  if (status.kind === "saving") {
    return (
      <span role="status" aria-live="polite" className="text-muted-foreground text-xs">
        {t("builder.saving")}
      </span>
    );
  }

  if (status.kind === "saved") {
    return (
      <Badge
        role="status"
        aria-live="polite"
        className={
          deck
            ? "text-muted-foreground border-transparent bg-transparent px-0 font-normal"
            : undefined
        }
      >
        <Check aria-hidden="true" />
        {t("builder.saved", { time: formatTime(status.at) })}
      </Badge>
    );
  }

  if (status.kind === "stale") {
    return (
      <Badge variant="danger" role="alert">
        <CircleAlert aria-hidden="true" />
        {staleLabel ?? t("builder.stale")}
      </Badge>
    );
  }
  return (
    <span role="alert" className="flex items-center gap-2">
      <Badge variant="danger">
        <CircleAlert aria-hidden="true" />
        {t("builder.saveFailed")}
      </Badge>
      {status.message === "" ? null : (
        <span className="text-muted-foreground text-xs">{status.message}</span>
      )}
      {onRetry === undefined ? null : (
        <Button variant="outline" size="xs" onClick={onRetry}>
          {t("common.retry")}
        </Button>
      )}
    </span>
  );
}
