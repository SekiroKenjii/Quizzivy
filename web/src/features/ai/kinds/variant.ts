import { CopyPlus, Save } from "lucide-react";
import { makeTestVariant, type AiVariantChange, type AiVariantKind } from "../api";
import type { AiKind } from "../kind";

/**
 * VariantContext is what Test detail gives the "Make a variant" panel: the
 * test, and `save`, the page's own writes for the kept changes, which make a
 * new draft test and answer its id.
 */
export interface VariantContext {
  testId: string;
  title: string;
  save: (changes: readonly AiVariantChange[]) => Promise<string>;
}

interface VariantValues {
  changes: readonly string[];
  difficulty: string;
}

const CHANGES: readonly AiVariantKind[] = ["reword", "reorder", "distract"];
const DIFFICULTIES = ["same", "easier", "harder"] as const;

function isDifficulty(value: string): value is (typeof DIFFICULTIES)[number] {
  return (DIFFICULTIES as readonly string[]).includes(value);
}

/**
 * variantKind is "Make a variant of this test" (V-12). A variant is a new
 * draft test, never a version of this one (DG-197): its primary asks the page
 * to copy the test and write the kept changes into the copy's questions.
 */
export const variantKind: AiKind<VariantValues, AiVariantChange, VariantContext> = {
  icon: CopyPlus,
  titleKey: "ai.variant.title",
  subtitleKey: "ai.variant.subtitle",
  runLabelKey: "ai.variant.run",
  stepKeys: ["ai.variant.step1", "ai.variant.step2", "ai.variant.step3"],
  emptyKey: "ai.variant.empty",
  initial: () => ({ changes: ["reword", "reorder"], difficulty: "same" }),
  fields: (values, set, _context, t) => [
    {
      kind: "chips",
      label: t("ai.variant.what"),
      options: CHANGES.map((change) => ({
        value: change,
        label: t(`ai.variant.changes.${change}`),
      })),
      value: values.changes,
      onChange: (changes) => set({ changes }),
    },
    {
      kind: "seg",
      label: t("ai.variant.difficulty"),
      options: DIFFICULTIES.map((difficulty) => ({
        value: difficulty,
        label: t(`ai.variant.difficulties.${difficulty}`),
      })),
      value: values.difficulty,
      onChange: (difficulty) => set({ difficulty }),
    },
    { kind: "info", text: t("ai.variant.info") },
  ],
  problem: (values) => (values.changes.length === 0 ? "ai.variant.needChange" : null),
  run: async (values, env, signal) => {
    const result = await makeTestVariant(
      env.context.testId,
      {
        changes: CHANGES.filter((change) => values.changes.includes(change)),
        difficulty: isDifficulty(values.difficulty) ? values.difficulty : "same",
      },
      signal,
    );
    return { items: result.changes, usage: result.usage };
  },
  card: (change, { t }) => ({
    selectable: true,
    tags: [
      { label: t("ai.questionNumber", { n: change.number }), tone: "muted" },
      { label: t(`ai.variant.kinds.${change.kind}`), tone: "info" },
    ],
    title: change.title,
    diff: {
      beforeLabel: t("ai.variant.before"),
      before: change.before,
      afterLabel: t("ai.variant.after"),
      after: change.after,
      strike: true,
    },
  }),
  summary: (items, selected, t) =>
    t("ai.variant.summary", { n: selected, total: items.length }),
  primary: {
    icon: () => Save,
    label: (selected, { t }) => t("ai.variant.save", { count: selected.length }),
    apply: async (selected, { t, context }) => {
      const id = await context.save(selected);
      return {
        message: t("ai.variant.saved"),
        action: { label: t("ai.variant.open"), href: `/teacher/tests/${id}` },
      };
    },
  },
};
