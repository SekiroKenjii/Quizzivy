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
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { SplitPane } from "@/components/shared/SplitPane";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { useContentWidthAtLeast } from "@/layouts/shell/contentWidth";
import { failureMessage } from "@/lib/api/errors";
import { notify } from "@/lib/toast";
import { cn } from "@/lib/utils";
import {
  adoptWordImportReprocessed,
  type ImportDraftQuestion,
  type ImportDraftSection,
  type ImportFinding,
  type ImportReview,
  type ImportSource,
  type ImportSourceRef,
  type ImportSourceRole,
  type WordImport,
} from "../../api";
import {
  blockKey,
  blockOwners,
  draftPositions,
  exclude,
  questionPlaces,
  setPoints,
} from "../../draft";
import {
  FINDING_FILTERS,
  findingRank,
  isFindingFilter,
  isOpenIn,
  isUnresolved,
  matchesFilter,
  orderFindings,
  stepFinding,
  type FindingAnchor,
  type FindingFilter,
} from "../../findings";
import { useImportAvailability } from "../../availability";
import { sourceViewQuery } from "../../queries";
import { isActiveStatus, reprocessOutcome } from "../../status";
import { useReprocess } from "../../useReprocess";
import { useReviewSession } from "../../useReviewSession";
import {
  ExamPane,
  WHOLE_TEST,
  type ExamPaneHandlers,
  type WholeTestFact,
} from "./ExamPane";
import { QuestionEditor } from "./QuestionEditor";
import { ExcludeDialog, PointsDialog } from "./ReviewDialogs";
import { PhoneNote, ReviewBanners } from "./ReviewBanners";
import { ReviewToolbar, type ReviewPane } from "./ReviewToolbar";
import { ReviewTopBar } from "./ReviewTopBar";
import { SourcePane, type BlockTag, type SourceFocus } from "./SourcePane";

const NONE: readonly ImportFinding[] = [];

const TWO_PANES = 980;

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

function isGlobal(finding: ImportFinding, structure: Structure): boolean {
  return finding.target === undefined || !structure.positions.has(finding.target);
}

function firstSelection(review: ImportReview, structure: Structure): string | null {
  const ordered = orderFindings(
    review.findings.filter((finding) => isUnresolved(finding)),
    structure.positions,
  );
  for (const finding of ordered) {
    if (isGlobal(finding, structure)) return WHOLE_TEST;
    const questionId = questionFor(finding, structure);
    if (questionId !== null) return questionId;
  }
  return questionPlaces(review.draft.sections)[0]?.question.id ?? null;
}

function selectionFor(finding: ImportFinding, structure: Structure): string | null {
  const questionId = questionFor(finding, structure);
  if (questionId !== null) return questionId;
  return isGlobal(finding, structure) ? WHOLE_TEST : null;
}

function firstOpenIn(
  review: ImportReview,
  filter: FindingFilter,
  structure: Structure,
): ImportFinding | null {
  return (
    orderFindings(
      review.findings.filter((finding) => isOpenIn(finding, filter)),
      structure.positions,
    )[0] ?? null
  );
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
 * ReviewWorkspace is the linked review of one import in the deck's card: the
 * top bar, the toolbar of sections, filters and the navigator, and the source
 * and the reconstructed test side by side from 980px of content width, one
 * at a time below it. It edits a copy of `initial` taken at mount and is
 * read-only on a phone and whenever the import is not under review, the
 * draft changed elsewhere, processing finished after the page opened, or a
 * reprocess is under way. Opened with `?filter=`, it starts on that filter's
 * first open finding and focuses it, once, at mount; with `?question=`, on
 * that question's card, focused. "Preview & finish" goes
 * to the import's "Preview and create" page.
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
  const importId = value.id;
  const [origin] = useState(initial);
  const locked = useRef(false);
  const session = useReviewSession(importId, origin, locked);
  const { working, server, editQuestion, setTitle, acknowledge } = session;
  const { status, flush, retry } = session.autosave;
  const navigate = useNavigate();
  const {
    reprocess,
    pending: reprocessPending,
    error: reprocessError,
  } = useReprocess(importId, flush);
  const processingOn = useImportAvailability() !== "reviewOnly";
  const phone = !useMediaQuery("(min-width: 768px)");
  const roomy = useContentWidthAtLeast(TWO_PANES);
  const wide = roomy && !phone;
  const [params, setParams] = useSearchParams();
  const requestedFilter = params.get("filter");
  const filter: FindingFilter = isFindingFilter(requestedFilter)
    ? requestedFilter
    : "all";

  const structure = useMemo(() => structureOf(origin.draft.sections), [origin]);
  const [asked] = useState(() => {
    const question = params.get("question");
    return question !== null && structure.sourceRefs.has(question) ? question : null;
  });
  const [jump] = useState(() =>
    requestedFilter === null || asked !== null
      ? null
      : firstOpenIn(origin, filter, structure),
  );
  const [selectedId, setSelectedId] = useState<string | null>(() => {
    if (asked !== null) return asked;
    return jump === null
      ? firstSelection(origin, structure)
      : selectionFor(jump, structure);
  });
  const [section, setSection] = useState("all");
  const [sourceRole, setSourceRole] = useState<ImportSourceRole>("exam");
  const [focus, setFocus] = useState<SourceFocus>(() => ({
    refs: selectedId === null ? [] : (structure.sourceRefs.get(selectedId) ?? []),
    nonce: 0,
    take: false,
  }));
  const [anchor, setAnchor] = useState<FindingAnchor | null>(() =>
    jump === null ? null : { id: jump.id, at: findingRank(jump, structure.positions) },
  );
  const [findingTarget, setFindingTarget] = useState<Target | null>(() =>
    jump === null ? null : { id: jump.id, focus: true, nonce: 0 },
  );
  const [examTarget, setExamTarget] = useState<Target | null>(() =>
    asked === null ? null : { id: asked, focus: true, nonce: 0 },
  );
  const [view, setView] = useState<ReviewPane>("exam");
  const [leaveFailed, setLeaveFailed] = useState(false);
  const [adopting, setAdopting] = useState(false);
  const [confirmReload, setConfirmReload] = useState(false);
  const [excluding, setExcluding] = useState<string | null>(null);
  const excludeReturn = useRef<HTMLElement | null>(null);
  const [pointsOpen, setPointsOpen] = useState(false);
  const [phoneNote, setPhoneNote] = useState(true);
  const examScroller = useRef<HTMLDivElement>(null);
  const examScrollTop = useRef(0);

  const stale = status.kind === "stale";
  const unsaved =
    status.kind === "dirty" ||
    status.kind === "saving" ||
    status.kind === "failed" ||
    stale;
  const committed = value.status === "committed";
  const underReview = value.status === "needs_review";
  const [openedWhile] = useState(value.status);
  const finished =
    isActiveStatus(openedWhile) && underReview && reprocessOutcome(value) === null;
  const readOnly =
    phone || stale || committed || finished || !underReview || reprocessPending;

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
    for (const finding of findings) {
      if (isGlobal(finding, structure)) global.push(finding);
      else
        byTarget.set(finding.target ?? "", [
          ...(byTarget.get(finding.target ?? "") ?? []),
          finding,
        ]);
    }
    return { byTarget, global };
  }, [findings, structure]);
  const sectionOf = useMemo(() => {
    const out = new Map<string, string>();
    for (const place of questionPlaces(working.sections))
      out.set(place.question.id, place.sectionId);
    return out;
  }, [working.sections]);
  const inSection = useCallback(
    (finding: ImportFinding) => {
      if (section === "all") return true;
      const questionId = questionFor(finding, structure);
      if (questionId === null) return finding.target === section;
      return sectionOf.get(questionId) === section;
    },
    [section, sectionOf, structure],
  );
  const navigable = useMemo(
    () =>
      orderFindings(
        findings.filter((finding) => isOpenIn(finding, filter) && inSection(finding)),
        structure.positions,
      ),
    [findings, filter, inSection, structure],
  );
  const visible = useMemo(() => {
    if (filter === "all") return null;
    const ids = new Set<string>();
    for (const finding of findings) {
      if (!matchesFilter(finding, filter)) continue;
      if (isGlobal(finding, structure)) ids.add(WHOLE_TEST);
      const target = finding.target;
      if (target === undefined) continue;
      if (structure.sourceRefs.has(target)) ids.add(target);
      for (const member of structure.groupMembers.get(target) ?? []) ids.add(member);
    }
    return ids;
  }, [filter, findings, structure]);
  const counts = useMemo(
    () =>
      Object.fromEntries(
        FINDING_FILTERS.map((name) => [
          name,
          name === "all"
            ? findings.length
            : findings.filter((finding) => matchesFilter(finding, name)).length,
        ]),
      ) as Record<FindingFilter, number>,
    [findings],
  );
  const open = useMemo(
    () => ({
      blocking: findings.filter((finding) => finding.severity === "blocking").length,
      review: findings.filter(
        (finding) => finding.severity === "review_required" && isUnresolved(finding),
      ).length,
    }),
    [findings],
  );
  const places = useMemo(() => questionPlaces(working.sections), [working.sections]);
  const included = useMemo(
    () => places.filter((place) => place.question.excluded === undefined),
    [places],
  );
  const labels = useMemo(
    () => new Map(places.map((place) => [place.question.id, place.question])),
    [places],
  );
  const contexts = useMemo(() => {
    const out = new Map<string, string>();
    for (const current of working.sections) {
      for (const item of current.items) {
        if (item.question) out.set(item.question.id, current.title);
        const group = item.group;
        if (group)
          for (const question of group.questions)
            out.set(
              question.id,
              t("imports.review.contextGroup", {
                section: current.title,
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
  const unassigned = useMemo(
    () =>
      new Set(
        findings
          .filter((finding) => finding.code === "UNASSIGNED_SOURCE_TEXT")
          .flatMap((finding) =>
            finding.evidence.map((ref) => blockKey(ref.sourceId, ref.blockId)),
          ),
      ),
    [findings],
  );
  const tagOf = useCallback(
    (key: string): BlockTag | undefined => {
      const owner = structure.owners.get(key);
      const question = owner === undefined ? undefined : labels.get(owner);
      if (question !== undefined)
        return {
          label:
            question.excluded === undefined
              ? t("imports.source.questionTag", { label: question.label })
              : t("imports.review.excludedBadge"),
          tone: "muted",
        };
      return unassigned.has(key)
        ? { label: t("imports.source.notAssigned"), tone: "warning" }
        : undefined;
    },
    [labels, structure, t, unassigned],
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
    (next: ReviewPane) => {
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
    (id: string) => {
      setSelectedId(id);
      setExamTarget(nextTarget(id, true));
      const refs = structure.sourceRefs.get(id) ?? [];
      if (refs.length > 0) locate(refs, false);
    },
    [locate, structure],
  );
  const closeCard = useCallback(() => setSelectedId(null), []);
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
      else if (isGlobal(finding, structure)) setSelectedId(WHOLE_TEST);
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
  const onReprocess = processingOn ? reprocessWith : undefined;
  const openPoints = useCallback(() => setPointsOpen(true), []);
  const askExclude = useCallback((questionId: string, returnTo: HTMLElement | null) => {
    excludeReturn.current = returnTo;
    setExcluding(questionId);
  }, []);
  const handlers = useMemo<ExamPaneHandlers>(
    () => ({
      onSelect: selectFromExam,
      onClose: closeCard,
      onAcknowledge: readOnly ? noop : acknowledge,
      onLocate: showInSource,
      onReprocess,
      onPoints: openPoints,
    }),
    [
      acknowledge,
      closeCard,
      onReprocess,
      openPoints,
      readOnly,
      selectFromExam,
      showInSource,
    ],
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
      onExclude={askExclude}
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

  const openPreview = () => void navigate(`/teacher/imports/${importId}/confirm`);

  const excluded = excluding === null ? undefined : labels.get(excluding);
  const facts = wholeTestFacts(value, working.sections, included.length, t);
  const firstPoints = included[0]?.question.points ?? "1";

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
      tagOf={tagOf}
      selectedBlocks={selectedBlocks}
      onSelect={selectFromSource}
    />
  );
  const examPane = (
    <div
      ref={examScroller}
      data-resize-middle
      className={
        wide
          ? "bg-sidebar min-h-0 flex-1 overflow-y-auto px-5 pt-4 pb-10"
          : "bg-sidebar min-h-0 flex-1 overflow-y-auto p-3"
      }
    >
      <ExamPane
        sections={working.sections}
        section={section}
        selectedId={selectedId}
        findingsByTarget={grouped.byTarget}
        globalFindings={grouped.global}
        currentFindingId={currentFindingId}
        visible={visible}
        readOnly={readOnly}
        facts={facts}
        allDone={underReview && !readOnly && open.blocking + open.review === 0}
        onFinish={openPreview}
        handlers={handlers}
        editor={editor}
      />
    </div>
  );

  return (
    <div className="bg-card shadow-card flex h-[calc(100svh-8rem)] min-h-140 min-w-0 flex-col overflow-hidden rounded-xl border">
      <ReviewTopBar
        title={working.title}
        readOnly={readOnly}
        onTitle={setTitle}
        counts={{
          questions: included.length,
          answers: included.filter((place) => place.question.answer.state === "known")
            .length,
          sections: working.sections.length,
        }}
        status={status}
        staleLabel={t("imports.review.staleLabel")}
        onRetry={retry}
        blocking={open.blocking}
        review={open.review}
        finishDisabled={!underReview && !committed}
        onFinish={openPreview}
      />
      <ReviewBanners
        value={value}
        stale={stale}
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
        <p role="alert" className="m-0 border-b px-4 py-2 text-sm">
          {reprocessError}
        </p>
      )}
      <ReviewToolbar
        sections={working.sections.map((current) => ({
          id: current.id,
          title: current.title,
        }))}
        section={section}
        onSection={setSection}
        filter={filter}
        counts={counts}
        onFilter={setFilter}
        narrow={!wide}
        pane={view}
        onPane={showView}
        position={positionLabel(index, navigable.length, t)}
        canStep={navigable.length > 0}
        onStep={step}
      />
      {phone && phoneNote ? <PhoneNote onDismiss={() => setPhoneNote(false)} /> : null}
      <SplitPane
        label={t("imports.review.resizePanes")}
        unit="percent"
        defaultSize={42}
        min={30}
        max={65}
        step={2}
        storageKey="quizzivy.importReview.split"
        handle="line"
        split={wide}
        className="flex-1"
        firstClassName={cn("flex", !wide && view !== "source" && "hidden")}
        secondClassName={cn("flex", !wide && view === "source" && "hidden")}
        first={
          <section
            aria-label={t("imports.source.title")}
            hidden={!wide && view !== "source"}
            className="flex min-w-0 flex-1 flex-col"
          >
            {sourcePane}
          </section>
        }
        second={
          <section
            aria-label={t("imports.review.testPane")}
            hidden={!wide && view === "source"}
            className="flex min-w-0 flex-1 flex-col"
          >
            {examPane}
          </section>
        }
      />

      <ExcludeDialog
        label={excluded?.label ?? ""}
        open={excluding !== null && !readOnly}
        returnFocus={excludeReturn}
        onOpenChange={(next) => {
          if (!next) setExcluding(null);
        }}
        onExclude={(reason) => {
          const questionId = excluding;
          setExcluding(null);
          if (questionId === null) return;
          editQuestion(questionId, (current) => exclude(current, reason));
          notify.success(
            t("imports.review.excludedToast", { label: excluded?.label ?? "" }),
          );
        }}
      />
      <PointsDialog
        sections={working.sections}
        initialPoints={firstPoints}
        open={pointsOpen && !readOnly}
        onOpenChange={setPointsOpen}
        onApply={(points) => {
          setPointsOpen(false);
          for (const place of included)
            editQuestion(place.question.id, (current) => setPoints(current, points));
          notify.success(
            t("imports.review.pointsSet", { points, count: included.length }),
          );
        }}
      />
      <ConfirmDialog
        open={adopting}
        onOpenChange={(next) => !adopt.isPending && setAdopting(next)}
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
      <ConfirmDialog
        open={blocker.state === "blocked"}
        onOpenChange={(next) => {
          if (next) return;
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
    </div>
  );
}

function sourceName(source: ImportSource | undefined, t: TFunction): string {
  if (source === undefined) return t("imports.review.factNone");
  return source.format === "text" ? t("imports.detail.pastedText") : source.filename;
}

function wholeTestFacts(
  value: WordImport,
  sections: readonly ImportDraftSection[],
  questions: number,
  t: TFunction,
): WholeTestFact[] {
  const exam = value.sources.find((source) => source.role === "exam");
  const key = value.sources.find((source) => source.role === "answer_key");
  const groups = sections.reduce(
    (count, current) => count + current.items.filter((item) => item.group).length,
    0,
  );
  return [
    {
      label: t("imports.review.factExam"),
      value: sourceName(exam, t),
    },
    {
      label: t("imports.review.factKey"),
      value: sourceName(key, t),
    },
    {
      label: t("imports.review.factQuestions"),
      value: t("imports.review.factQuestionsValue", {
        questions: t("imports.review.questionsCount", { count: questions }),
        sections: t("imports.review.sectionsCount", { count: sections.length }),
        groups: t("imports.review.groupsCount", { count: groups }),
      }),
    },
  ];
}

function positionLabel(index: number, total: number, t: TFunction): string {
  if (total === 0) return t("imports.review.noFindings");
  if (index === -1) return t("imports.review.openTotal", { count: total });
  return t("imports.review.openPosition", { current: index + 1, total });
}

function leaveDescription(stale: boolean, failed: boolean, t: TFunction): string {
  if (stale) return t("imports.review.leaveStale");
  return failed ? t("imports.review.leaveFailed") : t("imports.review.leaveSaving");
}
