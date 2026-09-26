import { useMemo, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { ChevronDown, ChevronRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ContentView } from "@/components/shared/content/ContentView";
import type {
  ImportDraftGroup,
  ImportDraftQuestion,
  ImportDraftSection,
  ImportFinding,
  ImportSourceRef,
} from "../../api";
import { FindingNotice } from "./FindingNotice";
import { QuestionCard } from "./QuestionCard";

const NONE: readonly ImportFinding[] = [];

/** ExamPaneHandlers are the actions the reconstructed exam raises; each is a stable callback. */
export interface ExamPaneHandlers {
  onSelect: (questionId: string) => void;
  onAcknowledge: (findingId: string, on: boolean) => void;
  onLocate: (refs: readonly ImportSourceRef[]) => void;
  onReprocess?: ((paper: number) => void) | undefined;
}

/**
 * ExamPane draws the reconstructed exam: sections, shared passages with their
 * gaps, and one card per question. Only the selected question is rendered by
 * `editor`; the others are read-only cards. While `readOnly`, notices offer no
 * decisions.
 */
export function ExamPane({
  sections,
  selectedId,
  findingsByTarget,
  globalFindings,
  informational,
  currentFindingId,
  visible,
  readOnly,
  handlers,
  editor,
}: Readonly<{
  sections: readonly ImportDraftSection[];
  selectedId: string | null;
  findingsByTarget: ReadonlyMap<string, readonly ImportFinding[]>;
  globalFindings: readonly ImportFinding[];
  informational: readonly ImportFinding[];
  currentFindingId: string | null;
  visible: ReadonlySet<string> | null;
  readOnly: boolean;
  handlers: ExamPaneHandlers;
  editor: (question: ImportDraftQuestion) => ReactNode;
}>) {
  const { t } = useTranslation();
  const card = (question: ImportDraftQuestion) => {
    if (visible !== null && !visible.has(question.id) && question.id !== selectedId)
      return null;
    if (question.id === selectedId)
      return <div key={question.id}>{editor(question)}</div>;
    return (
      <QuestionCard
        key={question.id}
        question={question}
        findings={findingsByTarget.get(question.id) ?? NONE}
        onSelect={handlers.onSelect}
      />
    );
  };
  const notices = (id: string) =>
    (findingsByTarget.get(id) ?? NONE).map((finding) => (
      <FindingNotice
        key={finding.id}
        finding={finding}
        current={finding.id === currentFindingId}
        readOnly={readOnly}
        onAcknowledge={handlers.onAcknowledge}
        onLocate={handlers.onLocate}
        onReprocess={handlers.onReprocess}
      />
    ));

  return (
    <div className="mx-auto max-w-3xl space-y-8">
      {globalFindings.length > 0 ? (
        <section aria-labelledby="import-global-findings" className="space-y-2">
          <h2 id="import-global-findings" className="text-sm font-medium">
            {t("imports.review.wholeImport")}
          </h2>
          {globalFindings.map((finding) => (
            <FindingNotice
              key={finding.id}
              finding={finding}
              current={finding.id === currentFindingId}
              readOnly={readOnly}
              onAcknowledge={handlers.onAcknowledge}
              onLocate={handlers.onLocate}
              onReprocess={handlers.onReprocess}
            />
          ))}
        </section>
      ) : null}
      {sections.map((section) => {
        const items = section.items.flatMap((item) => {
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
          const groupNotices = notices(group.id);
          if (members.length === 0 && groupNotices.length === 0) return [];
          return [
            <GroupBlock
              key={group.id}
              group={group}
              selectedId={selectedId}
              notices={groupNotices}
              onSelect={handlers.onSelect}
            >
              {members}
            </GroupBlock>,
          ];
        });
        const sectionNotices = notices(section.id);
        if (visible !== null && items.length === 0 && sectionNotices.length === 0)
          return null;
        return (
          <section key={section.id} aria-label={section.title} className="space-y-3">
            <header className="space-y-1 border-b pb-2">
              <h2 className="text-base font-semibold">
                {section.title === ""
                  ? t("imports.review.untitledSection")
                  : section.title}
              </h2>
              {section.instructions ? (
                <p className="text-muted-foreground text-sm whitespace-pre-wrap">
                  {section.instructions}
                </p>
              ) : null}
            </header>
            {sectionNotices}
            {items}
          </section>
        );
      })}
      <ProcessingDetails findings={informational} onLocate={handlers.onLocate} />
    </div>
  );
}

function GroupBlock({
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
        className="content-gap focus-visible:ring-ring cursor-pointer focus-visible:ring-2 focus-visible:outline-none"
      >
        {gap.label}
      </button>
    );
  };
  return (
    <div className="space-y-3 rounded-lg border border-dashed p-4">
      <header className="space-y-1">
        <h3 className="text-sm font-medium">
          {group.label
            ? t("imports.review.groupLabel", { label: group.label })
            : t("imports.review.group")}
        </h3>
        {group.instructions ? (
          <p className="text-muted-foreground text-sm whitespace-pre-wrap">
            {group.instructions}
          </p>
        ) : null}
      </header>
      {notices}
      {group.stimulus === undefined ? null : (
        <ContentView
          document={group.stimulus}
          className="bg-muted/30 rounded-md p-3 text-sm"
          renderGap={renderGap}
        />
      )}
      <div className="space-y-3">{children}</div>
    </div>
  );
}

function ProcessingDetails({
  findings,
  onLocate,
}: Readonly<{
  findings: readonly ImportFinding[];
  onLocate: (refs: readonly ImportSourceRef[]) => void;
}>) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  if (findings.length === 0) return null;
  const Icon = open ? ChevronDown : ChevronRight;
  return (
    <section className="space-y-2 border-t pt-4">
      <Button
        type="button"
        variant="ghost"
        size="sm"
        aria-expanded={open}
        aria-controls="import-processing-details"
        onClick={() => setOpen((value) => !value)}
      >
        <Icon aria-hidden="true" />
        {t("imports.review.processingDetails", { count: findings.length })}
      </Button>
      {open ? (
        <div id="import-processing-details" className="space-y-2">
          {findings.map((finding) => (
            <FindingNotice
              key={finding.id}
              finding={finding}
              current={false}
              onAcknowledge={noop}
              onLocate={onLocate}
            />
          ))}
        </div>
      ) : null}
    </section>
  );
}

function noop() {}
