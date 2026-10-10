import { useMemo, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { CircleCheck, FileCog } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ContentView } from "@/components/shared/content/ContentView";
import { contentPlainText } from "@/components/shared/content/plainText";
import type {
  ImportDraftGroup,
  ImportDraftQuestion,
  ImportDraftSection,
  ImportFinding,
  ImportSourceRef,
} from "../../api";
import { isUnresolved } from "../../findings";
import { FINDING_ACTION, FindingNotice } from "./FindingNotice";
import { ReviewCard } from "./ReviewCard";

const NONE: readonly ImportFinding[] = [];

/** WHOLE_TEST is the selection that opens the whole-test card. */
export const WHOLE_TEST = "whole-test";

/** ExamPaneHandlers are the actions the test pane raises; each is a stable callback. */
export interface ExamPaneHandlers {
  onSelect: (id: string) => void;
  onClose: () => void;
  onAcknowledge: (findingId: string, on: boolean) => void;
  onLocate: (refs: readonly ImportSourceRef[]) => void;
  onReprocess?: ((paper: number) => void) | undefined;
  onPoints: () => void;
}

/** WholeTestFact is one row of the whole-test card's table. */
export interface WholeTestFact {
  label: string;
  value: string;
}

function headlineOf(question: ImportDraftQuestion): string {
  return contentPlainText(question.prompt).split("\n")[0]?.trim() ?? "";
}

/**
 * ExamPane draws the reconstructed test as the deck's cards: the whole-test
 * card first, then each section's heading and its question cards, a shared
 * passage holding its members. Only the selected card is open and renders
 * `editor`; the rest are their header rows. `section` narrows the pane to one
 * section and `visible`, when not null, to the cards a filter holds; the
 * selected card always stays. `allDone` adds the deck's "Everything is
 * resolved" band.
 */
export function ExamPane({
  sections,
  section,
  selectedId,
  findingsByTarget,
  globalFindings,
  currentFindingId,
  visible,
  readOnly,
  facts,
  allDone,
  onFinish,
  handlers,
  editor,
}: Readonly<{
  sections: readonly ImportDraftSection[];
  section: string;
  selectedId: string | null;
  findingsByTarget: ReadonlyMap<string, readonly ImportFinding[]>;
  globalFindings: readonly ImportFinding[];
  currentFindingId: string | null;
  visible: ReadonlySet<string> | null;
  readOnly: boolean;
  facts: readonly WholeTestFact[];
  allDone: boolean;
  onFinish: () => void;
  handlers: ExamPaneHandlers;
  editor: (question: ImportDraftQuestion) => ReactNode;
}>) {
  const { t } = useTranslation();
  const shown = (id: string) =>
    visible === null || visible.has(id) || id === selectedId;
  const toggle = (id: string) =>
    id === selectedId ? handlers.onClose() : handlers.onSelect(id);
  const notice = (finding: ImportFinding, actions?: ReactNode) => (
    <FindingNotice
      key={finding.id}
      finding={finding}
      current={finding.id === currentFindingId}
      readOnly={readOnly}
      onAcknowledge={handlers.onAcknowledge}
      onLocate={handlers.onLocate}
      onReprocess={handlers.onReprocess}
      actions={actions}
    />
  );
  const card = (question: ImportDraftQuestion) => {
    if (!shown(question.id)) return null;
    const findings = findingsByTarget.get(question.id) ?? NONE;
    const excluded = question.excluded !== undefined;
    return (
      <ReviewCard
        key={question.id}
        cardId={question.id}
        headerId={question.id}
        open={question.id === selectedId}
        dimmed={excluded}
        done={!excluded && !findings.some(isUnresolved)}
        badge={question.label}
        headline={headlineOf(question)}
        strike={excluded}
        meta={t(`imports.type.${question.type}`)}
        findings={findings}
        onToggle={() => toggle(question.id)}
      >
        {editor(question)}
      </ReviewCard>
    );
  };

  const pointsFinding = globalFindings.some(
    (finding) => finding.code === "SCORING_DEFAULTED",
  );
  const out: ReactNode[] = [];
  if (shown(WHOLE_TEST))
    out.push(
      <SectionHeading key="whole-heading" title={t("imports.review.wholeTest")} />,
      <ReviewCard
        key={WHOLE_TEST}
        cardId={WHOLE_TEST}
        open={selectedId === WHOLE_TEST}
        done={!globalFindings.some(isUnresolved)}
        badge={<FileCog aria-hidden="true" className="size-3.5" />}
        headline={t("imports.review.wholeTestHeadline")}
        findings={globalFindings}
        onToggle={() => toggle(WHOLE_TEST)}
      >
        <div className="flex flex-col gap-3.5 border-t px-3.5 pt-3 pb-4">
          <span className="text-muted-fg text-xs">
            {t("imports.review.wholeTestScope")}
          </span>
          {globalFindings.map((finding) =>
            notice(
              finding,
              finding.code === "SCORING_DEFAULTED" && !readOnly ? (
                <Button
                  type="button"
                  className={FINDING_ACTION}
                  variant="outline"
                  onClick={handlers.onPoints}
                >
                  {t("imports.review.reviewPoints")}
                </Button>
              ) : null,
            ),
          )}
          <dl className="m-0 flex flex-col overflow-hidden rounded-[10px] border">
            {facts.map((fact) => (
              <div
                key={fact.label}
                className="flex justify-between gap-3 border-t px-3 py-2.25 text-sm first:border-t-0"
              >
                <dt className="text-muted-fg">{fact.label}</dt>
                <dd className="m-0 text-right font-medium [overflow-wrap:anywhere]">
                  {fact.value}
                </dd>
              </div>
            ))}
          </dl>
          {readOnly || pointsFinding ? null : (
            <div>
              <Button
                type="button"
                variant="outline"
                className={FINDING_ACTION}
                onClick={handlers.onPoints}
              >
                {t("imports.review.reviewPoints")}
              </Button>
            </div>
          )}
        </div>
      </ReviewCard>,
    );

  for (const current of sections) {
    if (section !== "all" && section !== current.id) continue;
    const total = current.items.reduce(
      (count, item) =>
        count + (item.question ? 1 : (item.group?.questions.length ?? 0)),
      0,
    );
    const count = current.items.reduce(
      (sum, item) =>
        sum +
        (item.question
          ? Number(shown(item.question.id))
          : (item.group?.questions.filter((question) => shown(question.id)).length ??
            0)),
      0,
    );
    const items = current.items.flatMap((item) => {
      if (item.question) {
        const rendered = card(item.question);
        return rendered === null ? [] : [rendered];
      }
      const group = item.group;
      if (!group) return [];
      const members = group.questions.flatMap((question) => {
        const rendered = card(question);
        return rendered === null ? [] : [rendered];
      });
      const groupNotices = (findingsByTarget.get(group.id) ?? NONE).map((finding) =>
        notice(finding),
      );
      if (members.length === 0 && groupNotices.length === 0) return [];
      return [
        <GroupPanel
          key={group.id}
          group={group}
          selectedId={selectedId}
          notices={groupNotices}
          onSelect={handlers.onSelect}
        >
          {members}
        </GroupPanel>,
      ];
    });
    const sectionNotices = (findingsByTarget.get(current.id) ?? NONE).map((finding) =>
      notice(finding),
    );
    if (visible !== null && items.length === 0 && sectionNotices.length === 0) continue;
    out.push(
      <section key={current.id} aria-label={current.title} className="contents">
        <SectionHeading
          title={current.title}
          meta={
            count === total
              ? t("imports.review.questionsCount", { count: total })
              : t("imports.review.someOf", { count, total })
          }
        />
        {sectionNotices}
        {items}
      </section>,
    );
  }

  return (
    <div className="mx-auto flex w-full max-w-190 flex-col gap-2.5">
      {allDone ? (
        <div className="bg-success-soft text-success-ink flex flex-wrap items-center gap-3 rounded-xl px-3.5 py-3">
          <CircleCheck aria-hidden="true" className="size-4.5 flex-none" />
          <span className="text-ui min-w-0 flex-[1_1_220px] font-medium">
            {t("imports.review.allResolved")}
          </span>
          <Button
            className="h-8 rounded-lg px-3 text-sm font-semibold"
            onClick={onFinish}
          >
            {t("imports.review.finish")}
          </Button>
        </div>
      ) : null}
      {out}
      {out.length === 0 ? (
        <p className="text-muted-fg text-ui px-4 py-10 text-center">
          {t("imports.review.nothingWithFilter")}
        </p>
      ) : null}
    </div>
  );
}

function SectionHeading({ title, meta }: Readonly<{ title: string; meta?: string }>) {
  return (
    <div className="flex items-baseline gap-2 px-0.5 pt-2.5 pb-0.5">
      <h2 className="text-muted-fg m-0 text-xs font-semibold tracking-[0.04em] uppercase">
        {title}
      </h2>
      {meta ? <span className="text-muted-fg text-xs">{meta}</span> : null}
    </div>
  );
}

function GroupPanel({
  group,
  selectedId,
  notices,
  onSelect,
  children,
}: Readonly<{
  group: ImportDraftGroup;
  selectedId: string | null;
  notices: ReactNode;
  onSelect: (questionId: string) => void;
  children: ReactNode;
}>) {
  const { t } = useTranslation();
  const labels = useMemo(
    () => new Map(group.questions.map((question) => [question.id, question.label])),
    [group.questions],
  );
  const links = useMemo(
    () => new Map(group.gaps.map((link) => [link.gapId, link.questionId])),
    [group.gaps],
  );
  const renderGap = (gap: { id: string; label: string }) => {
    const questionId = links.get(gap.id);
    if (questionId === undefined)
      return (
        <span
          className="content-gap"
          role="img"
          aria-label={t("contentEditor.gapLabel", { label: gap.label })}
        >
          {gap.label}
        </span>
      );
    return (
      <button
        type="button"
        aria-pressed={questionId === selectedId}
        aria-label={t("imports.review.gapLink", {
          gap: gap.label,
          question: labels.get(questionId) ?? "",
        })}
        onClick={() => onSelect(questionId)}
        className="content-gap cursor-pointer"
      >
        {gap.label}
      </button>
    );
  };
  return (
    <div className="flex flex-col gap-2.5 rounded-xl border border-dashed p-2.5">
      <div className="flex flex-col gap-1.5 px-1">
        <span className="text-meta font-medium">
          {group.label
            ? t("imports.review.groupLabel", { label: group.label })
            : t("imports.review.group")}
        </span>
        {group.instructions ? (
          <p className="text-muted-fg m-0 text-sm whitespace-pre-wrap">
            {group.instructions}
          </p>
        ) : null}
      </div>
      {notices}
      {group.stimulus === undefined ? null : (
        <div className="bg-card flex flex-col gap-1.5 rounded-[10px] border px-3.5 py-3">
          <span className="text-meta font-medium">
            {t("imports.review.sharedPassage")}
          </span>
          <ContentView
            document={group.stimulus}
            className="text-base leading-[1.8]"
            renderGap={renderGap}
          />
        </div>
      )}
      <div className="flex flex-col gap-2.5">{children}</div>
    </div>
  );
}
