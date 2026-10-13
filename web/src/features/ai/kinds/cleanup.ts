import { Tag, WandSparkles } from "lucide-react";
import {
  cleanUpQuestionBank,
  type AiCleanupCheck,
  type AiDuplicate,
  type AiTagSuggestion,
} from "../api";
import type { AiCard, AiKind } from "../kind";

/**
 * CleanupContext is what the question bank gives the "Clean up" panel:
 * where a question opens, and `tag`, the page's own bulk tag write for the
 * kept suggestions.
 */
export interface CleanupContext {
  href: (questionId: string) => string;
  tag: (suggestions: readonly AiTagSuggestion[]) => Promise<void>;
}

interface CleanupValues {
  checks: readonly string[];
}

type CleanupItem =
  | { kind: "duplicate"; duplicate: AiDuplicate }
  | { kind: "tags"; suggestion: AiTagSuggestion };

const CHECKS: readonly AiCleanupCheck[] = ["duplicates", "tags"];

function suggestions(items: readonly CleanupItem[]): AiTagSuggestion[] {
  return items.flatMap((item) => (item.kind === "tags" ? [item.suggestion] : []));
}

/**
 * cleanupKind is "Clean up the question bank" (V-14c). A near-duplicate is
 * shown with "Open" for each question and "Keep both", and nothing merges
 * (DG-197); kept tag suggestions go through the page's bulk tag write. One
 * call reads a bounded part of the bank, and its `truncated` says so.
 */
export const cleanupKind: AiKind<CleanupValues, CleanupItem, CleanupContext> = {
  icon: WandSparkles,
  titleKey: "ai.cleanup.title",
  subtitleKey: "ai.cleanup.subtitle",
  runLabelKey: "ai.cleanup.run",
  stepKeys: ["ai.cleanup.step1", "ai.cleanup.step2", "ai.cleanup.step3"],
  emptyKey: "ai.cleanup.empty",
  initial: () => ({ checks: CHECKS }),
  fields: (values, set, _context, t) => [
    {
      kind: "chips",
      label: t("ai.cleanup.lookFor"),
      options: CHECKS.map((check) => ({
        value: check,
        label: t(`ai.cleanup.checks.${check}`),
      })),
      value: values.checks,
      onChange: (checks) => set({ checks }),
    },
    { kind: "info", text: t("ai.cleanup.info") },
  ],
  problem: (values) => (values.checks.length === 0 ? "ai.cleanup.needCheck" : null),
  run: async (values, env, signal) => {
    const result = await cleanUpQuestionBank(
      {
        checks: CHECKS.filter((check) => values.checks.includes(check)),
        language: env.language,
      },
      signal,
    );
    return {
      items: [
        ...result.duplicates.map((duplicate) => ({
          kind: "duplicate" as const,
          duplicate,
        })),
        ...result.tags.map((suggestion) => ({ kind: "tags" as const, suggestion })),
      ],
      usage: result.usage,
      ...(result.truncated
        ? { notice: env.t("ai.cleanup.truncated", { count: result.scanned }) }
        : {}),
    };
  },
  card: (item, { t, context }): AiCard => {
    if (item.kind === "tags") {
      const { suggestion } = item;
      return {
        selectable: true,
        tags: [
          { label: t("ai.cleanup.missingTags"), tone: "info" },
          ...(suggestion.level
            ? [{ label: t(`bank.level.${suggestion.level}`), tone: "muted" as const }]
            : []),
        ],
        title: suggestion.prompt,
        chips: suggestion.tags,
      };
    }
    const { a, b, why } = item.duplicate;
    return {
      selectable: false,
      tags: [{ label: t("ai.cleanup.duplicate"), tone: "warning" }],
      meta: t("ai.cleanup.usedIn", {
        a: t("ai.cleanup.testCount", { count: a.usedInTests }),
        b: t("ai.cleanup.testCount", { count: b.usedInTests }),
      }),
      title: why,
      diff: {
        beforeLabel: t("ai.cleanup.one"),
        before: a.prompt,
        afterLabel: t("ai.cleanup.other"),
        after: b.prompt,
        strike: false,
      },
      actions: [
        { label: t("ai.cleanup.openOne"), href: context.href(a.questionId) },
        { label: t("ai.cleanup.openOther"), href: context.href(b.questionId) },
        { label: t("ai.cleanup.keepBoth"), drop: true },
      ],
    };
  },
  summary: (items, _selected, t) => {
    const duplicates = items.filter((item) => item.kind === "duplicate").length;
    const tagged = items.length - duplicates;
    return [
      duplicates > 0 ? t("ai.cleanup.duplicates", { count: duplicates }) : "",
      tagged > 0 ? t("ai.cleanup.untagged", { count: tagged }) : "",
    ]
      .filter(Boolean)
      .join(" · ");
  },
  primary: {
    icon: () => Tag,
    available: (items) => suggestions(items).length > 0,
    label: (selected, { t }) =>
      t("ai.cleanup.addTags", { count: suggestions(selected).length }),
    apply: async (selected, { t, context }) => {
      const kept = suggestions(selected);
      await context.tag(kept);
      return t("ai.cleanup.tagged", { count: kept.length });
    },
  },
};
