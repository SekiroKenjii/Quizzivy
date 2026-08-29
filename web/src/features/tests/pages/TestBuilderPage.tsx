import { lazy, Suspense, useCallback, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router";
import { useQueries, useQuery } from "@tanstack/react-query";
import { ArrowLeft } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { QuestionEditor } from "@/features/question-bank/components/QuestionEditor";
import {
  createQuestion,
  getQuestion,
  toFormValues,
  updateQuestion,
} from "@/features/question-bank/api";
import {
  emptyQuestion,
  type QuestionValues,
} from "@/features/question-bank/questionSchema";
import type { MediaAsset } from "@/features/media/api";
import {
  getTest,
  publishTest,
  saveOutline,
  toOutlineDraft,
  type PublishViolation,
  type Test,
} from "@/features/tests/api";
import { PublishDialog } from "@/features/tests/components/PublishDialog";
import { AutosaveStatusLabel } from "@/features/tests/components/AutosaveStatusLabel";
import { QuestionPickerDialog } from "@/features/tests/components/QuestionPickerDialog";
import { useAutosave } from "@/features/tests/useAutosave";
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
    return (
      <p role="status" aria-live="polite" className="text-muted-foreground text-sm">
        {t("common.loading")}
      </p>
    );
  }

  if (test.isError) {
    return (
      <div className="space-y-3">
        <p role="alert" className="text-sm">
          {t("builder.loadFailed")}
        </p>
        <Button variant="outline" size="sm" onClick={() => void test.refetch()}>
          {t("common.retry")}
        </Button>
      </div>
    );
  }

  return <Builder key={test.data.id} test={test.data} />;
}

function Builder({ test }: { test: Test }) {
  const { t } = useTranslation();
  const navigate = useNavigate();

  const [title, setTitle] = useState(test.title);
  const [sections, setSections] = useState<OutlineSection[]>(
    () => toOutlineDraft(test).sections,
  );
  const [selectedId, setSelectedId] = useState<string | null>(
    () => test.sections[0]?.questionIds[0] ?? null,
  );
  const [violations, setViolations] = useState<PublishViolation[] | null>(null);
  const [picking, setPicking] = useState(false);
  const [creating, setCreating] = useState(false);
  const [publishError, setPublishError] = useState<string | null>(null);
  const [publishing, setPublishing] = useState(false);

  const outline = useAutosave<{ title: string; sections: OutlineSection[] }>({
    save: async (draft) => {
      await saveOutline(test.id, {
        expectedUpdatedAt: test.updatedAt,
        title: draft.title,
        description: test.description ?? null,
        sections: draft.sections,
      });
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

  const updateOutline = useCallback(
    (next: OutlineSection[]) => {
      setSections(next);
      outline.schedule({ title, sections: next });
    },
    [outline, title],
  );

  function updateTitle(next: string) {
    setTitle(next);
    outline.schedule({ title: next, sections });
  }

  // A new question goes into the LAST section, which is where a teacher who
  // just typed a section title is looking.
  function appendQuestion(questionId: string) {
    const last = sections.length - 1;
    updateOutline(
      sections.map((section, i) =>
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
      const created = await createQuestion(emptyQuestion());
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
      // Publishing snapshots what is SAVED, so anything still in the debounce
      // window has to land first or the version misses the last edit.
      await outline.flush();
      await publishTest(test.id);
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

  const stale = outline.status.kind === "stale";

  return (
    <div className="-m-6 flex min-h-full flex-col">
      <div className="flex h-14 items-center gap-3 border-b px-4">
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
          className="h-8 w-96 border-transparent font-medium shadow-none"
          onChange={(event) => updateTitle(event.target.value)}
        />
        <Badge variant="secondary">{t(`builder.${test.status}`)}</Badge>
        <AutosaveStatusLabel status={outline.status} />

        <div className="ml-auto flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => void navigate(`/admin/tests/${test.id}`)}
          >
            {t("builder.previewAsStudent")}
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
        <div role="alert" className="flex items-center gap-3 border-b px-4 py-2">
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

      <div className="flex min-h-0 flex-1">
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

        <div className="min-w-0 flex-1 p-6">
          {selectedId === null ? (
            <p className="text-muted-foreground text-sm">
              {questionIds.length === 0 ? t("builder.empty") : t("builder.noSelection")}
            </p>
          ) : (
            <QuestionPane key={selectedId} questionId={selectedId} />
          )}
        </div>
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
    </div>
  );
}

/**
 * The builder edits the BANK copy of a question, which is what §7's snapshot
 * model expects: published versions hold their own copy, so a bank edit never
 * reaches a test someone is already sitting.
 */
function QuestionPane({ questionId }: { questionId: string }) {
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

  return <QuestionForm questionId={questionId} initial={question.data} />;
}

function QuestionForm({
  questionId,
  initial,
}: {
  questionId: string;
  initial: Parameters<typeof toFormValues>[0];
}) {
  const [values, setValues] = useState<QuestionValues>(() => toFormValues(initial));
  const [asset, setAsset] = useState<MediaAsset | null>(initial.media ?? null);

  const autosave = useAutosave<QuestionValues>({
    save: async (next) => {
      await updateQuestion(questionId, next);
    },
  });

  return (
    <div className="space-y-3">
      <AutosaveStatusLabel status={autosave.status} />
      <QuestionEditor
        value={values}
        asset={asset}
        onChange={(next) => {
          setValues(next);
          autosave.schedule(next);
        }}
        onAssetChange={setAsset}
      />
    </div>
  );
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
