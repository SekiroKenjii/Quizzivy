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
  onGoToSection?: (sectionId: string) => void;
  warnings?: { questionId: string; message: string }[];
  location?: (violation: PublishViolation) => string | null;
}

/**
 * A-05's publish gate. §8 lists rules a test can fail in several places at
 * once, and a toast cannot express five failures with locations -- so every
 * blocking issue is a line with a jump link.
 */
export function PublishDialog({
  violations,
  onClose,
  onGoTo,
  onGoToSection,
  warnings = [],
  location,
}: Readonly<PublishDialogProps>) {
  const { t } = useTranslation();

  return (
    <Dialog open={violations !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto">
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
                {location?.(violation) && (
                  <span className="mb-0.5 block font-medium">
                    {location(violation)}
                  </span>
                )}
                {violation.message}
              </p>
              {violation.questionId || (violation.sectionId && onGoToSection) ? (
                <Button
                  variant="outline"
                  size="xs"
                  aria-describedby={messageId(index)}
                  onClick={() => {
                    if (violation.questionId) onGoTo(violation.questionId);
                    else if (violation.sectionId) onGoToSection?.(violation.sectionId);
                  }}
                >
                  {t("builder.goToQuestion")}
                </Button>
              ) : null}
            </div>
          ))}
        </div>

        {warnings.length > 0 && (
          <div className="space-y-2 border-t pt-3">
            <p className="text-xs font-medium">{t("builder.publishWarnings")}</p>
            {warnings.map((warning) => (
              <div
                key={warning.questionId}
                className="text-muted-foreground flex items-center gap-2 text-sm"
              >
                <span className="flex-1">{warning.message}</span>
                <Button
                  variant="ghost"
                  size="xs"
                  onClick={() => onGoTo(warning.questionId)}
                >
                  {t("builder.goToQuestion")}
                </Button>
              </div>
            ))}
          </div>
        )}

        <DialogFooter>
          <Button variant="outline" className="flex-1" onClick={onClose}>
            {t("builder.later")}
          </Button>
          <Button className="flex-1" disabled>
            {t("builder.publish")}
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
