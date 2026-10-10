import { useLayoutEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { ChevronLeft, ChevronRight } from "lucide-react";
import {
  DialogShell,
  DialogShellBody,
  DialogShellFooter,
  DialogShellHeader,
} from "@/components/shared/form/DialogShell";
import { Button } from "@/components/ui/button";
import { StudentPreviewPane } from "@/features/tests/components/StudentPreviewPane";
import type { AdminQuestion } from "@/features/question-bank/api";
import type { components } from "@/lib/api/schema";
import type { StoredGroup } from "@/features/question-groups/api";
import { groupPreview } from "@/features/question-groups/preview";
import type { OutlineSection } from "../outline";
import { unitsOf } from "../outlineUnits";

type StudentQuestion = components["schemas"]["StudentQuestion"];

/**
 * DraftPreviewDialog is the builder's "Student preview" for a draft: the
 * outline's questions in the shape a student receives, with the key
 * stripped, drawn by the engine's readers through `StudentPreview`. Previous
 * and Next step through the questions, scrolling each into view with its
 * passage above it, and it opens on `startAt`, the question being edited.
 */
export function DraftPreviewDialog({
  open,
  questions,
  sections,
  groups = [],
  startAt = null,
  onOpenChange,
}: Readonly<{
  open: boolean;
  questions: { sectionId: string; question: AdminQuestion }[];
  sections?: OutlineSection[];
  groups?: StoredGroup[];
  startAt?: string | null;
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
    <DialogShell open={open} onOpenChange={onOpenChange} width={720}>
      <DialogShellHeader
        title={t("builder.previewAsStudent")}
        description={t("builder.previewHint")}
      />
      {open ? (
        <Pages
          questions={displayed}
          groups={shared}
          start={Math.max(
            0,
            displayed.findIndex((question) => question.id === startAt),
          )}
          onClose={() => onOpenChange(false)}
        />
      ) : null}
    </DialogShell>
  );
}

function Pages({
  questions,
  groups,
  start,
  onClose,
}: Readonly<{
  questions: StudentQuestion[];
  groups: components["schemas"]["StudentGroup"][];
  start: number;
  onClose: () => void;
}>) {
  const { t } = useTranslation();
  const body = useRef<HTMLDivElement>(null);
  const [index, setIndex] = useState(start);
  const total = questions.length;
  useLayoutEffect(() => {
    const item = body.current?.querySelector("ol")?.children[index];
    if (item instanceof HTMLElement) item.scrollIntoView?.({ block: "start" });
  }, [index]);
  return (
    <>
      <DialogShellBody ref={body}>
        {total === 0 ? (
          <p className="text-muted-fg text-sm">{t("builder.previewEmpty")}</p>
        ) : (
          <StudentPreviewPane questions={questions} groups={groups} />
        )}
      </DialogShellBody>
      <DialogShellFooter className="items-center justify-between">
        {total > 0 ? (
          <span className="flex items-center gap-2">
            <Button
              type="button"
              variant="outline"
              disabled={index === 0}
              onClick={() => setIndex(index - 1)}
            >
              <ChevronLeft aria-hidden="true" />
              {t("builder.previewPrevious")}
            </Button>
            <span role="status" className="text-muted-fg text-sm tabular-nums">
              {t("builder.previewPosition", { n: index + 1, total })}
            </span>
            <Button
              type="button"
              variant="outline"
              disabled={index >= total - 1}
              onClick={() => setIndex(index + 1)}
            >
              {t("builder.previewNext")}
              <ChevronRight aria-hidden="true" />
            </Button>
          </span>
        ) : (
          <span />
        )}
        <Button type="button" onClick={onClose}>
          {t("common.close")}
        </Button>
      </DialogShellFooter>
    </>
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
