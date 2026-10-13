import { lazy, Suspense, useId, useState } from "react";
import { Trans, useTranslation } from "react-i18next";
import { Link } from "react-router";
import { useQueryClient } from "@tanstack/react-query";
import { KeyRound, Sparkles, TrendingUp, TriangleAlert, X } from "lucide-react";
import { contentPlainText } from "@/components/shared/content/plainText";
import { Callout } from "@/components/shared/Callout";
import { IconTile } from "@/components/shared/IconTile";
import { Button } from "@/components/ui/button";
import type { AiInsight, AiInsightsResult } from "@/features/ai/api";
import { AiEntryButton } from "@/features/ai/components/AiEntryButton";
import { AiWriteError } from "@/features/ai/kind";
import type { AiPassage, GeneratedDraft } from "@/features/ai/kinds/generate";
import {
  AI_BLOCK_KEYS,
  useAiAvailable,
  useAiBlock,
} from "@/features/ai/useAiAvailable";
import { aiPreferencesOf } from "@/features/auth/preferences";
import { draftQuestion } from "@/features/question-bank/aiDrafts";
import { questionSchema } from "@/features/question-bank/questionSchema";
import { savePractice } from "@/features/tests/aiPractice";
import type { AssignmentInsights, InsightsFailure } from "../assignmentInsights";
import type { components } from "@/lib/api/schema";
import { useAuthStore } from "@/stores/auth";

const AiPanel = lazy(() => import("@/features/ai/components/AiPanel"));

const TITLE_MAX = 200;

type QuestionType = AiInsight["types"][number];
type StudentGroup = components["schemas"]["StudentGroup"];

const KINDS = {
  key: { icon: KeyRound, tone: "warning" },
  problem: { icon: TriangleAlert, tone: "danger" },
  strength: { icon: TrendingUp, tone: "success" },
} as const;

function useReadsStudentAnswers(): boolean {
  return useAuthStore(
    (s) => aiPreferencesOf(s.user?.preferences?.ai, "vi").readStudentAnswers,
  );
}

/**
 * AiInsightsAction is the Questions tab's "Explain results with AI" (V-06):
 * "Reading results…" with a spinner while it reads and "Refresh" once it
 * has. It is not drawn without the assistant or `ai.use`; while the
 * teacher's "Let AI read student answers" is off it is a line that says so,
 * with a link to Settings › AI assistant; it is disabled with the reason
 * while the teacher is paused or out of credits, or nothing is handed in.
 */
export function AiInsightsAction({
  insights,
  handedIn,
}: Readonly<{ insights: AssignmentInsights; handedIn: number | null }>) {
  const { t } = useTranslation();
  const available = useAiAvailable();
  const blocked = useAiBlock(available);
  const reads = useReadsStudentAnswers();
  if (!available) return null;
  if (!reads)
    return (
      <p className="text-muted-fg text-meta min-w-0 flex-[0_1_auto]">
        <ReadOffNote />
      </p>
    );
  let label = t("assignmentDetail.insights.explain");
  if (insights.running) label = t("assignmentDetail.insights.reading");
  else if (insights.result) label = t("assignmentDetail.insights.refresh");
  return (
    <AiEntryButton
      id={insights.openerId}
      icon={Sparkles}
      label={label}
      blocked={blocked}
      unavailable={handedIn === 0 ? t("assignmentDetail.analysis.empty") : null}
      busy={insights.running}
      className="shadow-card h-8 gap-1.5 rounded-[0.5rem] px-3 text-sm in-data-[scale=deck]:px-3 in-data-[scale=deck]:text-sm in-data-[scale=deck]:[&_svg:not([class*='size-'])]:size-3.5"
      onClick={insights.run}
    />
  );
}

function ReadOffNote() {
  return (
    <Trans
      i18nKey="assignmentDetail.insights.readOff"
      components={{
        settings: (
          <Link
            to="/teacher/settings/ai"
            className="text-fg font-medium underline underline-offset-2"
          />
        ),
      }}
    />
  );
}

function passagesOf(groups: readonly StudentGroup[]): AiPassage[] {
  return groups.flatMap((group) =>
    group.stimuli.flatMap((stimulus) => {
      const text = contentPlainText(stimulus.content).trim();
      return text === "" ? [] : [{ id: stimulus.id, title: stimulus.title, text }];
    }),
  );
}

/**
 * AiInsightsPanel is what "Explain results with AI" answered (V-06): "What AI
 * noticed", the number of papers it read, "Hide insights", and up to four
 * cards, each a pattern with its icon tile and, where the assistant suggests
 * one, "Review answers" (Grading narrowed to the assignment) or "Make a
 * practice set" (the Generate panel with the insight's types, saving a new
 * draft test "{title} · Practice"). It writes nothing itself; the practice
 * set is the one write, made by its primary through the existing writes. A
 * failed explanation is a callout in its place, with "Try again" when the
 * assistant did not answer.
 */
export function AiInsightsPanel({
  insights,
  assignmentId,
  testTitle,
  groups,
}: Readonly<{
  insights: AssignmentInsights;
  assignmentId: string;
  testTitle: string;
  groups: readonly StudentGroup[] | undefined;
}>) {
  const { t } = useTranslation();
  const available = useAiAvailable();
  const reads = useReadsStudentAnswers();
  const [practice, setPractice] = useState<readonly QuestionType[] | null>(null);
  if (!available || !reads) return null;
  return (
    <>
      <p role="status" className="sr-only">
        {insights.running ? t("assignmentDetail.insights.reading") : ""}
      </p>
      {insights.failure !== null && (
        <InsightsFailureNote failure={insights.failure} onRetry={insights.run} />
      )}
      {insights.result !== null && (
        <InsightsResult
          result={insights.result}
          gradingHref={`/teacher/grading?${new URLSearchParams({ assignment: assignmentId }).toString()}`}
          onHide={insights.hide}
          onPractice={setPractice}
        />
      )}
      {practice !== null && (
        <PracticePanel
          types={practice}
          testTitle={testTitle}
          passages={passagesOf(groups ?? [])}
          onClose={() => setPractice(null)}
        />
      )}
    </>
  );
}

function InsightsFailureNote({
  failure,
  onRetry,
}: Readonly<{ failure: InsightsFailure; onRetry: () => void }>) {
  const { t } = useTranslation();
  const box = "mx-4 mb-3.5";
  switch (failure.kind) {
    case "unavailable":
      return (
        <Callout
          tone="warning"
          size="md"
          announce
          className={box}
          actions={
            <Button variant="outline" size="sm" className="bg-card" onClick={onRetry}>
              {t("ai.tryAgain")}
            </Button>
          }
        >
          {t("ai.unavailable")}
        </Callout>
      );
    case "blocked":
      return (
        <Callout tone="warning" size="md" announce className={box}>
          {t(AI_BLOCK_KEYS[failure.block])}
        </Callout>
      );
    case "readOff":
      return (
        <Callout tone="warning" size="md" announce className={box}>
          <ReadOffNote />
        </Callout>
      );
    case "nothingHandedIn":
      return (
        <Callout tone="info" size="md" announce className={box}>
          {t("assignmentDetail.analysis.empty")}
        </Callout>
      );
    case "error":
      return (
        <Callout tone="danger" size="md" announce className={box}>
          {failure.message}
        </Callout>
      );
  }
}

function InsightsResult({
  result,
  gradingHref,
  onHide,
  onPractice,
}: Readonly<{
  result: AiInsightsResult;
  gradingHref: string;
  onHide: () => void;
  onPractice: (types: readonly QuestionType[]) => void;
}>) {
  const { t } = useTranslation();
  const headingId = useId();
  return (
    <section
      aria-labelledby={headingId}
      data-slot="ai-insights"
      className="bg-brand-soft mx-4 mb-3.5 flex flex-col gap-2.5 rounded-[0.625rem] border p-3.5"
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3
          id={headingId}
          className="text-brand-ink text-meta flex items-center gap-1.5 leading-normal font-semibold"
        >
          <Sparkles aria-hidden="true" className="size-[0.8125rem]" />
          {t("assignmentDetail.insights.title")}
        </h3>
        <span className="flex items-center gap-1.5">
          <span className="text-muted-fg text-xs leading-normal">
            {t("assignmentDetail.insights.basis", { count: result.basis })}
          </span>
          <button
            type="button"
            aria-label={t("assignmentDetail.insights.hide")}
            className="text-muted-fg hover:bg-card hover:text-fg grid size-6.5 place-items-center rounded-[0.375rem]"
            onClick={onHide}
          >
            <X aria-hidden="true" className="size-3.5" />
          </button>
        </span>
      </div>
      {result.items.length === 0 ? (
        <p className="text-muted-fg text-sm">{t("assignmentDetail.insights.empty")}</p>
      ) : (
        <ul className="flex flex-col gap-2.5">
          {result.items.map((insight, index) => (
            <InsightCard
              key={index}
              insight={insight}
              gradingHref={gradingHref}
              onPractice={onPractice}
            />
          ))}
        </ul>
      )}
    </section>
  );
}

const ACTION =
  "text-meta h-7.5 gap-1.5 rounded-seg px-2.5 font-medium shadow-none in-data-[scale=deck]:px-2.5 in-data-[scale=deck]:text-meta";

function InsightCard({
  insight,
  gradingHref,
  onPractice,
}: Readonly<{
  insight: AiInsight;
  gradingHref: string;
  onPractice: (types: readonly QuestionType[]) => void;
}>) {
  const { t } = useTranslation();
  const kind = KINDS[insight.kind];
  return (
    <li className="bg-card rounded-ctl flex flex-wrap items-start gap-3 border p-3">
      <IconTile
        icon={kind.icon}
        size={28}
        tone={kind.tone}
        className="[&>svg]:size-3.5"
      />
      <span className="min-w-0 flex-[1_1_240px]">
        <span className="text-ui block leading-[1.4] font-semibold [overflow-wrap:anywhere]">
          {insight.title}
        </span>
        <span className="text-muted-fg mt-0.5 block text-sm leading-[1.55] text-pretty [overflow-wrap:anywhere]">
          {insight.body}
        </span>
      </span>
      {insight.action === "review" && (
        <Button variant="outline" className={ACTION} asChild>
          <Link to={gradingHref}>{t("assignmentDetail.insights.review")}</Link>
        </Button>
      )}
      {insight.action === "practice" && (
        <Button
          variant="outline"
          className={ACTION}
          onClick={() => onPractice(insight.types)}
        >
          {t("assignmentDetail.insights.practice")}
        </Button>
      )}
    </li>
  );
}

function PracticePanel({
  types,
  testTitle,
  passages,
  onClose,
}: Readonly<{
  types: readonly QuestionType[];
  testTitle: string;
  passages: readonly AiPassage[];
  onClose: () => void;
}>) {
  const { t } = useTranslation();
  const client = useQueryClient();

  function practiceTitle() {
    const full = t("assignmentDetail.insights.practiceTitle", { title: testTitle });
    if (full.length <= TITLE_MAX) return full;
    const room = TITLE_MAX - (full.length - testTitle.length);
    return t("assignmentDetail.insights.practiceTitle", {
      title: testTitle.slice(0, room).trimEnd(),
    });
  }

  async function save(drafts: readonly GeneratedDraft[], tag: string | null) {
    const values = drafts.map((draft) => draftQuestion(draft, tag));
    const invalid = values.findIndex(
      (value) => !questionSchema.safeParse(value).success,
    );
    if (invalid >= 0)
      throw new AiWriteError(t("ai.generate.invalid", { n: invalid + 1 }));
    try {
      await savePractice(values, {
        title: practiceTitle(),
        section: t("builder.newSection", { n: 1 }),
      });
    } finally {
      await Promise.all([
        client.invalidateQueries({ queryKey: ["teacher-tests"] }),
        client.invalidateQueries({ queryKey: ["teacher-questions"] }),
      ]);
    }
  }

  return (
    <Suspense fallback={null}>
      <AiPanel
        request={{
          kind: "generate",
          context: { passages, types, destination: { kind: "practice" }, save },
        }}
        onClose={onClose}
      />
    </Suspense>
  );
}
