import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Eye, Monitor, Smartphone, X } from "lucide-react";
import { Dialog as Primitive } from "radix-ui";
import { Segmented } from "@/components/ui/segmented";
import { StudentPreview } from "@/features/tests/components/StudentPreview";
import { StudentPreviewPane } from "@/features/tests/components/StudentPreviewPane";
import {
  WINDOW_FRAME,
  type PreviewDevice,
} from "@/features/tests/components/previewFrame";
import type { AdminQuestion } from "@/features/question-bank/api";
import type { components } from "@/lib/api/schema";
import type { StoredGroup } from "@/features/question-groups/api";
import { groupPreview, studentAudioPolicy } from "@/features/question-groups/preview";
import { preventLocalEscapeDismissal } from "@/lib/localEscape";
import type { OutlineSection } from "../outline";
import { unitsOf } from "../outlineUnits";

type StudentQuestion = components["schemas"]["StudentQuestion"];

/**
 * DraftPreviewDialog is the builder's "Student preview" for a draft: a modal
 * over the whole window whose bar names the test, switches the frame between
 * Computer and Phone and closes it, and whose frame holds the outline's
 * questions in the shape a student receives, with the key stripped, drawn
 * by the engine's readers through `StudentPreview`. It writes nothing; the
 * builder saves what is pending before it opens it. Esc and "Close preview"
 * close it through `onOpenChange`, focus stays inside while it is open, and
 * on close it returns to the control that opened it, or to the `<main>` that
 * control was in when it has gone.
 */
export function DraftPreviewDialog({
  open,
  title,
  questions,
  sections,
  groups = [],
  onOpenChange,
}: Readonly<{
  open: boolean;
  title: string;
  questions: { sectionId: string; question: AdminQuestion }[];
  sections?: OutlineSection[];
  groups?: StoredGroup[];
  onOpenChange: (open: boolean) => void;
}>) {
  const { t } = useTranslation();
  const [device, setDevice] = useState<PreviewDevice>("desktop");
  const opener = useRef<HTMLElement | null>(null);
  const page = useRef<HTMLElement | null>(null);
  return (
    <Primitive.Root open={open} onOpenChange={onOpenChange}>
      <Primitive.Portal>
        <Primitive.Overlay className="bg-overlay fixed inset-0 z-(--z-dialog)" />
        <Primitive.Content
          data-scale="deck"
          aria-label={t("builder.previewAsStudent")}
          aria-labelledby={undefined}
          aria-describedby={undefined}
          className="text-fg fixed inset-0 z-(--z-dialog) flex flex-col gap-3 p-3.5 outline-none!"
          onEscapeKeyDown={preventLocalEscapeDismissal}
          onOpenAutoFocus={() => {
            const active = document.activeElement;
            opener.current =
              active instanceof HTMLElement && active !== document.body ? active : null;
            page.current = opener.current?.closest("main") ?? null;
          }}
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            const target = opener.current?.isConnected ? opener.current : page.current;
            opener.current = null;
            page.current = null;
            const active = document.activeElement;
            if (active === null || active === document.body) target?.focus();
          }}
        >
          <div className="bg-card shadow-float flex flex-none flex-wrap items-center gap-2.5 rounded-xl border py-2 pr-2.5 pl-3.5">
            <Eye aria-hidden="true" className="size-4 flex-none" />
            <Primitive.Title className="min-w-0 flex-1 truncate text-base font-semibold">
              {t("builder.previewTitle", { title })}
            </Primitive.Title>
            <Segmented
              size="sm"
              label={t("preview.displayMode")}
              value={device}
              onChange={(value) => setDevice(value === "phone" ? "phone" : "desktop")}
              options={[
                { value: "desktop", label: t("preview.desktop"), icon: Monitor },
                { value: "phone", label: t("preview.phone"), icon: Smartphone },
              ]}
            />
            <Primitive.Close
              aria-label={t("builder.previewClose")}
              className="hover:bg-hover grid size-8.5 flex-none cursor-pointer place-items-center rounded-[8px]"
            >
              <X aria-hidden="true" className="size-4.5" />
            </Primitive.Close>
          </div>
          <div className="flex min-h-0 flex-1 justify-center">
            <StudentPreviewPane device={device} sizes={WINDOW_FRAME}>
              {open ? (
                <DraftPaper questions={questions} sections={sections} groups={groups} />
              ) : null}
            </StudentPreviewPane>
          </div>
        </Primitive.Content>
      </Primitive.Portal>
    </Primitive.Root>
  );
}

function DraftPaper({
  questions,
  sections,
  groups,
}: Readonly<{
  questions: { sectionId: string; question: AdminQuestion }[];
  sections: OutlineSection[] | undefined;
  groups: StoredGroup[];
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
  if (displayed.length === 0)
    return <p className="text-muted-fg text-sm">{t("builder.previewEmpty")}</p>;
  return <StudentPreview questions={displayed} groups={shared} />;
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
    audio: q.audio ? studentAudioPolicy(q.audio) : null,
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
