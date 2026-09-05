import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Check } from "lucide-react";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { toast } from "@/components/ui/sonner";
import { updateClass, type Class } from "@/features/classes/api";
import { invalidateClass } from "@/features/classes/invalidate";

/** G-08's "Lưu trữ lớp": a confirm, not a delete, repeating the two numbers it leaves alone. */
export function ArchiveClassDialog({
  klass,
  onOpenChange,
}: Readonly<{
  klass: Class | null;
  onOpenChange: (open: boolean) => void;
}>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();

  const restore = useMutation({
    mutationFn: (id: string) => updateClass(id, { archived: false }),
    onSuccess: (_, id) => invalidateClass(queryClient, id),
  });
  const archive = useMutation({
    mutationFn: (id: string) => updateClass(id, { archived: true }),
    onSuccess: async (_, id) => {
      await invalidateClass(queryClient, id);
      onOpenChange(false);
      toast(t("classes.archived"), {
        action: { label: t("common.undo"), onClick: () => restore.mutate(id) },
      });
    },
  });

  if (klass === null) return null;
  return (
    <ConfirmDialog
      open
      onOpenChange={(next) => {
        if (!next) archive.reset();
        onOpenChange(next);
      }}
      title={t("classes.archiveTitle", { name: klass.name })}
      description={t("classes.archiveBody")}
      confirmLabel={t("classes.archive")}
      pending={archive.isPending}
      error={archive.isError ? t("classes.archiveFailed") : null}
      onConfirm={() => archive.mutate(klass.id)}
    >
      <ul className="space-y-1.5 text-sm">
        <Line>{t("classes.archiveStudents", { count: klass.studentCount })}</Line>
        <Line>
          {klass.openAssignmentCount === 0
            ? t("classes.archiveNoneOpen")
            : t("classes.archiveOpen", { count: klass.openAssignmentCount })}
        </Line>
        <Line>{t("classes.archiveRestoreNote")}</Line>
      </ul>
    </ConfirmDialog>
  );
}

function Line({ children }: Readonly<{ children: string }>) {
  return (
    <li className="flex items-start gap-2">
      <Check
        className="text-muted-foreground mt-0.5 size-4 shrink-0"
        aria-hidden="true"
      />
      <span>{children}</span>
    </li>
  );
}
