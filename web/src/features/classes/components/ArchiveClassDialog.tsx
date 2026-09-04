import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Check } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { updateClass, type Class } from "@/features/classes/api";
import { invalidateClass } from "@/features/classes/invalidate";

/** G-08's "Lưu trữ lớp": a confirm, not a delete, repeating the two numbers it leaves alone. */
export function ArchiveClassDialog({
  klass,
  onOpenChange,
}: {
  klass: Class | null;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [failed, setFailed] = useState(false);

  const archive = useMutation({
    mutationFn: (id: string) => updateClass(id, { archived: true }),
    onSuccess: async (_, id) => {
      await invalidateClass(queryClient, id);
      setFailed(false);
      onOpenChange(false);
    },
    onError: () => setFailed(true),
  });

  return (
    <Dialog
      open={klass !== null}
      onOpenChange={(next) => {
        if (!next) setFailed(false);
        onOpenChange(next);
      }}
    >
      {klass === null ? null : (
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("classes.archiveTitle", { name: klass.name })}</DialogTitle>
            <DialogDescription>{t("classes.archiveBody")}</DialogDescription>
          </DialogHeader>
          <ul className="mt-1 space-y-1.5 text-sm">
            <Line>{t("classes.archiveStudents", { count: klass.studentCount })}</Line>
            <Line>
              {klass.openAssignmentCount === 0
                ? t("classes.archiveNoneOpen")
                : t("classes.archiveOpen", { count: klass.openAssignmentCount })}
            </Line>
            <Line>{t("classes.archiveRestoreNote")}</Line>
          </ul>
          {failed && (
            <p role="alert" className="text-destructive text-sm">
              {t("classes.archiveFailed")}
            </p>
          )}
          <DialogFooter className="mt-3">
            <Button
              variant="outline"
              className="flex-1"
              onClick={() => onOpenChange(false)}
            >
              {t("common.cancel")}
            </Button>
            <Button
              className="flex-1"
              disabled={archive.isPending}
              onClick={() => archive.mutate(klass.id)}
            >
              {t("classes.archive")}
            </Button>
          </DialogFooter>
        </DialogContent>
      )}
    </Dialog>
  );
}

function Line({ children }: { children: string }) {
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
