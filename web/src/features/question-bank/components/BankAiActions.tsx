import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";
import { Sparkles, WandSparkles } from "lucide-react";
import type { AiTagSuggestion } from "@/features/ai/api";
import type { AiPanelRequest } from "@/features/ai/components/AiPanel";
import { AiEntryButton } from "@/features/ai/components/AiEntryButton";
import { AiPanelStack } from "@/features/ai/components/AiPanelStack";
import { AiWriteError } from "@/features/ai/kind";
import type { GeneratedDraft } from "@/features/ai/kinds/generate";
import { useAiAvailable, useAiBlock } from "@/features/ai/useAiAvailable";
import { createQuestion, tagQuestions } from "@/features/question-bank/api";
import { draftQuestion } from "@/features/question-bank/aiDrafts";
import { questionSchema } from "@/features/question-bank/questionSchema";

const ENTRY = "shadow-card h-9";

type BankPanel = "cleanup" | "generate";

function byTags(suggestions: readonly AiTagSuggestion[]) {
  const groups = new Map<string, { tags: string[]; ids: string[] }>();
  for (const suggestion of suggestions) {
    const tags = [...new Set(suggestion.tags)].sort((a, b) => a.localeCompare(b));
    const key = JSON.stringify(tags);
    const group = groups.get(key) ?? { tags, ids: [] };
    group.ids.push(suggestion.questionId);
    groups.set(key, group);
  }
  return [...groups.values()];
}

/**
 * BankAiActions are the question bank header's "Clean up" and "Generate"
 * (V-08) and the panel each opens. Neither is drawn without the assistant or
 * `ai.use`; both are drawn disabled with the reason while the teacher is
 * paused or out of credits (Q34). Kept tag suggestions go through the bulk
 * tag write, one call for each set of tags, and kept questions through the
 * bank's create. Closing a panel keeps its session while the bank page is open.
 */
export function BankAiActions() {
  const { t } = useTranslation();
  const client = useQueryClient();
  const available = useAiAvailable();
  const blocked = useAiBlock(available);
  const [open, setOpen] = useState<BankPanel | null>(null);

  if (!available) return null;

  async function tag(suggestions: readonly AiTagSuggestion[]) {
    try {
      for (const group of byTags(suggestions))
        await tagQuestions(group.ids, group.tags);
    } finally {
      await client.invalidateQueries({ queryKey: ["teacher-questions"] });
    }
  }

  async function save(drafts: readonly GeneratedDraft[], label: string | null) {
    const values = drafts.map((draft) => draftQuestion(draft, label));
    const invalid = values.findIndex(
      (value) => !questionSchema.safeParse(value).success,
    );
    if (invalid >= 0)
      throw new AiWriteError(t("ai.generate.invalid", { n: invalid + 1 }));
    try {
      for (const value of values) await createQuestion(value);
    } finally {
      await client.invalidateQueries({ queryKey: ["teacher-questions"] });
    }
  }

  function requestFor(kind: BankPanel): AiPanelRequest {
    if (kind === "cleanup")
      return {
        kind,
        context: { href: (id) => `/teacher/question-bank/${id}`, tag },
      };
    return { kind, context: { passages: [], destination: { kind: "bank" }, save } };
  }

  return (
    <>
      <AiEntryButton
        icon={WandSparkles}
        label={t("ai.entry.cleanup")}
        blocked={blocked}
        className={ENTRY}
        onClick={() => setOpen("cleanup")}
      />
      <AiEntryButton
        icon={Sparkles}
        label={t("ai.entry.generate")}
        blocked={blocked}
        className={ENTRY}
        onClick={() => setOpen("generate")}
      />
      <AiPanelStack open={open} requestFor={requestFor} onClose={() => setOpen(null)} />
    </>
  );
}
