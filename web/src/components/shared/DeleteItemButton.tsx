import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { ApiError } from "@/lib/api/errors";
import { toast } from "@/components/ui/sonner";

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
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  async function remove() {
    setPending(true);
    setError(null);
    try {
      await onDelete();
      setOpen(false);
      toast(t("common.deleted"));
      await onDeleted().catch(() => toast(t("common.refreshFailed")));
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : t("common.actionFailed"));
    } finally {
      setPending(false);
    }
  }
  return (
    <>
      <Button
        variant="ghost"
        size="icon-xs"
        aria-label={t("common.deleteNamed", { name })}
        onClick={() => {
          setError(null);
          setOpen(true);
        }}
      >
        <Trash2 aria-hidden="true" />
      </Button>
      <ConfirmDialog
        open={open}
        onOpenChange={(next) => {
          if (!pending) setOpen(next);
        }}
        title={t("common.deletePermanently")}
        description={t("common.permanentDeleteBody")}
        confirmLabel={t("common.deletePermanently")}
        destructive
        pending={pending}
        onConfirm={() => void remove()}
      >
        <p className="text-sm">{name}</p>
        {error ? (
          <p role="alert" className="text-destructive text-sm">
            {error}
          </p>
        ) : null}
      </ConfirmDialog>
    </>
  );
}
