import { publishProblem } from "../publishProblem";
import {
  lazy,
  Suspense,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useTranslation } from "react-i18next";
import type { RefObject } from "react";
import type { TFunction } from "i18next";
import { useBlocker, useNavigate, useParams } from "react-router";
import { useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, Eye, History, SlidersHorizontal } from "lucide-react";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { ListSkeleton, LoadError } from "@/components/shared/ListState";
import { StatusBadge } from "@/components/shared/StatusBadge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { QuestionEditor } from "@/features/question-bank/components/QuestionEditor";
import { PageAsideSlot } from "@/layouts/slots";
import {
  createQuestion,
  getQuestion,
  toFormValues,
  updateQuestion,
} from "@/features/question-bank/api";
import type { QuestionValues } from "@/features/question-bank/questionSchema";
import type { MediaAsset } from "@/features/media/api";
import {
  getTest,
  publishTest,
  saveMixedOutline,
  type PublishViolation,
  type Test,
} from "@/features/tests/api";
import { DraftPreviewDialog } from "@/features/tests/components/DraftPreviewDialog";
import { PublishDialog } from "@/features/tests/components/PublishDialog";
import { AutosaveStatusLabel } from "@/features/tests/components/AutosaveStatusLabel";
import { QuestionPickerDialog } from "@/features/tests/components/QuestionPickerDialog";
import {
  mergeAutosave,
  useAutosave,
  type AutosaveStatus,
} from "@/features/tests/useAutosave";
import type { OutlineSection } from "@/features/tests/outline";
import type { OutlineQuestion } from "@/features/tests/components/OutlineTree";
import {
  createGroup,
  copyGroup,
  deleteGroup,
  getGroup,
  updateGroup,
  type GroupBundle,
  type GroupSummary,
  type StoredGroup,
} from "@/features/question-groups/api";
import { emptyGroup } from "@/features/question-groups/model";
import { BuilderGroupPane, type GroupPaneBridge } from "../components/BuilderGroupPane";
import { GroupPickerDialog } from "../components/GroupPickerDialog";
import { BuilderWrites } from "../BuilderWrites";
import {
  editableOutline,
  findUnit,
  groupOwners,
  reconcileSections,
  unitsOf,
  withUnits,
} from "../outlineUnits";
import { ApiError } from "@/lib/api/errors";

/**
 * §2 asks for dnd-kit to be split out: it is ~40 kB that only the one admin
 * who is reordering an outline ever needs, and every student pays for it
 * otherwise.
 */
const OutlineTree = lazy(() =>
  import("@/features/tests/components/OutlineTree").then((m) => ({
    default: m.OutlineTree,
  })),
);

export default function TestBuilderPage() {
  const { t } = useTranslation();
  const { id = "" } = useParams();

  const test = useQuery({
    queryKey: ["admin-test", id],
    queryFn: ({ signal }) => getTest(id, signal),
  });

  if (test.isPending) {
    return <ListSkeleton rows={8} />;
  }

  if (test.isError) {
    return (
      <LoadError error={test.error} onRetry={() => void test.refetch()}>
        {t("builder.loadFailed")}
      </LoadError>
    );
  }

  return <Builder key={test.data.id} test={test.data} />;
}

function Builder({ test }: Readonly<{ test: Test }>) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [title, setTitle] = useState(test.title);
  const [asideSlot, setAsideSlot] = useState<HTMLDivElement | null>(null);
  const [sections, setSections] = useState<OutlineSection[]>(() =>
    editableOutline(test),
  );
  const [selectedId, setSelectedId] = useState<string | null>(() =>
    test.sections[0]?.units?.[0]?.kind === "group"
      ? null
      : (test.sections[0]?.questionIds[0] ?? null),
  );
  const [selectedGroupId, setSelectedGroupId] = useState<string | null>(() =>
    test.sections[0]?.units?.[0]?.kind === "group"
      ? test.sections[0].units[0].id
      : null,
  );
  const [activeBundle, setActiveBundle] = useState<GroupBundle | null>(null);
  const [pickingGroup, setPickingGroup] = useState(false);
  const [removing, setRemoving] = useState<
    { groupId: string } | { sectionIndex: number } | null
  >(null);
  const groupBridge = useRef<GroupPaneBridge | null>(null);
  const [writes] = useState(() => new BuilderWrites());
  const savedOutline = useRef(editableOutline(test));
  const sectionIds = useRef(
    new Map(test.sections.map((section) => [section.id, section.id])),
  );
  const [violations, setViolations] = useState<PublishViolation[] | null>(null);
  const [picking, setPicking] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [previewing, setPreviewing] = useState(false);

  const selectionRequest = useRef(0);
  const [starterIds, setStarterIds] = useState<ReadonlySet<string>>(new Set());
  const flushQuestion = useRef<(() => Promise<void>) | null>(null);
  const retryQuestion = useRef<(() => void) | null>(null);
  const latestOutline = useRef<OutlineSection[]>(sections);
  const [questionStatus, setQuestionStatus] = useState<AutosaveStatus>({
    kind: "idle",
  });
  const [publishError, setPublishError] = useState<string | null>(null);
  const [publishing, setPublishing] = useState(false);
  const [leaveBusy, setLeaveBusy] = useState(false);

  // The version guard moves with each save.
  const version = useRef(test.updatedAt);

  const coordinate = useCallback(
    (operation: () => Promise<void>) => writes.run(operation),
    [writes],
  );

  async function stopGroupReads() {
    const ids = [...groupOwners(savedOutline.current).keys()];
    const known = new Map<string, StoredGroup>();
    for (const id of ids) {
      const group = queryClient.getQueryData<StoredGroup>(["admin-group", id]);
      if (group) {
        await queryClient.cancelQueries({ queryKey: ["admin-group", id] });
        known.set(id, group);
      }
    }
    return known;
  }

  function acknowledge(
    saved: Test,
    submitted: OutlineSection[],
    known: Map<string, StoredGroup>,
  ) {
    const previous = groupOwners(savedOutline.current);
    const next = editableOutline(saved);
    const owners = groupOwners(next);
    for (const [index, section] of submitted.entries()) {
      const id = saved.sections[index]?.id;
      if (id) sectionIds.current.set(section.clientId ?? section.id ?? id, id);
    }
    for (const [id, ownerSectionId] of owners) {
      const moved = previous.has(id) && previous.get(id) !== ownerSectionId;
      const group = known.get(id);
      if (group)
        queryClient.setQueryData<StoredGroup>(["admin-group", id], {
          ...group,
          ownerSectionId,
          revision: group.revision + (moved ? 1 : 0),
          testUpdatedAt: saved.updatedAt,
        });
      else void queryClient.invalidateQueries({ queryKey: ["admin-group", id] });
      if (id === groupBridge.current?.id)
        groupBridge.current.advanceOwner(moved, saved.updatedAt);
    }
    savedOutline.current = next;
    version.current = saved.updatedAt;
    latestOutline.current = reconcileSections(latestOutline.current, submitted, saved);
    setSections(latestOutline.current);
    queryClient.setQueryData(["admin-test", test.id], saved);
  }

  const outline = useAutosave<{ title: string; sections: OutlineSection[] }>({
    save: (draft) =>
      writes.run(async () => {
        const known = await stopGroupReads();
        const resolved = draft.sections.map((section) => ({
          ...section,
          id:
            sectionIds.current.get(section.clientId ?? section.id ?? "") ?? section.id,
        }));
        const saved = await saveMixedOutline(
          test.id,
          version.current,
          draft.title,
          resolved.map((section) => ({
            id: section.id,
            title: section.title,
            instructions: section.instructions,
            units: unitsOf(section),
          })),
        );
        acknowledge(saved, resolved, known);
      }),
  });

  const questionIds = useMemo(
    () => sections.flatMap((section) => section.questionIds),
    [sections],
  );

  const loaded = useQueries({
    queries: questionIds.map((questionId) => ({
      queryKey: ["admin-question", questionId],
      queryFn: ({ signal }: { signal: AbortSignal }) => getQuestion(questionId, signal),
      staleTime: 60_000,
    })),
  });

  const groupIds = useMemo(
    () =>
      sections.flatMap((section) =>
        unitsOf(section)
          .filter((unit) => unit.kind === "group")
          .map((unit) => unit.id),
      ),
    [sections],
  );
  const loadedGroups = useQueries({
    queries: groupIds.map((id) => ({
      queryKey: ["admin-group", id],
      refetchOnWindowFocus: false,
      queryFn: ({ signal }: { signal: AbortSignal }) => getGroup(id, signal),
      staleTime: 60_000,
    })),
  });
  const groups = new Map(
    loadedGroups.flatMap((result) =>
      result.data ? [[result.data.bundle.group.id, result.data.bundle] as const] : [],
    ),
  );
  if (activeBundle && groupIds.includes(activeBundle.group.id))
    groups.set(activeBundle.group.id, activeBundle);

  const byId = useMemo(() => {
    const map = new Map<string, OutlineQuestion>();
    for (const [index, result] of loaded.entries()) {
      const questionId = questionIds[index];
      if (questionId === undefined || !result.data) continue;
      map.set(questionId, {
        id: questionId,
        prompt: result.data.prompt,
        points: result.data.points,
        hasAudio: result.data.media?.kind === "audio",
        problem: publishProblem(result.data, t) ?? problemFor(violations, questionId),
      });
    }
    return map;
  }, [loaded, questionIds, violations, t]);

  const updateOutline = useCallback(
    (next: OutlineSection[]) => {
      latestOutline.current = next;
      setSections(next);
      setSelectedId((current) => {
        const groupSelected =
          selectedGroupId !== null &&
          findUnit(next, `group:${selectedGroupId}`) !== null;
        if (
          current === null ||
          groupSelected ||
          next.some((s) => s.questionIds.includes(current))
        ) {
          return current;
        }
        setQuestionStatus({ kind: "idle" });
        return null;
      });
      outline.schedule({ title, sections: next });
    },
    [outline, title, selectedGroupId],
  );

  function updateTitle(next: string) {
    setTitle(next);
    outline.schedule({ title: next, sections: latestOutline.current });
  }

  async function selectQuestion(questionId: string) {
    const group = [...groups.values()].find((bundle) =>
      bundle.group.members.some((member) => member.questionId === questionId),
    );
    if (group) {
      await selectGroup(group.group.id, questionId);
      return;
    }
    const request = ++selectionRequest.current;
    try {
      await flushQuestion.current?.();
      if (selectionRequest.current === request) {
        setSelectedGroupId(null);
        setActiveBundle(null);
        setSelectedId(questionId);
      }
    } catch (cause) {
      setPublishError(
        cause instanceof ApiError ? cause.message : t("builder.saveBeforeSwitchFailed"),
      );
    }
  }

  function appendQuestion(questionId: string) {
    const current = latestOutline.current;
    const last = destinationIndex();
    updateOutline(
      current.map((section, i) =>
        i === last
          ? withUnits(section, [
              ...unitsOf(section),
              { kind: "question", id: questionId },
            ])
          : section,
      ),
    );
    void selectQuestion(questionId);
  }

  async function onCreateQuestion() {
    setPublishError(null);
    setCreating(true);
    try {
      const created = await createQuestion(starterQuestion(t));
      queryClient.setQueryData(["admin-question", created.id], created);
      setStarterIds((current) => new Set([...current, created.id]));
      appendQuestion(created.id);
    } catch (cause) {
      setPublishError(
        cause instanceof ApiError ? cause.message : t("builder.addFailed"),
      );
    } finally {
      setCreating(false);
    }
  }

  function onAddSection() {
    updateOutline([
      ...sections,
      {
        id: null,
        clientId: crypto.randomUUID(),
        units: [],
        title: t("builder.newSection", { n: sections.length + 1 }),
        instructions: null,
        questionIds: [],
      },
    ]);
  }

  async function onPublish() {
    setPublishError(null);
    setViolations(null);
    setPublishing(true);
    try {
      await flushQuestion.current?.();
      await outline.flush();
      await publishTest(test.id);
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ["admin-test", test.id],
          refetchType: "all",
        }),
        queryClient.invalidateQueries({ queryKey: ["admin-test-versions", test.id] }),
        queryClient.invalidateQueries({ queryKey: ["admin-test-preview", test.id] }),
        queryClient.invalidateQueries({ queryKey: ["admin-tests"] }),
      ]);
      await navigate(`/admin/tests/${test.id}`);
    } catch (cause) {
      if (cause instanceof ApiError && cause.code === "PUBLISH_VALIDATION_FAILED") {
        setViolations(cause.violations);
        return;
      }
      setPublishError(
        cause instanceof ApiError ? cause.message : t("builder.publishFailed"),
      );
    } finally {
      setPublishing(false);
    }
  }

  async function selectGroup(groupId: string, questionId?: string) {
    const request = ++selectionRequest.current;
    if (selectedGroupId === groupId) {
      setSelectedId(questionId ?? null);
      return;
    }
    try {
      await flushQuestion.current?.();
      if (selectionRequest.current !== request) return;
      setSelectedGroupId(groupId);
      setSelectedId(questionId ?? null);
      setActiveBundle(null);
      setQuestionStatus({ kind: "idle" });
    } catch (cause) {
      report(cause);
    }
  }

  function report(cause: unknown) {
    setPublishError(
      cause instanceof ApiError ? cause.message : t("builder.saveBeforeSwitchFailed"),
    );
  }

  async function saveGroup(bundle: GroupBundle, revision: number) {
    await queryClient.cancelQueries({ queryKey: ["admin-group", bundle.group.id] });
    const saved = await updateGroup(bundle.group.id, {
      bundle,
      expectedRevision: revision,
      expectedTestUpdatedAt: version.current,
    });
    version.current = saved.testUpdatedAt ?? version.current;
    queryClient.setQueryData(["admin-group", bundle.group.id], saved);
    return saved;
  }

  async function insertGroup(source?: GroupSummary, sectionIndex = destinationIndex()) {
    setCreating(true);
    setPublishError(null);
    try {
      await flushQuestion.current?.();
      await outline.flush();
      await writes.run(async () => {
        const section = latestOutline.current[sectionIndex];
        if (!section?.id) throw new Error("Destination section is not persisted");
        const destination = {
          ownerSectionId: section.id,
          expectedTestUpdatedAt: version.current,
        };
        const saved = source
          ? await copyGroup(source.id, {
              ...destination,
              expectedRevision: source.revision,
            })
          : await createGroup({
              ...destination,
              bundle: emptyGroup(t("groups.newGroup")),
            });
        version.current = saved.testUpdatedAt ?? version.current;
        groupBridge.current?.advanceOwner(false, version.current);
        queryClient.setQueryData(["admin-group", saved.bundle.group.id], saved);
        const next = latestOutline.current.map((item, index) =>
          index === sectionIndex
            ? withUnits(item, [
                ...unitsOf(item),
                { kind: "group", id: saved.bundle.group.id },
              ])
            : item,
        );
        latestOutline.current = next;
        savedOutline.current = next;
        setSections(next);
        setPickingGroup(false);
        setSelectedGroupId(saved.bundle.group.id);
        setSelectedId(null);
        setActiveBundle(null);
        setQuestionStatus({ kind: "idle" });
      });
    } catch (cause) {
      report(cause);
      throw cause;
    } finally {
      setCreating(false);
    }
  }

  function destinationIndex() {
    const index = sections.findIndex((section) =>
      unitsOf(section).some((unit) =>
        selectedGroupId
          ? unit.kind === "group" && unit.id === selectedGroupId
          : unit.kind === "question" && unit.id === selectedId,
      ),
    );
    return index < 0 ? sections.length - 1 : index;
  }

  async function removeContent() {
    if (!removing) return;
    setCreating(true);
    setPublishError(null);
    try {
      await flushQuestion.current?.();
      await outline.flush();
      const removedSectionId =
        "sectionIndex" in removing
          ? latestOutline.current[removing.sectionIndex]?.id
          : null;
      const ids =
        "groupId" in removing
          ? [removing.groupId]
          : unitsOf(latestOutline.current[removing.sectionIndex]!)
              .filter((unit) => unit.kind === "group")
              .map((unit) => unit.id);
      for (const id of ids) await removeOwnedGroup(id);
      if ("sectionIndex" in removing)
        updateOutline(
          latestOutline.current.filter((section) => section.id !== removedSectionId),
        );
      setRemoving(null);
    } catch (cause) {
      report(cause);
    } finally {
      setCreating(false);
    }
  }

  async function removeOwnedGroup(id: string) {
    await writes.run(async () => {
      const group =
        queryClient.getQueryData<StoredGroup>(["admin-group", id]) ??
        (await getGroup(id));
      await deleteGroup(id, group.revision, version.current);
      const fresh = await getTest(test.id);
      version.current = fresh.updatedAt;
      groupBridge.current?.advanceOwner(false, fresh.updatedAt);
      const next = editableOutline(fresh);
      latestOutline.current = next;
      savedOutline.current = next;
      setTitle(fresh.title);
      setSections(next);
      queryClient.setQueryData(["admin-test", test.id], fresh);
      if (selectedGroupId === id) {
        setSelectedGroupId(null);
        setSelectedId(null);
        setActiveBundle(null);
        setQuestionStatus({ kind: "idle" });
      }
    });
  }

  async function openPreview() {
    setPublishError(null);
    try {
      await flushQuestion.current?.();
      await outline.flush();
      await Promise.all([
        ...questionIds.map((id) =>
          queryClient.query({
            staleTime: 60_000,
            queryKey: ["admin-question", id],
            queryFn: ({ signal }) => getQuestion(id, signal),
          }),
        ),
        ...groupIds.map((id) =>
          queryClient.query({
            staleTime: 60_000,
            queryKey: ["admin-group", id],
            queryFn: ({ signal }) => getGroup(id, signal),
          }),
        ),
      ]);
      setPreviewing(true);
    } catch (cause) {
      report(cause);
    }
  }

  const contextLabel = describePosition(sections, selectedId, t, groups);

  const saveStatus = mergeAutosave([outline.status, questionStatus]);
  const stale = saveStatus.kind === "stale";
  // Typed but not yet on the server: leaving now would lose it (F-14).
  const unsaved =
    saveStatus.kind === "dirty" ||
    saveStatus.kind === "saving" ||
    saveStatus.kind === "failed" ||
    saveStatus.kind === "stale";
  const leaving = useBlocker(
    ({ currentLocation, nextLocation }) =>
      unsaved &&
      nextLocation.pathname !== "/login" &&
      currentLocation.pathname !== nextLocation.pathname,
  );
  useEffect(() => {
    if (!unsaved) return;
    const warn = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [unsaved]);

  const draftQuestions = sections.flatMap((section) =>
    section.questionIds.flatMap((questionId) => {
      const found = loaded[questionIds.indexOf(questionId)]?.data;
      return found ? [{ sectionId: section.id ?? "", question: found }] : [];
    }),
  );

  function activeEditor() {
    if (selectedGroupId)
      return (
        <BuilderGroupPane
          key={selectedGroupId}
          id={selectedGroupId}
          selectedQuestionId={selectedId}
          onSelectedQuestionChange={setSelectedId}
          coordinate={coordinate}
          save={saveGroup}
          onChange={setActiveBundle}
          flushRef={flushQuestion}
          retryRef={retryQuestion}
          bridgeRef={groupBridge}
          onStatus={setQuestionStatus}
        />
      );
    if (selectedId === null)
      return (
        <p className="text-muted-foreground text-sm">
          {questionIds.length === 0 ? t("builder.empty") : t("builder.noSelection")}
        </p>
      );
    return (
      <QuestionPane
        settingsOpen={settingsOpen}
        onSettingsOpenChange={setSettingsOpen}
        key={selectedId}
        questionId={selectedId}
        clearStarterPrompt={starterIds.has(selectedId)}
        flushRef={flushQuestion}
        retryRef={retryQuestion}
        onStatus={setQuestionStatus}
        contextLabel={contextLabel}
      />
    );
  }

  return (
    <fieldset
      disabled={creating || publishing}
      className="-m-6 flex h-[calc(100svh-3.5rem)] min-w-0 flex-col overflow-hidden"
    >
      <div className="flex h-14 shrink-0 items-center gap-3 border-b px-4">
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={t("common.back")}
          onClick={() => void navigate("/admin/tests")}
        >
          <ArrowLeft aria-hidden="true" />
        </Button>
        <Input
          disabled={creating}
          value={title}
          aria-label={t("builder.titleLabel")}
          className="h-8 w-96 min-w-32 border-transparent font-medium shadow-none"
          onChange={(event) => updateTitle(event.target.value)}
        />
        <StatusBadge kind="test" status={test.status} />
        <AutosaveStatusLabel
          status={saveStatus}
          onRetry={() => {
            outline.retry();
            retryQuestion.current?.();
          }}
        />
        {selectedId === null || selectedGroupId ? null : (
          <Button
            variant="outline"
            size="sm"
            className="lg:hidden"
            onClick={() => setSettingsOpen(true)}
          >
            <SlidersHorizontal aria-hidden="true" />
            <span className="hidden sm:inline">{t("questionEditor.settings")}</span>
          </Button>
        )}

        <div className="ml-auto flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            className="text-muted-foreground"
            aria-label={t("builder.versions")}
            onClick={() => void navigate(`/admin/tests/${test.id}#versions`)}
          >
            <History aria-hidden="true" />
            <span className="hidden lg:inline">{t("builder.versions")}</span>
          </Button>
          <Button
            variant="outline"
            size="sm"
            aria-label={t("builder.previewAsStudent")}
            onClick={() => void openPreview()}
          >
            <Eye aria-hidden="true" />
            <span className="hidden lg:inline">{t("builder.previewAsStudent")}</span>
          </Button>
          <Button
            size="sm"
            disabled={publishing || stale}
            onClick={() => void onPublish()}
          >
            {publishing ? t("builder.publishing") : t("builder.publish")}
          </Button>
        </div>
      </div>

      {stale ? (
        <div
          role="alert"
          className="flex shrink-0 items-center gap-3 border-b px-4 py-2"
        >
          <p className="text-sm">{t("builder.staleBody")}</p>
          <Button
            variant="outline"
            size="sm"
            className="ml-auto"
            onClick={() => window.location.reload()}
          >
            {t("builder.reload")}
          </Button>
        </div>
      ) : null}

      {publishError === null ? null : (
        <p role="alert" className="text-destructive border-b px-4 py-2 text-sm">
          {publishError}
        </p>
      )}

      <div data-columns className="flex min-h-0 flex-1 overflow-hidden">
        <fieldset disabled={creating} className="contents">
          <Suspense
            fallback={<div className="w-72 shrink-0 border-r" aria-hidden="true" />}
          >
            <OutlineTree
              sections={sections}
              groups={groups}
              selectedGroupId={selectedGroupId}
              onSelectGroup={(id, questionId) => void selectGroup(id, questionId)}
              onCreateGroup={() => void insertGroup().catch(() => undefined)}
              onRemoveGroup={(id) => setRemoving({ groupId: id })}
              onRemoveSection={(sectionIndex) => setRemoving({ sectionIndex })}
              questions={byId}
              selectedId={selectedId}
              creating={creating}
              onSelect={(questionId) => void selectQuestion(questionId)}
              onChange={updateOutline}
              onCreateQuestion={() => void onCreateQuestion()}
              onPickFromBank={() => setPicking(true)}
              onAddSection={onAddSection}
            />
          </Suspense>
        </fieldset>

        <div data-resize-middle className="min-w-0 flex-1 overflow-y-auto p-6">
          <PageAsideSlot.Provider value={asideSlot}>
            {activeEditor()}
          </PageAsideSlot.Provider>
        </div>
        {/* A-04 puts the settings column under the builder's own bar. */}
        <div ref={setAsideSlot} className="contents" />
      </div>

      <QuestionPickerDialog
        open={picking}
        excluded={new Set(questionIds)}
        onOpenChange={setPicking}
        onPick={appendQuestion}
        onPickGroup={() => {
          setPicking(false);
          setPickingGroup(true);
        }}
      />

      <GroupPickerDialog
        open={pickingGroup}
        sections={sections}
        onOpenChange={setPickingGroup}
        onPick={(group, sectionIndex) => insertGroup(group, sectionIndex)}
      />
      <ConfirmDialog
        open={removing !== null}
        onOpenChange={(open) => !open && !creating && setRemoving(null)}
        title={t("builder.removeGroupTitle")}
        description={t("builder.removeGroupBody")}
        confirmLabel={t("common.delete")}
        destructive
        pending={creating}
        error={publishError}
        onConfirm={() => void removeContent()}
      />

      <PublishDialog
        violations={violations}
        warnings={loaded.flatMap((result, index) =>
          result.data && !result.data.explanation?.trim()
            ? [
                {
                  questionId: result.data.id,
                  message: t("builder.missingExplanation", { number: index + 1 }),
                },
              ]
            : [],
        )}
        location={(violation) =>
          violation.questionId
            ? describePosition(sections, violation.questionId, t, groups)
            : (sections.find((section) => section.id === violation.sectionId)?.title ??
              null)
        }
        onGoToSection={(sectionId) => {
          setViolations(null);
          requestAnimationFrame(() => {
            const section = document.querySelector(
              `[data-outline-section="${CSS.escape(sectionId)}"]`,
            );
            const control = section?.querySelector<HTMLButtonElement>(
              "button[aria-expanded]",
            );
            if (control?.getAttribute("aria-expanded") === "false") control.click();
            control?.focus();
            control?.scrollIntoView({ block: "nearest" });
          });
        }}
        onClose={() => setViolations(null)}
        onGoTo={(questionId) => {
          void selectQuestion(questionId);
          setViolations(null);
        }}
      />

      <DraftPreviewDialog
        open={previewing}
        questions={draftQuestions}
        sections={sections}
        groups={loadedGroups.flatMap((result) => (result.data ? [result.data] : []))}
        onOpenChange={setPreviewing}
      />

      <ConfirmDialog
        open={leaving.state === "blocked"}
        onOpenChange={(open) => !open && !leaveBusy && leaving.reset?.()}
        title={t("builder.leaveTitle")}
        description={t("builder.leaveBody")}
        confirmLabel={t("builder.leave")}
        cancelLabel={t("builder.stay")}
        destructive
        pending={leaveBusy}
        error={publishError}
        onConfirm={() => {
          if (!groupBridge.current) {
            leaving.proceed?.();
            return;
          }
          setLeaveBusy(true);
          void groupBridge.current
            .persist()
            .then((ok) => {
              if (ok) leaving.proceed?.();
              else setPublishError(t("groups.localUnavailable"));
            })
            .finally(() => setLeaveBusy(false));
        }}
      >
        <Button
          variant="outline"
          disabled={leaveBusy || stale}
          onClick={() => {
            setLeaveBusy(true);
            void (async () => {
              try {
                await flushQuestion.current?.();
                await outline.flush();
                leaving.proceed?.();
              } catch (cause) {
                report(cause);
              } finally {
                setLeaveBusy(false);
              }
            })();
          }}
        >
          {t("groups.saveAndLeave")}
        </Button>
      </ConfirmDialog>
    </fieldset>
  );
}

/**
 * The builder edits the BANK copy of a question, which is what §7's snapshot
 * model expects: published versions hold their own copy, so a bank edit never
 * reaches a test someone is already sitting.
 */
function QuestionPane({
  questionId,
  clearStarterPrompt,
  flushRef,
  retryRef,
  onStatus,
  contextLabel,
  settingsOpen,
  onSettingsOpenChange,
}: Readonly<{
  questionId: string;
  clearStarterPrompt: boolean;
  flushRef: RefObject<(() => Promise<void>) | null>;
  retryRef: RefObject<(() => void) | null>;
  onStatus: (status: AutosaveStatus) => void;
  contextLabel: string | null;
  settingsOpen: boolean;
  onSettingsOpenChange: (open: boolean) => void;
}>) {
  const { t } = useTranslation();
  const question = useQuery({
    queryKey: ["admin-question", questionId],
    queryFn: ({ signal }) => getQuestion(questionId, signal),
  });

  if (question.isPending) {
    return (
      <p role="status" aria-live="polite" className="text-muted-foreground text-sm">
        {t("common.loading")}
      </p>
    );
  }
  if (question.isError) {
    return (
      <p role="alert" className="text-destructive text-sm">
        {t("questionEditor.loadFailed")}
      </p>
    );
  }

  return (
    <QuestionForm
      questionId={questionId}
      clearStarterPrompt={clearStarterPrompt}
      initial={question.data}
      flushRef={flushRef}
      retryRef={retryRef}
      onStatus={onStatus}
      contextLabel={contextLabel}
      settingsOpen={settingsOpen}
      onSettingsOpenChange={onSettingsOpenChange}
    />
  );
}

function QuestionForm({
  questionId,
  clearStarterPrompt,
  initial,
  flushRef,
  retryRef,
  onStatus,
  contextLabel,
  settingsOpen,
  onSettingsOpenChange,
}: Readonly<{
  questionId: string;
  clearStarterPrompt: boolean;
  initial: Parameters<typeof toFormValues>[0];
  flushRef: RefObject<(() => Promise<void>) | null>;
  retryRef: RefObject<(() => void) | null>;
  onStatus: (status: AutosaveStatus) => void;
  contextLabel: string | null;
  settingsOpen: boolean;
  onSettingsOpenChange: (open: boolean) => void;
}>) {
  const queryClient = useQueryClient();
  const [values, setValues] = useState<QuestionValues>(() => toFormValues(initial));
  const [asset, setAsset] = useState<MediaAsset | null>(initial.media ?? null);

  const autosave = useAutosave<QuestionValues>({
    save: async (next) => {
      await queryClient.cancelQueries({ queryKey: ["admin-question", questionId] });
      const saved = await updateQuestion(questionId, next);
      queryClient.setQueryData(["admin-question", questionId], saved);
      void queryClient.invalidateQueries({ queryKey: ["admin-questions"] });
    },
  });

  const { flush, retry, status } = autosave;
  useEffect(() => {
    flushRef.current = flush;
    retryRef.current = retry;
    return () => {
      flushRef.current = null;
      retryRef.current = null;
    };
  }, [flush, retry, flushRef, retryRef]);

  useEffect(() => {
    onStatus(status);
  }, [onStatus, status]);

  return (
    <div>
      <QuestionEditor
        value={values}
        clearPromptOnFocus={clearStarterPrompt}
        asset={asset}
        contextLabel={contextLabel}
        settings={{
          hideBelow: "lg",
          open: settingsOpen,
          onOpenChange: onSettingsOpenChange,
        }}
        onChange={(next) => {
          setValues(next);
          autosave.schedule(next);
        }}
        onAssetChange={setAsset}
      />
    </div>
  );
}

/** A question the API will accept, not a blank form. */
function starterQuestion(t: TFunction): QuestionValues {
  return {
    type: "single_choice",
    prompt: t("builder.starterPrompt"),
    mediaAssetId: null,
    audio: null,
    transcript: null,
    options: [
      { id: null, text: t("builder.starterOption", { n: 1 }), isCorrect: true },
      { id: null, text: t("builder.starterOption", { n: 2 }), isCorrect: false },
    ],
    blanks: [],
    points: 1,
    explanation: null,
    sampleAnswer: null,
    tags: [],
  };
}

function describePosition(
  sections: OutlineSection[],
  questionId: string | null,
  t: TFunction,
  groups: Map<string, GroupBundle>,
): string | null {
  if (questionId === null) return null;
  let number = 0;
  for (const section of sections) {
    const ids = unitsOf(section).flatMap((unit) =>
      unit.kind === "question"
        ? [unit.id]
        : (groups.get(unit.id)?.group.members.map((member) => member.questionId) ?? []),
    );
    for (const id of ids) {
      number += 1;
      if (id === questionId) {
        return t("builder.position", { number, section: section.title });
      }
    }
  }
  return null;
}

function problemFor(
  violations: PublishViolation[] | null,
  questionId: string,
): string | null {
  return violations?.find((v) => v.questionId === questionId)?.message ?? null;
}
