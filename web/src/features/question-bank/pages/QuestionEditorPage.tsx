import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
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

export default function QuestionEditorPage() {
  const { t } = useTranslation();
  const { id } = useParams<{ id: string }>();

  const existing = useQuery({
    queryKey: ["admin-question", id],
    queryFn: ({ signal }) => getQuestion(id ?? "", signal),
    enabled: id !== undefined,
  });

  if (id !== undefined && existing.isPending) {
    return (
      <p role="status" aria-live="polite" className="text-muted-foreground text-sm">
        {t("common.loading")}
      </p>
    );
  }

  if (existing.isError) {
    return (
      <div className="space-y-3">
        <p role="alert" className="text-sm">
          {t("questionEditor.loadFailed")}
        </p>
        <Button variant="outline" size="sm" onClick={() => void existing.refetch()}>
          {t("common.retry")}
        </Button>
      </div>
    );
  }

  const question = existing.data ?? null;

  // Remounting on identity is what seeds the form from the server without an
  // effect that would fight the teacher's edits on every background refetch.
  return <Editor key={question?.id ?? "new"} question={question} />;
}

function Editor({ question }: { question: AdminQuestion | null }) {
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
    <div className="-m-6 flex min-h-full flex-col">
      <div className="flex h-12 items-center gap-2 border-b px-4">
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={t("common.back")}
          onClick={() => void navigate("/admin/question-bank")}
        >
          <ArrowLeft aria-hidden="true" />
        </Button>
        <h1 className="text-sm font-medium">
          {question === null
            ? t("questionEditor.newTitle")
            : t("questionEditor.editTitle")}
        </h1>
        <div className="ml-auto flex items-center gap-2">
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
        </div>
      </div>

      {error === null ? null : (
        <p role="alert" className="text-destructive border-b px-4 py-2 text-sm">
          {error}
        </p>
      )}

      <div className="flex-1 p-6">
        <QuestionEditor
          value={values}
          asset={asset}
          onChange={setValues}
          onAssetChange={setAsset}
        />
      </div>
    </div>
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
