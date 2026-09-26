import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { useBlocker, useNavigate, useSearchParams } from "react-router";
import { useMutation, useQueries } from "@tanstack/react-query";
import { ArrowLeft, ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Segmented } from "@/components/ui/segmented";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { SideColumn } from "@/components/shared/SideColumn";
import { AutosaveStatusLabel } from "@/features/tests/components/AutosaveStatusLabel";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { failureMessage } from "@/lib/api/errors";
import {
  adoptWordImportReprocessed,
  type ImportDraftQuestion,
  type ImportDraftSection,
  type ImportFinding,
  type ImportReview,
  type ImportSourceRef,
  type ImportSourceRole,
  type WordImport,
} from "../../api";
import { useImportAvailability } from "../../availability";
import { blockKey, blockOwners, draftPositions, questionPlaces } from "../../draft";
import {
  findingRank,
  isFindingFilter,
  isUnresolved,
  matchesFilter,
  orderFindings,
  stepFinding,
  type FindingAnchor,
  type FindingFilter,
} from "../../findings";
import { sourceViewQuery } from "../../queries";
import { isActiveStatus, reprocessOutcome } from "../../status";
import { useImportCommit } from "../../useImportCommit";
import { useReprocess } from "../../useReprocess";
import { useReviewSession } from "../../useReviewSession";
import { ExamPane, type ExamPaneHandlers } from "./ExamPane";
import { QuestionEditor } from "./QuestionEditor";
import { PhoneNotice, ReviewBanners } from "./ReviewBanners";
import { ReviewSummaryDialog } from "./ReviewSummaryDialog";
import { SourcePane, type SourceFocus } from "./SourcePane";

const NONE: readonly ImportFinding[] = [];

interface Structure {
  sourceRefs: Map<string, ImportSourceRef[]>;
  groupFirst: Map<string, string>;
  groupMembers: Map<string, string[]>;
  owners: Map<string, string>;
  positions: Map<string, number>;
}

interface Target {
  id: string;
  nonce: number;
  focus: boolean;
}

function structureOf(sections: readonly ImportDraftSection[]): Structure {
  const places = questionPlaces(sections);
  const groupMembers = new Map<string, string[]>();
  for (const place of places) {
    if (place.groupId !== null)
      groupMembers.set(place.groupId, [
        ...(groupMembers.get(place.groupId) ?? []),
        place.question.id,
      ]);
  }
  return {
    sourceRefs: new Map(
      places.map((place) => [place.question.id, place.question.source]),
    ),
    groupFirst: new Map(
      [...groupMembers].flatMap(([group, members]) =>
        members[0] === undefined ? [] : [[group, members[0]] as const],
      ),
    ),
    groupMembers,
    owners: blockOwners(sections),
    positions: draftPositions(sections),
  };
}

function questionFor(finding: ImportFinding, structure: Structure): string | null {
  const target = finding.target;
  if (target === undefined) return null;
  if (structure.sourceRefs.has(target)) return target;
  return structure.groupFirst.get(target) ?? null;
}

function firstSelection(review: ImportReview, structure: Structure): string | null {
  const ordered = orderFindings(
    review.findings.filter((finding) => isUnresolved(finding)),
    structure.positions,
  );
  for (const finding of ordered) {
    const questionId = questionFor(finding, structure);
    if (questionId !== null) return questionId;
  }
  return questionPlaces(review.draft.sections)[0]?.question.id ?? null;
}

function nextTarget(id: string, focus: boolean) {
  return (previous: Target | null): Target => ({
    id,
    focus,
    nonce: (previous?.nonce ?? 0) + 1,
  });
}

function noop() {}

/**
 * ReviewWorkspace is the linked review of one import: the source and the
 * reconstructed exam side by side, finding navigation above them, and the
 * summary that creates the draft test. It edits a copy of `initial` taken at
 * mount and is read-only whenever the import is not under review, the draft
 * changed elsewhere, processing finished after the page opened, or a commit
 * or reprocess is under way.
 */
export function ReviewWorkspace({
  value,
  initial,
  onReload,
  onAdopted,
}: Readonly<{
  value: WordImport;
  initial: ImportReview;
  onReload: () => void;
  onAdopted: (adopted: ImportReview) => void;
}>) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const importId = value.id;
  const [origin] = useState(initial);
  const locked = useRef(false);
  const session = useReviewSession(importId, origin, locked);
  const { working, server, editQuestion, setTitle, acknowledge } = session;
  const { status, flush, retry } = session.autosave;
  const { state: commitState, commit } = useImportCommit(
    importId,
    flush,
    session.revision,
  );
  const {
    reprocess,
    pending: reprocessPending,
    error: reprocessError,
  } = useReprocess(importId, flush);
  const processing = useImportAvailability() !== "reviewOnly";
  const wide = useMediaQuery("(min-width: 1280px)");
  const phone = useMediaQuery("(max-width: 767px)");
  const [params, setParams] = useSearchParams();
  const requestedFilter = params.get("filter");
  const filter: FindingFilter = isFindingFilter(requestedFilter)
    ? requestedFilter
    : "all";

  const structure = useMemo(() => structureOf(origin.draft.sections), [origin]);
  const [selectedId, setSelectedId] = useState<string | null>(() =>
    firstSelection(origin, structure),
  );
  const [sourceRole, setSourceRole] = useState<ImportSourceRole>("exam");
  const [focus, setFocus] = useState<SourceFocus>(() => ({
    refs: selectedId === null ? [] : (structure.sourceRefs.get(selectedId) ?? []),
    nonce: 0,
    take: false,
  }));
  const [anchor, setAnchor] = useState<FindingAnchor | null>(null);
  const [findingTarget, setFindingTarget] = useState<Target | null>(null);
  const [examTarget, setExamTarget] = useState<Target | null>(null);
  const [view, setView] = useState<"source" | "exam">("exam");
  const [summaryOpen, setSummaryOpen] = useState(false);
  const [leaveFailed, setLeaveFailed] = useState(false);
  const [adopting, setAdopting] = useState(false);
  const [confirmReload, setConfirmReload] = useState(false);
  const finishButton = useRef<HTMLButtonElement>(null);
  const afterSummary = useRef<string | null>(null);
  const examScroller = useRef<HTMLDivElement>(null);
  const examScrollTop = useRef(0);

  const stale = status.kind === "stale";
  const unsaved =
    status.kind === "dirty" ||
    status.kind === "saving" ||
    status.kind === "failed" ||
    stale;
  const committed = value.status === "committed" || commitState.phase === "done";
  const underReview = value.status === "needs_review";
  const [openedWhile] = useState(value.status);
  const finished =
    isActiveStatus(openedWhile) && underReview && reprocessOutcome(value) === null;
  const readOnly =
    stale ||
    committed ||
    finished ||
    !underReview ||
    commitState.phase === "pending" ||
    commitState.phase === "lost" ||
    reprocessPending;

  useEffect(() => {
    locked.current = readOnly;
  }, [readOnly]);

  const setFilter = useCallback(
    (next: FindingFilter) =>
      setParams(
        (previous) => {
          const out = new URLSearchParams(previous);
          if (next === "all") out.delete("filter");
          else out.set("filter", next);
          return out;
        },
        { replace: true },
      ),
    [setParams],
  );

  const acknowledged = useMemo(
    () => new Set(working.acknowledged),
    [working.acknowledged],
  );
  const findings = useMemo(
    () =>
      server.findings.map((finding) =>
        finding.severity === "review_required"
          ? { ...finding, acknowledged: acknowledged.has(finding.id) }
          : finding,
      ),
    [server.findings, acknowledged],
  );
  const grouped = useMemo(() => {
    const byTarget = new Map<string, ImportFinding[]>();
    const global: ImportFinding[] = [];
    const informational: ImportFinding[] = [];
    for (const finding of findings) {
      if (finding.severity === "informational") informational.push(finding);
      else if (finding.target !== undefined && structure.positions.has(finding.target))
        byTarget.set(finding.target, [
          ...(byTarget.get(finding.target) ?? []),
          finding,
        ]);
      else global.push(finding);
    }
    return { byTarget, global, informational };
  }, [findings, structure]);
  const navigable = useMemo(
    () =>
      orderFindings(
        findings.filter((finding) => matchesFilter(finding, filter)),
        structure.positions,
      ),
    [findings, filter, structure],
  );
  const visible = useMemo(() => {
    if (filter === "all") return null;
    const ids = new Set<string>();
    for (const finding of navigable) {
      const target = finding.target;
      if (target === undefined) continue;
      if (structure.sourceRefs.has(target)) ids.add(target);
      for (const member of structure.groupMembers.get(target) ?? []) ids.add(member);
    }
    return ids;
  }, [filter, navigable, structure]);
  const counts = useMemo(
    () => ({
      all: findings.filter((finding) => matchesFilter(finding, "all")).length,
      blocking: findings.filter((finding) => matchesFilter(finding, "blocking")).length,
      review: findings.filter((finding) => matchesFilter(finding, "review")).length,
    }),
    [findings],
  );
  const included = useMemo(
    () =>
      questionPlaces(working.sections).filter(
        (place) => place.question.excluded === undefined,
      ).length,
    [working.sections],
  );
  const contexts = useMemo(() => {
    const out = new Map<string, string>();
    for (const section of working.sections) {
      for (const item of section.items) {
        if (item.question) out.set(item.question.id, section.title);
        const group = item.group;
        if (group)
          for (const question of group.questions)
            out.set(
              question.id,
              t("imports.review.contextGroup", {
                section: section.title,
                group: group.label ?? "",
              }),
            );
      }
    }
    return out;
  }, [t, working.sections]);
  const selectedBlocks = useMemo(
    () =>
      new Set(
        (selectedId === null ? [] : (structure.sourceRefs.get(selectedId) ?? [])).map(
          (ref) => blockKey(ref.sourceId, ref.blockId),
        ),
      ),
    [selectedId, structure],
  );

  const viewed = useQueries({
    queries: [...new Set(value.sources.map((source) => source.role))].map((role) =>
      sourceViewQuery(importId, role, value.sourceRevision),
    ),
    combine: (results) =>
      results
        .flatMap((result) =>
          result.data === undefined
            ? []
            : [`${result.data.sourceId}\t${result.data.role}`],
        )
        .join("\n"),
  });
  const roleById = useMemo(() => {
    const out = new Map<string, ImportSourceRole>(
      value.sources.map((source) => [source.id, source.role]),
    );
    for (const line of viewed.split("\n")) {
      const [id, role] = line.split("\t");
      if (id && (role === "exam" || role === "answer_key")) out.set(id, role);
    }
    return out;
  }, [value.sources, viewed]);
  const roleOf = useCallback(
    (sourceId: string) => roleById.get(sourceId) ?? null,
    [roleById],
  );

  const showView = useCallback(
    (next: "source" | "exam") => {
      if (wide || next === view) return;
      if (next === "source" && examScroller.current)
        examScrollTop.current = examScroller.current.scrollTop;
      setView(next);
    },
    [view, wide],
  );

  const locate = useCallback(
    (refs: readonly ImportSourceRef[], reveal: boolean) => {
      const role = refs
        .map((ref) => roleOf(ref.sourceId))
        .find((found): found is ImportSourceRole => found !== null);
      if (role !== undefined) setSourceRole(role);
      setFocus((previous) => ({
        refs,
        nonce: previous.nonce + 1,
        take: reveal && !wide,
      }));
      if (reveal) showView("source");
    },
    [roleOf, showView, wide],
  );
  const showInSource = useCallback(
    (refs: readonly ImportSourceRef[]) => locate(refs, true),
    [locate],
  );
  const selectFromExam = useCallback(
    (questionId: string) => {
      setSelectedId(questionId);
      setExamTarget(nextTarget(questionId, true));
      const refs = structure.sourceRefs.get(questionId) ?? [];
      if (refs.length > 0) locate(refs, false);
    },
    [locate, structure],
  );
  const selectFromSource = useCallback(
    (questionId: string) => {
      setSelectedId(questionId);
      showView("exam");
      setExamTarget(nextTarget(questionId, !wide));
    },
    [showView, wide],
  );

  const goTo = useCallback(
    (finding: ImportFinding) => {
      setAnchor({ id: finding.id, at: findingRank(finding, structure.positions) });
      const questionId = questionFor(finding, structure);
      if (questionId !== null) setSelectedId(questionId);
      let refs: readonly ImportSourceRef[] = finding.evidence;
      if (refs.length === 0 && questionId !== null)
        refs = structure.sourceRefs.get(questionId) ?? [];
      if (refs.length > 0) locate(refs, false);
      showView("exam");
      setFindingTarget(nextTarget(finding.id, true));
    },
    [locate, showView, structure],
  );

  useLayoutEffect(() => {
    if (wide || view !== "exam" || !examScroller.current) return;
    examScroller.current.scrollTop = examScrollTop.current;
  }, [view, wide]);

  useEffect(() => {
    if (findingTarget === null) return;
    const element = document.getElementById(`finding-${findingTarget.id}`);
    element?.scrollIntoView({ block: "center" });
    element?.focus({ preventScroll: true });
  }, [findingTarget]);

  useEffect(() => {
    if (examTarget === null) return;
    const found = [
      ...(examScroller.current?.querySelectorAll<HTMLElement>("[data-question-id]") ??
        []),
    ].find((element) => element.dataset["questionId"] === examTarget.id);
    if (examTarget.focus) found?.focus({ preventScroll: true });
    found?.scrollIntoView({ block: "nearest" });
  }, [examTarget]);

  const currentFindingId = anchor?.id ?? null;
  const index = navigable.findIndex((finding) => finding.id === currentFindingId);
  const step = (delta: 1 | -1) => {
    const finding = stepFinding(navigable, anchor, delta, structure.positions);
    if (finding) goTo(finding);
  };

  const adopt = useMutation({
    mutationFn: async () => {
      await flush();
      return adoptWordImportReprocessed(importId, {
        expectedRevision: session.revision.current,
      });
    },
    onSuccess: (adopted) => {
      setAdopting(false);
      onAdopted(adopted);
    },
  });

  const reprocessWith = useCallback(
    (paper: number) => void reprocess(paper),
    [reprocess],
  );
  const onReprocess = processing ? reprocessWith : undefined;
  const handlers = useMemo<ExamPaneHandlers>(
    () => ({
      onSelect: selectFromExam,
      onAcknowledge: readOnly ? noop : acknowledge,
      onLocate: showInSource,
      onReprocess,
    }),
    [acknowledge, onReprocess, readOnly, selectFromExam, showInSource],
  );
  const editor = (question: ImportDraftQuestion) => (
    <QuestionEditor
      key={question.id}
      question={question}
      context={contexts.get(question.id) ?? ""}
      findings={grouped.byTarget.get(question.id) ?? NONE}
      currentFindingId={currentFindingId}
      readOnly={readOnly}
      roleOf={roleOf}
      onEdit={(update) => editQuestion(question.id, update)}
      onAcknowledge={handlers.onAcknowledge}
      onLocate={showInSource}
      onReprocess={onReprocess}
    />
  );

  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      unsaved && currentLocation.pathname !== nextLocation.pathname,
  );
  const blockerRef = useRef(blocker);
  useEffect(() => {
    blockerRef.current = blocker;
  });
  useEffect(() => {
    if (blocker.state !== "blocked") return;
    let active = true;
    flush().then(
      () => {
        if (active) blockerRef.current.proceed?.();
      },
      () => {
        if (active) setLeaveFailed(true);
      },
    );
    return () => {
      active = false;
    };
  }, [blocker.state, flush]);
  useEffect(() => {
    if (!unsaved) return;
    const warn = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [unsaved]);

  const openSummary = () => {
    setSummaryOpen(true);
    flush().catch(() => undefined);
  };
  const showFilter = (next: FindingFilter) => {
    setSummaryOpen(false);
    setFilter(next);
    const first = orderFindings(
      findings.filter((finding) => matchesFilter(finding, next)),
      structure.positions,
    )[0];
    if (!first) return;
    afterSummary.current = first.id;
    goTo(first);
  };
  const restoreAfterSummary = (event: Event) => {
    event.preventDefault();
    const findingId = afterSummary.current;
    afterSummary.current = null;
    const target =
      findingId === null
        ? finishButton.current
        : document.getElementById(`finding-${findingId}`);
    target?.focus();
  };

  const leaveDialog = (
    <ConfirmDialog
      open={blocker.state === "blocked"}
      onOpenChange={(open) => {
        if (open) return;
        setLeaveFailed(false);
        blocker.reset?.();
      }}
      title={t("imports.review.leaveTitle")}
      description={leaveDescription(stale, leaveFailed, t)}
      confirmLabel={t("imports.review.leaveAnyway")}
      cancelLabel={t("imports.review.stay")}
      destructive
      disabled={!leaveFailed}
      onConfirm={() => {
        setLeaveFailed(false);
        blocker.proceed?.();
      }}
    />
  );

  if (phone)
    return (
      <>
        <PhoneNotice
          title={working.title}
          importId={importId}
          blocking={counts.blocking}
          review={counts.review}
        />
        {leaveDialog}
      </>
    );

  const sourcePane = (
    <SourcePane
      importId={importId}
      visible={wide || view === "source"}
      sourceRevision={value.sourceRevision}
      sources={value.sources}
      role={sourceRole}
      onRoleChange={setSourceRole}
      focus={focus}
      owners={structure.owners}
      selectedBlocks={selectedBlocks}
      onSelect={selectFromSource}
    />
  );

  return (
    <div className="-m-6 flex h-[calc(100svh-3.5rem)] min-w-0 flex-col overflow-hidden">
      <div className="flex min-h-14 shrink-0 flex-wrap items-center gap-3 border-b px-4 py-2">
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={t("imports.review.back")}
          onClick={() => void navigate(`/admin/imports/${importId}`)}
        >
          <ArrowLeft aria-hidden="true" />
        </Button>
        <Input
          value={working.title}
          maxLength={200}
          disabled={readOnly}
          aria-label={t("imports.review.titleLabel")}
          className="h-8 w-80 min-w-32 border-transparent font-medium shadow-none"
          onChange={(event) => setTitle(event.target.value)}
        />
        <span className="text-muted-foreground text-xs tabular-nums">
          {t("imports.review.counts", {
            questions: included,
            sections: working.sections.length,
          })}
        </span>
        <AutosaveStatusLabel
          status={status}
          onRetry={retry}
          staleLabel={t("imports.review.staleLabel")}
        />
        <div className="ml-auto">
          <Button
            ref={finishButton}
            size="sm"
            disabled={!underReview && commitState.phase !== "done"}
            onClick={openSummary}
          >
            {t("imports.review.finish")}
          </Button>
        </div>
      </div>

      <ReviewBanners
        value={value}
        stale={stale}
        committedTestId={
          commitState.phase === "done" ? commitState.result.testId : null
        }
        reprocessed={server.reprocessed && underReview && !stale && !finished}
        finished={finished}
        onReload={onReload}
        onReloadStale={() => setConfirmReload(true)}
        onAdopt={() => {
          adopt.reset();
          setAdopting(true);
        }}
      />
      {reprocessError === null ? null : (
        <p role="alert" className="border-b px-4 py-2 text-sm">
          {reprocessError}
        </p>
      )}

      <div className="flex shrink-0 flex-wrap items-center gap-2 border-b px-4 py-2">
        <Segmented
          label={t("imports.review.filterLabel")}
          value={filter}
          options={[
            {
              value: "all",
              label: t("imports.review.filterAll", { count: counts.all }),
            },
            {
              value: "blocking",
              label: t("imports.review.filterBlocking", { count: counts.blocking }),
            },
            {
              value: "review",
              label: t("imports.review.filterReview", { count: counts.review }),
            },
          ]}
          onChange={(next) => setFilter(isFindingFilter(next) ? next : "all")}
        />
        <Button
          variant="outline"
          size="xs"
          disabled={navigable.length === 0}
          onClick={() => step(-1)}
        >
          <ChevronLeft aria-hidden="true" />
          {t("imports.review.previousFinding")}
        </Button>
        <span
          role="status"
          aria-live="polite"
          className="text-muted-foreground text-xs tabular-nums"
        >
          {positionLabel(index, navigable.length, t)}
        </span>
        <Button
          variant="outline"
          size="xs"
          disabled={navigable.length === 0}
          onClick={() => step(1)}
        >
          {t("imports.review.nextFinding")}
          <ChevronRight aria-hidden="true" />
        </Button>
        {wide ? null : (
          <Segmented
            className="ml-auto"
            label={t("imports.review.viewLabel")}
            value={view}
            options={[
              { value: "source", label: t("imports.review.viewSource") },
              { value: "exam", label: t("imports.review.viewExam") },
            ]}
            onChange={(next) => showView(next === "source" ? "source" : "exam")}
          />
        )}
      </div>

      <div data-columns className="flex min-h-0 min-w-0 flex-1 overflow-hidden">
        {wide ? (
          <SideColumn
            column="importSource"
            side="left"
            aria-label={t("imports.source.title")}
            className="border-r"
          >
            {sourcePane}
          </SideColumn>
        ) : (
          <section
            aria-label={t("imports.source.title")}
            hidden={view !== "source"}
            className="min-w-0 flex-1"
          >
            {sourcePane}
          </section>
        )}
        <div
          ref={examScroller}
          data-resize-middle
          hidden={!wide && view === "source"}
          className="min-w-0 flex-1 overflow-y-auto p-6"
        >
          <ExamPane
            sections={working.sections}
            selectedId={selectedId}
            findingsByTarget={grouped.byTarget}
            globalFindings={grouped.global}
            informational={grouped.informational}
            currentFindingId={currentFindingId}
            visible={visible}
            readOnly={readOnly}
            handlers={handlers}
            editor={editor}
          />
        </div>
      </div>

      <ReviewSummaryDialog
        open={summaryOpen}
        onOpenChange={setSummaryOpen}
        summary={server.summary}
        ready={server.ready && underReview}
        save={status}
        state={commitState}
        onCommit={() => void commit()}
        onRetrySave={retry}
        onShowFilter={showFilter}
        onCloseAutoFocus={restoreAfterSummary}
      />
      <ConfirmDialog
        open={adopting}
        onOpenChange={(open) => !adopt.isPending && setAdopting(open)}
        title={t("imports.review.adoptTitle")}
        description={t("imports.review.adoptBody")}
        confirmLabel={t("imports.review.adoptConfirm")}
        cancelLabel={t("imports.review.adoptKeep")}
        destructive
        pending={adopt.isPending}
        error={
          adopt.isError
            ? failureMessage(adopt.error, t("imports.review.adoptFailed"))
            : null
        }
        onConfirm={() => adopt.mutate()}
      />
      <ConfirmDialog
        open={confirmReload}
        onOpenChange={setConfirmReload}
        title={t("imports.review.reloadTitle")}
        description={t("imports.review.reloadBody")}
        confirmLabel={t("imports.review.reload")}
        destructive
        onConfirm={() => {
          setConfirmReload(false);
          onReload();
        }}
      />
      {leaveDialog}
    </div>
  );
}

function positionLabel(index: number, total: number, t: TFunction): string {
  if (total === 0) return t("imports.review.noFindings");
  if (index === -1) return t("imports.review.findingCount", { count: total });
  return t("imports.review.findingPosition", { current: index + 1, total });
}

function leaveDescription(stale: boolean, failed: boolean, t: TFunction): string {
  if (stale) return t("imports.review.leaveStale");
  return failed ? t("imports.review.leaveFailed") : t("imports.review.leaveSaving");
}
