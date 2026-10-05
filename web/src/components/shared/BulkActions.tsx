import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { X, type LucideIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useDeckScale } from "@/components/ui/deck-scale";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { ApiError } from "@/lib/api/errors";
import { cn } from "@/lib/utils";
import { toast } from "@/components/ui/sonner";

/**
 * BulkAction is one thing a bulk bar does to every selected item, one at a
 * time, after a confirmation that lists them: `label` names the button and
 * the dialog, `description` is the dialog's sentence, `run` acts on one item.
 * `icon` is drawn before the label on a deck surface. The confirmation is a
 * destructive one unless `destructive` is false.
 */
export interface BulkAction<T> {
  label: string;
  description: string;
  run: (item: T) => Promise<unknown>;
  icon?: LucideIcon | undefined;
  destructive?: boolean | undefined;
}

const DECK_BAR =
  "bg-primary text-primary-fg flex flex-wrap items-center gap-2 rounded-lg py-2 pr-2.5 pl-3.5 [--focus:var(--primary-fg)]";

const DECK_COUNT = "mr-auto text-sm font-medium whitespace-nowrap";

const DECK_BUTTON =
  "border-muted-fg/35 hover:bg-primary-fg/10 rounded-seg text-meta inline-flex h-7.5 items-center gap-1.5 border bg-transparent px-2.5 leading-4 font-medium whitespace-nowrap disabled:pointer-events-none disabled:opacity-45";

const DECK_CLEAR =
  "hover:bg-primary-fg/10 rounded-seg grid size-7.5 flex-none place-items-center border-0 bg-transparent disabled:pointer-events-none disabled:opacity-45";

/**
 * BulkBarButton is a button of the bulk bar, for a control a page puts in the
 * bar as `children`, such as an action that opens its own form; the bar's own
 * actions render through it. On a deck surface it is the deck's 30px button
 * on the inverted bar, with `icon` before the label. Elsewhere it is the
 * outline button the bar has always drawn, and `icon` is not drawn.
 */
export function BulkBarButton({
  icon: Icon,
  children,
  onClick,
  disabled,
}: Readonly<{
  icon?: LucideIcon | undefined;
  children: ReactNode;
  onClick: () => void;
  disabled?: boolean | undefined;
}>) {
  const deck = useDeckScale();
  if (!deck) {
    return (
      <Button variant="outline" size="sm" disabled={disabled} onClick={onClick}>
        {children}
      </Button>
    );
  }
  return (
    <button type="button" className={DECK_BUTTON} disabled={disabled} onClick={onClick}>
      {Icon === undefined ? null : (
        <Icon aria-hidden="true" className="size-3.5 flex-none" />
      )}
      {children}
    </button>
  );
}

/**
 * BulkActions is the bar a list shows while rows are selected. Each action
 * confirms the explicit selection, runs on one item at a time and reports
 * each failure without hiding partial success; `children` are a page's own
 * controls, drawn before the actions. On a deck surface it is the deck's
 * inverted bar: the count, each action with its icon, and an icon button that
 * clears. There `className` lands on the bar, and with `hideOnPhone` the bar
 * is not drawn below 768px, for a list that has no checkboxes there, while an
 * open confirmation stays open. Off a deck surface the bar is the one the old
 * console draws, at every width, and reads neither prop.
 */
export function BulkActions<T extends { id: string }>({
  selected,
  children,
  selectionLabel,
  name,
  actions,
  onRemoved,
  onClear,
  onSettled,
  className,
  hideOnPhone = false,
}: Readonly<{
  selected: readonly T[];
  children?: ReactNode;
  selectionLabel?: string;
  name: (item: T) => string;
  actions: readonly BulkAction<T>[];
  onRemoved: (ids: string[]) => void;
  onClear: () => void;
  onSettled: () => Promise<unknown>;
  className?: string | undefined;
  hideOnPhone?: boolean | undefined;
}>) {
  const { t } = useTranslation();
  const deck = useDeckScale();
  const wide = useMediaQuery("(min-width: 768px)");
  const [confirmation, setConfirmation] = useState<{
    action: BulkAction<T>;
    items: readonly T[];
  } | null>(null);
  const [pending, setPending] = useState(false);
  const [completed, setCompleted] = useState(0);
  const [failures, setFailures] = useState<
    { id: string; name: string; message: string }[]
  >([]);

  async function run() {
    if (!confirmation || pending) return;
    setPending(true);
    setCompleted(0);
    setFailures([]);
    try {
      const { removed, failed } = await performSelected(
        confirmation,
        name,
        t("common.actionFailed"),
        setCompleted,
      );
      onRemoved(removed);
      setFailures(failed);
      if (failed.length === 0) setConfirmation(null);
      else {
        const failedIds = new Set(failed.map((item) => item.id));
        setConfirmation({
          ...confirmation,
          items: confirmation.items.filter((item) => failedIds.has(item.id)),
        });
      }
      if (removed.length > 0)
        toast(t("common.bulkCompleted", { count: removed.length }));
      await onSettled().catch(() => toast(t("common.refreshFailed")));
    } finally {
      setPending(false);
    }
  }

  if (selected.length === 0 && confirmation === null) return null;
  const buttons = actions.map((action) => (
    <BulkBarButton
      key={action.label}
      icon={action.icon}
      disabled={pending}
      onClick={() => {
        setFailures([]);
        setConfirmation({ action, items: [...selected] });
      }}
    >
      {action.label}
    </BulkBarButton>
  ));
  return (
    <>
      {deck ? (
        (wide || !hideOnPhone) && (
          <div className={cn(DECK_BAR, className)}>
            <span className={DECK_COUNT}>
              {selectionLabel ?? t("bulkBar.selected", { count: selected.length })}
            </span>
            {children}
            {buttons}
            <button
              type="button"
              aria-label={t("common.clearSelection")}
              className={DECK_CLEAR}
              disabled={pending}
              onClick={onClear}
            >
              <X aria-hidden="true" className="size-3.75" />
            </button>
          </div>
        )
      ) : (
        <div className="bg-secondary flex flex-wrap items-center gap-2 rounded-md px-3 py-2">
          <span className="mr-auto text-sm font-medium">
            {selectionLabel ?? t("common.selectedCount", { count: selected.length })}
          </span>
          {children}
          {buttons}
          <Button variant="ghost" size="sm" disabled={pending} onClick={onClear}>
            {t("common.clearSelection")}
          </Button>
        </div>
      )}
      <ConfirmDialog
        open={confirmation !== null}
        onOpenChange={(open) => {
          if (!open && !pending) setConfirmation(null);
        }}
        title={confirmation?.action.label ?? ""}
        description={confirmation?.action.description ?? ""}
        confirmLabel={t(
          failures.length > 0 ? "common.retryFailed" : "common.confirmSelected",
          { count: confirmation?.items.length ?? 0 },
        )}
        destructive={confirmation?.action.destructive ?? true}
        pending={pending}
        onConfirm={() => void run()}
      >
        <ul className="max-h-48 space-y-1 overflow-y-auto text-sm">
          {confirmation?.items.map((item) => (
            <li key={item.id}>{name(item)}</li>
          ))}
        </ul>
        {pending ? (
          <p role="status" className="text-muted-foreground text-sm">
            {t("common.bulkProgress", {
              completed,
              total: confirmation?.items.length ?? 0,
            })}
          </p>
        ) : null}
        {failures.length > 0 ? (
          <ul role="alert" className="text-destructive space-y-1 text-sm">
            {failures.map((item) => (
              <li key={item.id}>
                {item.name}: {item.message}
              </li>
            ))}
          </ul>
        ) : null}
      </ConfirmDialog>
    </>
  );
}

async function performSelected<T extends { id: string }>(
  selection: { action: BulkAction<T>; items: readonly T[] },
  name: (item: T) => string,
  fallback: string,
  progress: (count: number) => void,
) {
  const removed: string[] = [];
  const failed: { id: string; name: string; message: string }[] = [];
  for (const item of selection.items) {
    try {
      await selection.action.run(item);
      removed.push(item.id);
    } catch (cause) {
      failed.push({
        id: item.id,
        name: name(item),
        message: cause instanceof ApiError ? cause.message : fallback,
      });
    }
    progress(removed.length + failed.length);
  }
  return { removed, failed };
}
