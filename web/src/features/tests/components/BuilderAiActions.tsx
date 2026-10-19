import { lazy, Suspense, useState } from "react";
import { useTranslation } from "react-i18next";
import { ScanSearch, Sparkles } from "lucide-react";
import type { AiFinding } from "@/features/ai/api";
import type { AiPanelRequest } from "@/features/ai/components/AiPanel";
import { AiEntryButton } from "@/features/ai/components/AiEntryButton";
import { AiWriteError } from "@/features/ai/kind";
import type {
  AiPassage,
  AiSection,
  GeneratedDraft,
  GenerateDestination,
} from "@/features/ai/kinds/generate";
import { useAiAvailable, useAiBlock } from "@/features/ai/useAiAvailable";
import { draftQuestion } from "@/features/question-bank/aiDrafts";
import {
  questionSchema,
  type QuestionValues,
} from "@/features/question-bank/questionSchema";

const AiPanel = lazy(() => import("@/features/ai/components/AiPanel"));

const ENTRY = "shadow-card h-9";

type BuilderPanel = "generate" | "check";

/**
 * BuilderAi is what the builder hands its AI panels: the open test, its
 * passages, each with the group that holds it, the section new questions join
 * and its id (null when it has none, and the id null while it is unsaved),
 * every saved section a standalone run can be fitted to,
 * and the builder's own writes and moves. `prepare` saves every pending edit
 * before a panel opens; `add` creates the kept questions and appends them to
 * the section `sectionId` names, or else to that section; `addToGroup` appends them to
 * the group whose passage they were generated from, as new members after its
 * existing ones; `fix` writes a check's fix into its question.
 */
export interface BuilderAi {
  testId: string;
  title: string;
  passages: readonly AiPassage[];
  section: string | null;
  sectionId: string | null;
  sections: readonly AiSection[];
  prepare: () => Promise<void>;
  add: (questions: QuestionValues[], sectionId?: string) => Promise<void>;
  addToGroup: (groupId: string, questions: QuestionValues[]) => Promise<void>;
  fix: (finding: AiFinding) => Promise<void>;
  open: (questionId: string) => void;
  numberOf: (questionId: string) => number | null;
  onPrepareFailed: (cause: unknown) => void;
}

/**
 * BuilderAiActions are the title bar's "AI check" and "Generate" (V-09) and
 * the panel each opens. Neither is drawn without the assistant or `ai.use`;
 * both are drawn disabled with the reason while the teacher is paused or out
 * of credits (Q34). Like Preview, each saves pending edits before it opens.
 */
export function BuilderAiActions({ ai }: Readonly<{ ai: BuilderAi }>) {
  const { t } = useTranslation();
  const available = useAiAvailable();
  const blocked = useAiBlock(available);
  const [open, setOpen] = useState<BuilderPanel | null>(null);

  if (!available) return null;

  async function start(kind: BuilderPanel) {
    try {
      await ai.prepare();
      setOpen(kind);
    } catch (cause) {
      ai.onPrepareFailed(cause);
    }
  }

  async function save(
    drafts: readonly GeneratedDraft[],
    tag: string | null,
    destination: GenerateDestination,
  ) {
    const values = drafts.map((draft) => draftQuestion(draft, tag));
    const invalid = values.findIndex(
      (value) => !questionSchema.safeParse(value).success,
    );
    if (invalid >= 0)
      throw new AiWriteError(t("ai.generate.invalid", { n: invalid + 1 }));
    if (
      destination.kind === "section" &&
      destination.id &&
      !ai.sections.some((section) => section.id === destination.id)
    )
      throw new AiWriteError(
        t("ai.generate.sectionGone", { section: destination.title }),
      );
    if (destination.kind === "group") await ai.addToGroup(destination.id, values);
    else
      await ai.add(values, destination.kind === "section" ? destination.id : undefined);
  }

  function requestFor(kind: BuilderPanel): AiPanelRequest {
    if (kind === "generate")
      return {
        kind,
        context: {
          testId: ai.testId,
          passages: ai.passages,
          ...(ai.section !== null && ai.sectionId
            ? { section: { id: ai.sectionId, title: ai.section } }
            : {}),
          sections: ai.sections,
          destination: { kind: "section", title: ai.section ?? "" },
          save,
        },
      };
    return {
      kind,
      context: {
        testId: ai.testId,
        title: ai.title,
        numberOf: ai.numberOf,
        fix: ai.fix,
        open: ai.open,
      },
    };
  }

  return (
    <>
      <AiEntryButton
        icon={ScanSearch}
        label={t("ai.entry.check")}
        blocked={blocked}
        size="md"
        className={ENTRY}
        onClick={() => void start("check")}
      />
      <AiEntryButton
        icon={Sparkles}
        label={t("ai.entry.generate")}
        blocked={blocked}
        size="md"
        className={ENTRY}
        unavailable={ai.section === null ? t("ai.needsSection") : null}
        onClick={() => void start("generate")}
      />
      {open !== null && (
        <Suspense fallback={null}>
          <AiPanel request={requestFor(open)} onClose={() => setOpen(null)} />
        </Suspense>
      )}
    </>
  );
}
