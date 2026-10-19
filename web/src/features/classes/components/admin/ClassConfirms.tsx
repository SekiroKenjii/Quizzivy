import type { RefObject } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { toast } from "@/components/ui/sonner";
import { invalidateAdminClasses, type AdminClass } from "@/features/classes/adminApi";
import { rotateJoinCode, updateClass } from "@/features/classes/api";
import { invalidateClass } from "@/features/classes/invalidate";
import { ApiError } from "@/lib/api/errors";

const NEW_CODE = { expiresInDays: 30, maxUses: 40 } as const;

function message(cause: unknown, fallback: string): string {
  return cause instanceof ApiError ? cause.message : fallback;
}

/**
 * RotateCodeDialog asks before rotating a class's join code, in the deck's
 * words, and toasts the new code in full; for a class without a code it asks
 * to create one. The code lasts 30 days and 40 uses, as a teacher's does. It
 * is open while `klass` is set.
 */
export function RotateCodeDialog({
  klass,
  onClose,
  returnFocus,
}: Readonly<{
  klass: AdminClass | null;
  onClose: () => void;
  returnFocus: RefObject<HTMLElement | null>;
}>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const rotate = useMutation({
    mutationFn: (target: AdminClass) => rotateJoinCode(target.id, NEW_CODE),
    onSuccess: async (issued, target) => {
      onClose();
      toast(t("classes.codeRenewedWith", { code: issued.code }));
      await invalidateClass(queryClient, target.id);
      await invalidateAdminClasses(queryClient);
    },
    onError: () => invalidateAdminClasses(queryClient),
  });
  const fresh = klass?.joinCode === null;
  return (
    <ConfirmDialog
      open={klass !== null}
      onOpenChange={(open) => {
        if (open) return;
        rotate.reset();
        onClose();
      }}
      title={
        fresh
          ? t("classes.createCodeTitle")
          : t("classes.admin.rotateTitle", { name: klass?.name ?? "" })
      }
      description={t(fresh ? "classes.createCodeBody" : "classes.admin.rotateBody")}
      confirmLabel={t(fresh ? "classes.createCode" : "classes.admin.rotateConfirm")}
      pending={rotate.isPending}
      error={rotate.isError ? message(rotate.error, t("classes.newCodeFailed")) : null}
      returnFocus={returnFocus}
      onConfirm={() => {
        if (klass !== null && !rotate.isPending) rotate.mutate(klass);
      }}
    />
  );
}

/**
 * ArchiveAdminClassDialog asks before archiving a class, in the deck's words,
 * and calls `onArchived` once the class is archived and before the dialog
 * closes, so the caller can move where focus returns: the row is gone. It is
 * open while `klass` is set.
 */
export function ArchiveAdminClassDialog({
  klass,
  onClose,
  onArchived,
  returnFocus,
}: Readonly<{
  klass: AdminClass | null;
  onClose: () => void;
  onArchived: () => void;
  returnFocus: RefObject<HTMLElement | null>;
}>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const archive = useMutation({
    mutationFn: (target: AdminClass) => updateClass(target.id, { archived: true }),
    onSuccess: async (_, target) => {
      onArchived();
      onClose();
      toast(t("classes.archived"));
      await invalidateClass(queryClient, target.id);
      await invalidateAdminClasses(queryClient);
    },
    onError: () => invalidateAdminClasses(queryClient),
  });
  return (
    <ConfirmDialog
      open={klass !== null}
      destructive
      onOpenChange={(open) => {
        if (open) return;
        archive.reset();
        onClose();
      }}
      title={t("classes.admin.archiveTitle", { name: klass?.name ?? "" })}
      description={t("classes.admin.archiveBody")}
      confirmLabel={t("classes.archive")}
      pending={archive.isPending}
      error={
        archive.isError ? message(archive.error, t("classes.archiveFailed")) : null
      }
      returnFocus={returnFocus}
      onConfirm={() => {
        if (klass !== null && !archive.isPending) archive.mutate(klass);
      }}
    />
  );
}
