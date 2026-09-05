import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { ListSkeleton, LoadError } from "@/components/shared/ListState";
import { QuestionEditor } from "@/features/question-bank/components/QuestionEditor";
import {
  createQuestion,
  getQuestion,
  toFormValues,
  updateQuestion,
  type AdminQuestion,
} from "@/features/question-bank/api";
import {
  emptyQuestion,
  questionSchema,
  type QuestionValues,
} from "@/features/question-bank/questionSchema";
import {
  comparePlaceholders,
  hasMismatch,
} from "@/features/question-bank/placeholders";
import type { MediaAsset } from "@/features/media/api";
import { ApiError } from "@/lib/api/errors";
import { toast } from "@/components/ui/sonner";
import { PageHeader } from "@/components/shared/PageHeader";

export default function QuestionEditorPage() {
  const { t } = useTranslation();
  const { id } = useParams<{ id: string }>();

  const existing = useQuery({
    queryKey: ["admin-question", id],
    queryFn: ({ signal }) => getQuestion(id ?? "", signal),
    enabled: id !== undefined,
  });

  if (id !== undefined && existing.isPending) {
    return <ListSkeleton rows={8} />;
  }

  if (existing.isError) {
    return (
      <LoadError error={existing.error} onRetry={() => void existing.refetch()}>
        {t("questionEditor.loadFailed")}
      </LoadError>
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

function Editor({
  question,
  onRefreshAsset,
}: {
  question: AdminQuestion | null;
  onRefreshAsset: () => Promise<MediaAsset | null>;
}) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [values, setValues] = useState<QuestionValues>(() =>
    question === null ? emptyQuestion() : toFormValues(question),
  );
  const [asset, setAsset] = useState<MediaAsset | null>(question?.media ?? null);
  const [error, setError] = useState<string | null>(null);

  const save = useMutation({
    mutationFn: (body: QuestionValues) =>
      question === null ? createQuestion(body) : updateQuestion(question.id, body),
    onSuccess: async (saved) => {
      await queryClient.invalidateQueries({ queryKey: ["admin-questions"] });
      toast(t("questionEditor.saved"));
      void navigate(`/admin/question-bank/${saved.id}`, { replace: true });
    },
    onError: (cause) => {
      setError(
        cause instanceof ApiError ? cause.message : t("questionEditor.saveFailed"),
      );
    },
  });

  const blocked = blockingIssue(values);

  function submit() {
    setError(null);
    const parsed = questionSchema.safeParse(values);
    if (!parsed.success) {
      setError(t(parsed.error.issues[0]?.message ?? "questionEditor.saveFailed"));
      return;
    }
    save.mutate(parsed.data);
  }

  return (
    <>
      <PageHeader
        title={
          question === null
            ? t("questionEditor.newTitle")
            : t("questionEditor.editTitle")
        }
        backTo="/admin/question-bank"
        actions={
          <>
            {blocked === null ? null : (
              <span className="text-muted-foreground text-xs">{t(blocked)}</span>
            )}
            <Button
              size="sm"
              disabled={save.isPending || blocked !== null}
              onClick={submit}
            >
              {save.isPending ? t("common.saving") : t("common.save")}
            </Button>
          </>
        }
      />

      {error === null ? null : (
        <p role="alert" className="text-destructive mb-4 text-sm">
          {error}
        </p>
      )}

      <QuestionEditor
        value={values}
        asset={asset}
        onRefresh={() => {
          void onRefreshAsset().then((fresh) => {
            if (fresh) setAsset(fresh);
          });
        }}
        onChange={setValues}
        onAssetChange={setAsset}
      />
    </>
  );
}

// The same rules the server applies, named as translation keys so the reason a
// Save is disabled is on screen rather than discovered by clicking it.
function blockingIssue(values: QuestionValues): string | null {
  if (values.prompt.trim() === "") return "questionEditor.errors.promptRequired";
  if (values.points <= 0) return "questionEditor.pointsError";

  if (values.type === "fill_blank") {
    const ordinals = values.blanks.map((blank) => blank.ordinal);
    if (values.blanks.length === 0) return "questionEditor.errors.blankRequired";
    if (hasMismatch(comparePlaceholders(values.prompt, ordinals))) {
      return "questionEditor.errors.placeholderMismatch";
    }
    if (values.blanks.some((blank) => blank.acceptedAnswers.length === 0)) {
      return "questionEditor.errors.answerRequired";
    }
    return null;
  }

  if (values.type === "short_answer") return null;

  if (values.options.length < 2) return "questionEditor.errors.twoOptions";
  if (values.options.some((option) => option.text.trim() === "")) {
    return "questionEditor.errors.optionRequired";
  }
  if (!values.options.some((option) => option.isCorrect)) {
    return "questionEditor.errors.correctRequired";
  }
  return null;
}
