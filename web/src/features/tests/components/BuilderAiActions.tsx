import { lazy, Suspense, useState } from "react";
import { useTranslation } from "react-i18next";
import { ScanSearch, Sparkles } from "lucide-react";
import type { AiFinding } from "@/features/ai/api";
import type { AiPanelRequest } from "@/features/ai/components/AiPanel";
import { AiEntryButton } from "@/features/ai/components/AiEntryButton";
import { AiWriteError } from "@/features/ai/kind";
import type { AiPassage, GeneratedDraft } from "@/features/ai/kinds/generate";
import { useAiAvailable, useAiBlock } from "@/features/ai/useAiAvailable";
import { draftQuestion } from "@/features/question-bank/aiDrafts";
import {
  questionSchema,
  type QuestionValues,
} from "@/features/question-bank/questionSchema";

const AiPanel = lazy(() => import("@/features/ai/components/AiPanel"));

const ENTRY = "shadow-card h-9";

/**
 * BuilderAi is what the builder hands its AI panels: the open test, its
 * passages, the section new questions join (null when it has none), and the
 * builder's own writes and moves. `prepare` saves every pending edit before a
 * panel opens; `add` creates the kept questions and appends them to that
 * section; `fix` writes a check's fix into its question.
 */
export interface BuilderAi {
  testId: string;
  title: string;
  passages: readonly AiPassage[];
  section: string | null;
  prepare: () => Promise<void>;
  add: (questions: QuestionValues[]) => Promise<void>;
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
  const [open, setOpen] = useState<AiPanelRequest["kind"] | null>(null);

  if (!available) return null;

  async function start(kind: AiPanelRequest["kind"]) {
    try {
      await ai.prepare();
      setOpen(kind);
    } catch (cause) {
      ai.onPrepareFailed(cause);
    }
  }

  async function save(drafts: readonly GeneratedDraft[], tag: string | null) {
    const values = drafts.map((draft) => draftQuestion(draft, tag));
    const invalid = values.findIndex(
      (value) => !questionSchema.safeParse(value).success,
    );
    if (invalid >= 0)
      throw new AiWriteError(t("ai.generate.invalid", { n: invalid + 1 }));
    await ai.add(values);
  }

  function requestFor(kind: AiPanelRequest["kind"]): AiPanelRequest {
    if (kind === "generate")
      return {
        kind,
        context: {
          testId: ai.testId,
          passages: ai.passages,
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
        disabled={ai.section === null}
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
