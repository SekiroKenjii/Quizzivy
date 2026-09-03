import { useTranslation } from "react-i18next";
import { CircleAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { PublishViolation } from "@/features/tests/api";

interface PublishDialogProps {
  violations: PublishViolation[] | null;
  onClose: () => void;
  onGoTo: (questionId: string) => void;
}

/**
 * A-05's publish gate. §8 lists rules a test can fail in several places at
 * once, and a toast cannot express five failures with locations -- so every
 * blocking issue is a line with a jump link.
 */
export function PublishDialog({ violations, onClose, onGoTo }: PublishDialogProps) {
  const { t } = useTranslation();

  return (
    <Dialog open={violations !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("builder.publishBlockedTitle")}</DialogTitle>
          <DialogDescription>
            {t("builder.publishBlockedBody", { count: violations?.length ?? 0 })}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-1.5">
          {(violations ?? []).map((violation, index) => (
            <div
              key={`${violation.rule}-${violation.questionId ?? violation.sectionId ?? index}`}
              className="flex items-start gap-2.5 rounded-md border p-2.5"
            >
              <CircleAlert
                className="text-destructive mt-0.5 size-4 shrink-0"
                aria-hidden="true"
              />
              <p id={messageId(index)} className="min-w-0 flex-1 text-sm">
                {violation.message}
              </p>
              {violation.questionId ? (
                <Button
                  variant="outline"
                  size="xs"
                  aria-describedby={messageId(index)}
                  onClick={() => onGoTo(violation.questionId!)}
                >
                  {t("builder.goToQuestion")}
                </Button>
              ) : null}
            </div>
          ))}
        </div>

        <DialogFooter>
          <Button variant="outline" className="w-full" onClick={onClose}>
            {t("builder.later")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// Every row's button reads "Đi tới"; the message beside it is what says where.
// Linking them is what lets a screen reader announce the two together.
function messageId(index: number): string {
  return `publish-violation-${index}`;
}
