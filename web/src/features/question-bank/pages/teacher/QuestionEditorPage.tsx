import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Link,
  useBeforeUnload,
  useBlocker,
  useNavigate,
  useParams,
} from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CircleAlert } from "lucide-react";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { EmptyState, LoadError } from "@/components/shared/ListState";
import { Button } from "@/components/ui/button";
import { DeckScale } from "@/components/ui/deck-scale";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/sonner";
import type { MediaAsset } from "@/features/media/api";
import {
  createQuestion,
  getQuestion,
  toFormValues,
  updateQuestion,
  type AdminQuestion,
} from "@/features/question-bank/api";
import { QuestionEditorMenu } from "@/features/question-bank/components/QuestionEditorMenu";
import { QuestionMediaField } from "@/features/question-bank/components/QuestionMediaField";
import {
  ExplanationField,
  QuestionPromptAnswers,
} from "@/features/question-bank/components/QuestionPromptAnswers";
import { QuestionSettingsFields } from "@/features/question-bank/components/QuestionSettingsFields";
import { QuestionTypeControl } from "@/features/question-bank/components/QuestionTypeControl";
import { QuestionUsageBanner } from "@/features/question-bank/components/QuestionUsageBanner";
import {
  blockingIssue,
  emptyQuestion,
  issueKey,
  questionSchema,
  type QuestionValues,
} from "@/features/question-bank/questionSchema";
import { PageHead } from "@/layouts/shell/PageHead";
import { ApiError } from "@/lib/api/errors";

const BANK = "/teacher/question-bank";
const FRAME =
  "@container/qe mx-auto flex w-full max-w-[1320px] min-w-0 flex-col gap-4.5";
const CARD =
  "bg-card shadow-card flex min-w-0 flex-col gap-4.5 rounded-xl border p-4.5";
const GROUP_LABEL =
  "text-muted-fg text-[11.5px] font-semibold tracking-[0.06em] uppercase";

/**
 * QuestionEditorPage is the bank's Question editor at
 * /teacher/question-bank/new and /teacher/question-bank/:id, with its
 * loading skeleton, its not-found state and its load error.
 */
export default function QuestionEditorPage() {
  const { t } = useTranslation();
  const { id } = useParams<{ id: string }>();
  const existing = useQuery({
    queryKey: ["admin-question", id],
    queryFn: ({ signal }) => getQuestion(id ?? "", signal),
    enabled: id !== undefined,
  });

  if (id !== undefined && existing.isPending) return <EditorSkeleton />;
  if (existing.isError) {
    if (existing.error instanceof ApiError && existing.error.status === 404)
      return (
        <DeckScale className="mx-auto w-full max-w-[720px] min-w-0">
          <EmptyState
            action={
              <Button asChild size="sm" variant="outline">
                <Link to={BANK}>{t("questionEditor.backToBank")}</Link>
              </Button>
            }
          >
            {t("questionEditor.notFound")}
          </EmptyState>
        </DeckScale>
      );
    return (
      <DeckScale className="mx-auto w-full max-w-[720px] min-w-0">
        <LoadError error={existing.error} onRetry={() => void existing.refetch()}>
          {t("questionEditor.loadFailed")}
        </LoadError>
      </DeckScale>
    );
  }

  const question = existing.data ?? null;
  async function refreshAsset(): Promise<MediaAsset | null> {
    const { data } = await existing.refetch();
    return data?.media ?? null;
  }
  return (
    <Editor
      key={question?.id ?? "new"}
      question={question}
      onRefreshAsset={refreshAsset}
    />
  );
}

function EditorSkeleton() {
  const { t } = useTranslation();
  return (
    <DeckScale
      role="status"
      aria-label={t("common.loading")}
      className={`${FRAME} focus-within:[&_[data-slot=skeleton]]:[animation-play-state:paused]! hover:[&_[data-slot=skeleton]]:[animation-play-state:paused]!`}
    >
      <Skeleton className="h-4 w-28" />
      <Skeleton className="h-8 w-60" />
      <div className="grid items-start gap-4.5 @min-[900px]/qe:grid-cols-[minmax(0,1fr)_300px]">
        <Skeleton className="h-96 w-full rounded-xl" />
        <Skeleton className="h-72 w-full rounded-xl" />
      </div>
    </DeckScale>
  );
}

function Editor({
  question,
  onRefreshAsset,
}: Readonly<{
  question: AdminQuestion | null;
  onRefreshAsset: () => Promise<MediaAsset | null>;
}>) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [values, setValues] = useState<QuestionValues>(() =>
    question === null ? emptyQuestion() : toFormValues(question),
  );
  const [asset, setAsset] = useState<MediaAsset | null>(question?.media ?? null);
  const [dirty, setDirty] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const leaving = useRef(false);
  const [leaveThen, setLeaveThen] = useState<(() => void) | null>(null);

  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      dirty && !leaving.current && currentLocation.pathname !== nextLocation.pathname,
  );
  useBeforeUnload((event) => {
    if (dirty) event.preventDefault();
  });

  const save = useMutation({
    mutationFn: (body: QuestionValues) =>
      question === null ? createQuestion(body) : updateQuestion(question.id, body),
    onSuccess: async (saved) => {
      setDirty(false);
      await queryClient.invalidateQueries({ queryKey: ["admin-questions"] });
      await queryClient.invalidateQueries({ queryKey: ["admin-question", saved.id] });
      toast(t("questionEditor.saved"));
      if (question === null) {
        leaving.current = true;
        void navigate(`${BANK}/${saved.id}`, { replace: true });
      }
    },
    onError: (cause) =>
      setError(
        cause instanceof ApiError ? cause.message : t("questionEditor.saveFailed"),
      ),
  });

  const blocked = blockingIssue(values, "questionEditor.saveFailed");
  const canSave = blocked === null && (dirty || question === null) && !save.isPending;

  function change(next: QuestionValues) {
    setValues(next);
    setDirty(true);
  }

  function submit() {
    setError(null);
    const parsed = questionSchema.safeParse(values);
    if (!parsed.success) {
      setError(t(issueKey(parsed.error, "questionEditor.saveFailed")));
      return;
    }
    save.mutate(parsed.data);
  }

  function confirmLeave(proceed: () => void) {
    if (dirty) setLeaveThen(() => proceed);
    else proceed();
  }

  function leaveFor(to: string) {
    leaving.current = true;
    void navigate(to);
  }

  return (
    <DeckScale className={FRAME}>
      <PageHead
        title={
          question === null
            ? t("questionEditor.newTitle")
            : t("questionEditor.editTitle")
        }
        back={{ to: BANK, label: t("teacherShell.nav.questionBank") }}
        actions={
          <div className="flex flex-wrap items-center justify-end gap-2.5">
            {blocked === null ? null : (
              <span className="text-muted-fg inline-flex items-center gap-1.5 text-[12.5px]">
                <CircleAlert
                  aria-hidden="true"
                  className="text-warning-ink size-3.5 shrink-0"
                />
                {t(blocked)}
              </span>
            )}
            {question === null ? null : (
              <QuestionEditorMenu
                question={question}
                confirmLeave={confirmLeave}
                onOpen={(copy) => leaveFor(`${BANK}/${copy}`)}
                onDeleted={() => leaveFor(BANK)}
              />
            )}
            <Button
              className="h-9 px-4 in-data-[scale=deck]:h-9 in-data-[scale=deck]:px-4"
              disabled={!canSave}
              onClick={submit}
            >
              {save.isPending ? t("common.saving") : t("common.save")}
            </Button>
          </div>
        }
      />

      {error === null ? null : (
        <p role="alert" className="text-danger-ink text-sm">
          {error}
        </p>
      )}

      {question === null ? null : <QuestionUsageBanner question={question} />}

      <div className="grid items-start gap-4.5 @min-[900px]/qe:grid-cols-[minmax(0,1fr)_300px]">
        <section aria-label={t("questionEditor.question")} className={CARD}>
          <QuestionTypeControl value={values} onChange={change} />
          <QuestionPromptAnswers value={values} onChange={change} />
          <ExplanationField value={values} onChange={change} />
        </section>
        <aside aria-label={t("questionEditor.settings")} className={CARD}>
          <div className="flex flex-col gap-3">
            <p className={GROUP_LABEL}>{t("questionEditor.settingsLabel")}</p>
            <QuestionSettingsFields value={values} onChange={change} />
          </div>
          <hr className="border-border" />
          <div className="flex flex-col gap-3">
            <p className={GROUP_LABEL}>{t("questionEditor.mediaLabel")}</p>
            <QuestionMediaField
              value={values}
              asset={asset}
              onChange={change}
              onAssetChange={setAsset}
              onRefresh={() => {
                void onRefreshAsset().then((fresh) => {
                  if (fresh) setAsset(fresh);
                });
              }}
            />
          </div>
        </aside>
      </div>

      <ConfirmDialog
        open={blocker.state === "blocked" || leaveThen !== null}
        onOpenChange={(open) => {
          if (open) return;
          setLeaveThen(null);
          if (blocker.state === "blocked") blocker.reset();
        }}
        title={t("questionEditor.leaveTitle")}
        description={t("questionEditor.leaveBody")}
        confirmLabel={t("questionEditor.leave")}
        cancelLabel={t("questionEditor.stay")}
        destructive
        onConfirm={() => {
          if (leaveThen !== null) {
            setLeaveThen(null);
            leaveThen();
          } else if (blocker.state === "blocked") blocker.proceed();
        }}
      />
    </DeckScale>
  );
}
