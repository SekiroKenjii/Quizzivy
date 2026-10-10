import { useTranslation } from "react-i18next";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { StudentPreviewPane } from "@/features/tests/components/StudentPreviewPane";
import type { AdminQuestion } from "@/features/question-bank/api";
import type { components } from "@/lib/api/schema";
import type { StoredGroup } from "@/features/question-groups/api";
import { groupPreview } from "@/features/question-groups/preview";
import type { OutlineSection } from "../outline";
import { unitsOf } from "../outlineUnits";

type StudentQuestion = components["schemas"]["StudentQuestion"];

/**
 * A-04's "Xem như học viên" for a draft: the outline's questions in the shape
 * a student receives, with the key stripped, since nothing published exists
 * to preview yet.
 */
export function DraftPreviewDialog({
  open,
  questions,
  sections,
  groups = [],
  onOpenChange,
}: Readonly<{
  open: boolean;
  questions: { sectionId: string; question: AdminQuestion }[];
  sections?: OutlineSection[];
  groups?: StoredGroup[];
  onOpenChange: (open: boolean) => void;
}>) {
  const { t } = useTranslation();
  const standalone = new Map(
    questions.map(({ sectionId, question }) => [
      question.id,
      asStudent(sectionId, question),
    ]),
  );
  const contexts = new Map(
    groups.map((group) => [
      group.bundle.group.id,
      groupPreview(group.bundle, group.assets),
    ]),
  );
  const displayed = sections
    ? sections.flatMap((section) =>
        unitsOf(section).flatMap((unit) => {
          if (unit.kind === "group")
            return (
              contexts.get(unit.id)?.questions.map((question) => ({
                ...question,
                sectionId: section.id ?? "",
              })) ?? []
            );
          const question = standalone.get(unit.id);
          return question ? [question] : [];
        }),
      )
    : [...standalone.values()];
  const shared =
    sections?.flatMap((section) =>
      unitsOf(section).flatMap((unit) =>
        unit.kind === "group"
          ? (contexts
              .get(unit.id)
              ?.groups.map((group) => ({ ...group, sectionId: section.id ?? "" })) ??
            [])
          : [],
      ),
    ) ?? [];
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85svh] max-w-3xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t("builder.previewAsStudent")}</DialogTitle>
          <DialogDescription>{t("builder.previewHint")}</DialogDescription>
        </DialogHeader>
        {displayed.length === 0 ? (
          <p className="text-muted-foreground text-sm">{t("builder.previewEmpty")}</p>
        ) : (
          <StudentPreviewPane questions={displayed} groups={shared} />
        )}
      </DialogContent>
    </Dialog>
  );
}

/** §13.5's boundary, applied client-side: nothing that grades survives the mapping. */
function asStudent(sectionId: string, q: AdminQuestion): StudentQuestion {
  return {
    id: q.id,
    sectionId,
    type: q.type,
    prompt: q.prompt,
    promptContent: q.promptContent ?? null,
    points: q.points,
    media: q.media ?? null,
    audio: q.audio ?? null,
    options: (q.options ?? []).map((o) => ({
      id: o.id,
      text: o.text,
      content: o.content ?? null,
    })),
    blanks: (q.blanks ?? []).map((b) => ({
      id: b.id,
      ordinal: b.ordinal,
      gapId: b.gapId ?? null,
      caseSensitive: b.caseSensitive ?? false,
    })),
  };
}
