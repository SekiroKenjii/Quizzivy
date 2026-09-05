import { useTranslation } from "react-i18next";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { StudentPreview } from "@/features/tests/components/StudentPreview";
import type { AdminQuestion } from "@/features/question-bank/api";
import type { components } from "@/lib/api/schema";

type StudentQuestion = components["schemas"]["StudentQuestion"];

/**
 * A-04's "Xem như học viên" for a draft: the outline's questions in the shape
 * a student receives, with the key stripped, since nothing published exists
 * to preview yet.
 */
export function DraftPreviewDialog({
  open,
  questions,
  onOpenChange,
}: Readonly<{
  open: boolean;
  questions: AdminQuestion[];
  onOpenChange: (open: boolean) => void;
}>) {
  const { t } = useTranslation();
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85svh] max-w-3xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t("builder.previewAsStudent")}</DialogTitle>
          <DialogDescription>{t("builder.previewHint")}</DialogDescription>
        </DialogHeader>
        {questions.length === 0 ? (
          <p className="text-muted-foreground text-sm">{t("builder.previewEmpty")}</p>
        ) : (
          <StudentPreview questions={questions.map(asStudent)} />
        )}
      </DialogContent>
    </Dialog>
  );
}

/** §13.5's boundary, applied client-side: nothing that grades survives the mapping. */
function asStudent(q: AdminQuestion): StudentQuestion {
  return {
    id: q.id,
    type: q.type,
    prompt: q.prompt,
    points: q.points,
    media: q.media ?? null,
    audio: q.audio ?? null,
    options: (q.options ?? []).map((o) => ({ id: o.id, text: o.text })),
    blanks: (q.blanks ?? []).map((b) => ({ id: b.id, ordinal: b.ordinal })),
  };
}
