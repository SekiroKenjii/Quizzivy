import { ScanSearch } from "lucide-react";
import { checkTestWithAi, type AiCheckKind, type AiFinding } from "../api";
import type { AiCardAction, AiKind, AiTone } from "../kind";

/**
 * CheckContext is what the builder gives the "Check this test" panel: the
 * test, the number its outline gives a question, and the page's own write of
 * a one-click fix (`updateQuestion`) and its way to open a question.
 */
export interface CheckContext {
  testId: string;
  title: string;
  numberOf: (questionId: string) => number | null;
  fix: (finding: AiFinding) => Promise<void>;
  open: (questionId: string) => void;
}

interface CheckValues {
  checks: readonly string[];
}

const CHECKS: readonly AiCheckKind[] = ["key", "clarity", "giveaway", "language"];

const SEVERITY: Readonly<Record<AiFinding["severity"], AiTone>> = {
  likely_error: "danger",
  worth_a_look: "warning",
  minor: "info",
};

/**
 * checkKind is "Check this test with AI" (V-14b). A fix is one click only
 * when its write exists (an option's text, an accepted answer, the
 * explanation); any other finding offers "Open question" and "Dismiss"
 * (DG-202). It saves nothing as a whole, so its primary is "Done". A finding
 * goes stale when its question has left the test.
 */
export const checkKind: AiKind<CheckValues, AiFinding, CheckContext> = {
  icon: ScanSearch,
  titleKey: "ai.check.title",
  subtitleKey: "ai.check.subtitle",
  runLabelKey: "ai.check.run",
  stepKeys: ["ai.check.step1", "ai.check.step2", "ai.check.step3"],
  emptyKey: "ai.check.empty",
  initial: () => ({ checks: CHECKS }),
  fields: (values, set, context, t) => [
    {
      kind: "chips",
      label: t("ai.check.lookFor"),
      options: CHECKS.map((check) => ({
        value: check,
        label: t(`ai.check.kinds.${check}`),
      })),
      value: values.checks,
      onChange: (checks) => set({ checks }),
    },
    { kind: "info", text: t("ai.check.info", { title: context.title }) },
  ],
  problem: (values) => (values.checks.length === 0 ? "ai.check.needCheck" : null),
  run: async (values, env, signal) => {
    const result = await checkTestWithAi(
      env.context.testId,
      {
        checks: CHECKS.filter((check) => values.checks.includes(check)),
        language: env.language,
      },
      signal,
    );
    return { items: result.findings, usage: result.usage };
  },
  card: (finding, { t, context, close }) => {
    const number = context.numberOf(finding.questionId) ?? finding.number;
    const actions: AiCardAction[] = [];
    if (finding.fix.kind !== "none")
      actions.push({
        label: finding.fix.label ?? t("ai.check.applyFix"),
        primary: true,
        drop: true,
        run: async () => {
          await context.fix(finding);
          return t("ai.check.fixed", { n: number });
        },
      });
    actions.push(
      {
        label: t("ai.check.openQuestion"),
        run: () => {
          close();
          context.open(finding.questionId);
          return null;
        },
      },
      { label: t("ai.check.dismiss"), drop: true },
    );
    return {
      selectable: false,
      tags: [
        {
          label: t(`ai.check.severity.${finding.severity}`),
          tone: SEVERITY[finding.severity],
        },
        { label: t("ai.questionNumber", { n: number }), tone: "muted" },
      ],
      meta: t(`questionEditor.type.${finding.type}`),
      title: finding.title,
      body: finding.issue,
      note: { label: t("ai.check.suggestedFix"), text: finding.fix.description },
      actions,
    };
  },
  stale: (finding, context) => context.numberOf(finding.questionId) === null,
  summary: (items, _selected, t) =>
    items.length === 0 ? "" : t("ai.check.summary", { count: items.length }),
  primary: null,
};
