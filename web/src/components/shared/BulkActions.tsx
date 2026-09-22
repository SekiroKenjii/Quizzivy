import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { ApiError } from "@/lib/api/errors";
import { toast } from "@/components/ui/sonner";

export interface BulkAction<T> {
  label: string;
  description: string;
  run: (item: T) => Promise<unknown>;
}

/** BulkActions confirms an explicit selection and reports each failure without hiding partial success. */
export function BulkActions<T extends { id: string }>({
  selected,
  children,
  selectionLabel,
  name,
  actions,
  onRemoved,
  onClear,
  onSettled,
}: Readonly<{
  selected: readonly T[];
  children?: ReactNode;
  selectionLabel?: string;
  name: (item: T) => string;
  actions: readonly BulkAction<T>[];
  onRemoved: (ids: string[]) => void;
  onClear: () => void;
  onSettled: () => Promise<unknown>;
}>) {
  const { t } = useTranslation();
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
  return (
    <>
      <div className="bg-secondary flex flex-wrap items-center gap-2 rounded-md px-3 py-2">
        <span className="mr-auto text-sm font-medium">
          {selectionLabel ?? t("common.selectedCount", { count: selected.length })}
        </span>
        {children}
        {actions.map((action) => (
          <Button
            key={action.label}
            variant="outline"
            size="sm"
            disabled={pending}
            onClick={() => {
              setFailures([]);
              setConfirmation({ action, items: [...selected] });
            }}
          >
            {action.label}
          </Button>
        ))}
        <Button variant="ghost" size="sm" disabled={pending} onClick={onClear}>
          {t("common.clearSelection")}
        </Button>
      </div>
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
        destructive
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
