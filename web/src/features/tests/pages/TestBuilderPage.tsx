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
  saveOutline,
  toOutlineDraft,
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

function Builder({ test }: { test: Test }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [title, setTitle] = useState(test.title);
  const [asideSlot, setAsideSlot] = useState<HTMLDivElement | null>(null);
  const [sections, setSections] = useState<OutlineSection[]>(
    () => toOutlineDraft(test).sections,
  );
  const [selectedId, setSelectedId] = useState<string | null>(
    () => test.sections[0]?.questionIds[0] ?? null,
  );
  const [violations, setViolations] = useState<PublishViolation[] | null>(null);
  const [picking, setPicking] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [previewing, setPreviewing] = useState(false);

  const flushQuestion = useRef<(() => Promise<void>) | null>(null);
  const retryQuestion = useRef<(() => void) | null>(null);
  const latestOutline = useRef<OutlineSection[]>(sections);
  const [questionStatus, setQuestionStatus] = useState<AutosaveStatus>({
    kind: "idle",
  });
  const [publishError, setPublishError] = useState<string | null>(null);
  const [publishing, setPublishing] = useState(false);

  // The version guard moves with each save.
  const version = useRef(test.updatedAt);

  const outline = useAutosave<{ title: string; sections: OutlineSection[] }>({
    save: async (draft) => {
      const saved = await saveOutline(test.id, {
        expectedUpdatedAt: version.current,
        title: draft.title,
        description: test.description ?? null,
        sections: draft.sections,
      });
      version.current = saved.updatedAt;
      queryClient.setQueryData(["admin-test", test.id], saved);
    },
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
        problem: problemFor(violations, questionId),
      });
    }
    return map;
  }, [loaded, questionIds, violations]);

  useEffect(() => {
    latestOutline.current = sections;
  }, [sections]);

  const updateOutline = useCallback(
    (next: OutlineSection[]) => {
      setSections(next);
      setSelectedId((current) => {
        if (current === null || next.some((s) => s.questionIds.includes(current))) {
          return current;
        }
        setQuestionStatus({ kind: "idle" });
        return null;
      });
      outline.schedule({ title, sections: next });
    },
    [outline, title],
  );

  function updateTitle(next: string) {
    setTitle(next);
    outline.schedule({ title: next, sections: latestOutline.current });
  }

  function appendQuestion(questionId: string) {
    const current = latestOutline.current;
    const last = current.length - 1;
    updateOutline(
      current.map((section, i) =>
        i === last
          ? { ...section, questionIds: [...section.questionIds, questionId] }
          : section,
      ),
    );
    setSelectedId(questionId);
  }

  async function onCreateQuestion() {
    setPublishError(null);
    setCreating(true);
    try {
      const created = await createQuestion(starterQuestion(t));
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
      await Promise.all([outline.flush(), flushQuestion.current?.()]);
      await publishTest(test.id);
      await queryClient.invalidateQueries({
        queryKey: ["admin-test", test.id],
        refetchType: "all",
      });
      await navigate(`/admin/tests/${test.id}`);
    } catch (cause) {
      if (cause instanceof ApiError && cause.code === "PUBLISH_VALIDATION_FAILED") {
        setViolations(readViolations(cause));
        return;
      }
      setPublishError(
        cause instanceof ApiError ? cause.message : t("builder.publishFailed"),
      );
    } finally {
      setPublishing(false);
    }
  }

  const contextLabel = describePosition(sections, selectedId, t);

  const saveStatus = mergeAutosave([outline.status, questionStatus]);
  const stale = saveStatus.kind === "stale";
  // Typed but not yet on the server: leaving now would lose it (F-14).
  const unsaved =
    saveStatus.kind === "dirty" ||
    saveStatus.kind === "saving" ||
    saveStatus.kind === "failed";
  const leaving = useBlocker(
    ({ currentLocation, nextLocation }) =>
      unsaved && currentLocation.pathname !== nextLocation.pathname,
  );
  useEffect(() => {
    if (!unsaved) return;
    const warn = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [unsaved]);

  const draftQuestions = questionIds.flatMap((questionId) => {
    const found = loaded[questionIds.indexOf(questionId)]?.data;
    return found ? [found] : [];
  });

  return (
    <div className="-m-6 flex h-[calc(100svh-3.5rem)] flex-col overflow-hidden">
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
        {selectedId === null ? null : (
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
            onClick={() => setPreviewing(true)}
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
        <Suspense
          fallback={<div className="w-72 shrink-0 border-r" aria-hidden="true" />}
        >
          <OutlineTree
            sections={sections}
            questions={byId}
            selectedId={selectedId}
            creating={creating}
            onSelect={setSelectedId}
            onChange={updateOutline}
            onCreateQuestion={() => void onCreateQuestion()}
            onPickFromBank={() => setPicking(true)}
            onAddSection={onAddSection}
          />
        </Suspense>

        <div data-resize-middle className="min-w-0 flex-1 overflow-y-auto p-6">
          <PageAsideSlot.Provider value={asideSlot}>
            {selectedId === null ? (
              <p className="text-muted-foreground text-sm">
                {questionIds.length === 0
                  ? t("builder.empty")
                  : t("builder.noSelection")}
              </p>
            ) : (
              <QuestionPane
                settingsOpen={settingsOpen}
                onSettingsOpenChange={setSettingsOpen}
                key={selectedId}
                questionId={selectedId}
                flushRef={flushQuestion}
                retryRef={retryQuestion}
                onStatus={setQuestionStatus}
                contextLabel={contextLabel}
              />
            )}
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
      />

      <PublishDialog
        violations={violations}
        onClose={() => setViolations(null)}
        onGoTo={(questionId) => {
          setSelectedId(questionId);
          setViolations(null);
        }}
      />

      <DraftPreviewDialog
        open={previewing}
        questions={draftQuestions}
        onOpenChange={setPreviewing}
      />

      <ConfirmDialog
        open={leaving.state === "blocked"}
        onOpenChange={(open) => !open && leaving.reset?.()}
        title={t("builder.leaveTitle")}
        description={t("builder.leaveBody")}
        confirmLabel={t("builder.leave")}
        cancelLabel={t("builder.stay")}
        destructive
        onConfirm={() => leaving.proceed?.()}
      />
    </div>
  );
}

/**
 * The builder edits the BANK copy of a question, which is what §7's snapshot
 * model expects: published versions hold their own copy, so a bank edit never
 * reaches a test someone is already sitting.
 */
function QuestionPane({
  questionId,
  flushRef,
  retryRef,
  onStatus,
  contextLabel,
  settingsOpen,
  onSettingsOpenChange,
}: {
  questionId: string;
  flushRef: RefObject<(() => Promise<void>) | null>;
  retryRef: RefObject<(() => void) | null>;
  onStatus: (status: AutosaveStatus) => void;
  contextLabel: string | null;
  settingsOpen: boolean;
  onSettingsOpenChange: (open: boolean) => void;
}) {
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
  initial,
  flushRef,
  retryRef,
  onStatus,
  contextLabel,
  settingsOpen,
  onSettingsOpenChange,
}: {
  questionId: string;
  initial: Parameters<typeof toFormValues>[0];
  flushRef: RefObject<(() => Promise<void>) | null>;
  retryRef: RefObject<(() => void) | null>;
  onStatus: (status: AutosaveStatus) => void;
  contextLabel: string | null;
  settingsOpen: boolean;
  onSettingsOpenChange: (open: boolean) => void;
}) {
  const [values, setValues] = useState<QuestionValues>(() => toFormValues(initial));
  const [asset, setAsset] = useState<MediaAsset | null>(initial.media ?? null);

  const autosave = useAutosave<QuestionValues>({
    save: async (next) => {
      await updateQuestion(questionId, next);
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
): string | null {
  if (questionId === null) return null;
  let number = 0;
  for (const section of sections) {
    for (const id of section.questionIds) {
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

function readViolations(error: ApiError): PublishViolation[] {
  const raw = (error.details as { violations?: unknown } | undefined)?.violations;
  return Array.isArray(raw) ? (raw as PublishViolation[]) : [];
}
