import { useState, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { ApiError } from "@/lib/api/errors";
import { toast } from "@/components/ui/sonner";

/**
 * DeleteItemDialog confirms the permanent deletion of one item named `name`.
 * Confirming runs `onDelete`; on success it closes, toasts and runs
 * `onDeleted`, and a failure stays in the dialog as its alert. It cannot be
 * closed while the deletion runs. `returnFocus` is where focus goes when it
 * closes, for a dialog opened from a menu.
 */
export function DeleteItemDialog({
  open,
  onOpenChange,
  name,
  onDelete,
  onDeleted,
  returnFocus,
}: Readonly<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  name: string;
  onDelete: () => Promise<unknown>;
  onDeleted: () => Promise<unknown>;
  returnFocus?: RefObject<HTMLElement | null> | undefined;
}>) {
  const { t } = useTranslation();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [wasOpen, setWasOpen] = useState(open);
  if (wasOpen !== open) {
    setWasOpen(open);
    if (open) setError(null);
  }
  async function remove() {
    setPending(true);
    setError(null);
    try {
      await onDelete();
      onOpenChange(false);
      toast(t("common.deleted"));
      await onDeleted().catch(() => toast(t("common.refreshFailed")));
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : t("common.actionFailed"));
    } finally {
      setPending(false);
    }
  }
  return (
    <ConfirmDialog
      open={open}
      onOpenChange={(next) => {
        if (!pending) onOpenChange(next);
      }}
      title={t("common.deletePermanently")}
      description={t("common.permanentDeleteBody")}
      confirmLabel={t("common.deletePermanently")}
      destructive
      pending={pending}
      {...(returnFocus === undefined ? {} : { returnFocus })}
      onConfirm={() => void remove()}
    >
      <p className="text-sm">{name}</p>
      {error ? (
        <p role="alert" className="text-destructive text-sm">
          {error}
        </p>
      ) : null}
    </ConfirmDialog>
  );
}

/** DeleteItemButton is a row's trash button, which opens DeleteItemDialog for the item. */
export function DeleteItemButton({
  name,
  onDelete,
  onDeleted,
}: Readonly<{
  name: string;
  onDelete: () => Promise<unknown>;
  onDeleted: () => Promise<unknown>;
}>) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button
        variant="ghost"
        size="icon-xs"
        aria-label={t("common.deleteNamed", { name })}
        onClick={() => setOpen(true)}
      >
        <Trash2 aria-hidden="true" />
      </Button>
      <DeleteItemDialog
        open={open}
        onOpenChange={setOpen}
        name={name}
        onDelete={onDelete}
        onDeleted={onDeleted}
      />
    </>
  );
}
