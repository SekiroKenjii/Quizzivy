import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Copy, Ellipsis, FilePlus, Trash2 } from "lucide-react";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { toast } from "@/components/ui/sonner";
import {
  deleteQuestion,
  duplicateQuestion,
  type AdminQuestion,
} from "@/features/question-bank/api";
import { ApiError } from "@/lib/api/errors";
import { AddToTestDialog } from "./AddToTestDialog";

type ReferencingTest = { id: string; title: string };

function referencingTests(cause: unknown): ReferencingTest[] | null {
  if (!(cause instanceof ApiError) || cause.status !== 409) return null;
  const tests = cause.details?.["tests"];
  return Array.isArray(tests) ? (tests as ReferencingTest[]) : [];
}

/**
 * QuestionEditorMenu is the "…" of a saved question in the Question editor:
 * Add to test, Duplicate, and Delete question. Duplicate opens the new copy
 * through `onOpen`, so an unsaved edit still meets the page's guard. Delete
 * asks first, saying no test uses the question; a question still in a draft
 * test cannot be deleted, so Delete instead names those tests (as does a
 * refusal from the server). A deleted question leaves through `onDeleted`.
 */
export function QuestionEditorMenu({
  question,
  onOpen,
  onDeleted,
}: Readonly<{
  question: AdminQuestion;
  onOpen: (id: string) => void;
  onDeleted: () => void;
}>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [adding, setAdding] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [blocked, setBlocked] = useState<ReferencingTest[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const duplicate = useMutation({
    mutationFn: () => duplicateQuestion(question.id),
    onSuccess: async (copy) => {
      await queryClient.invalidateQueries({ queryKey: ["admin-questions"] });
      toast(t("bank.duplicated"));
      onOpen(copy.id);
    },
    onError: () => toast.error(t("bank.duplicateFailed")),
  });

  const remove = useMutation({
    mutationFn: () => deleteQuestion(question.id),
    onSuccess: async () => {
      setConfirming(false);
      await queryClient.invalidateQueries({ queryKey: ["admin-questions"] });
      toast(t("bank.deleted"));
      onDeleted();
    },
    onError: (cause) => {
      const tests = referencingTests(cause);
      if (tests !== null) {
        setConfirming(false);
        setBlocked(tests);
        return;
      }
      setError(cause instanceof ApiError ? cause.message : t("bank.deleteFailed"));
    },
  });

  function askToDelete() {
    setError(null);
    if ((question.usedInTests ?? 0) > 0) setBlocked(question.usedIn ?? []);
    else setConfirming(true);
  }

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            type="button"
            variant="outline"
            size="icon"
            aria-label={t("questionEditor.more")}
            className="bg-card shadow-card size-9 rounded-lg in-data-[scale=deck]:size-9 in-data-[scale=deck]:rounded-lg"
          >
            <Ellipsis aria-hidden="true" className="size-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-55 data-[scale=deck]:w-55">
          <DropdownMenuItem onSelect={() => setAdding(true)}>
            <FilePlus aria-hidden="true" />
            {t("bank.addToTest")}
          </DropdownMenuItem>
          <DropdownMenuItem
            disabled={duplicate.isPending}
            onSelect={() => duplicate.mutate()}
          >
            <Copy aria-hidden="true" />
            {t("bank.duplicate")}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem variant="destructive" onSelect={askToDelete}>
            <Trash2 aria-hidden="true" />
            {t("questionEditor.delete")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <AddToTestDialog
        questionIds={[question.id]}
        open={adding}
        onOpenChange={setAdding}
        onAdded={() =>
          void queryClient.invalidateQueries({
            queryKey: ["admin-question", question.id],
          })
        }
      />

      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        icon={Trash2}
        title={t("questionEditor.deleteTitle")}
        description={t("questionEditor.deleteUnused")}
        confirmLabel={t("common.delete")}
        destructive
        pending={remove.isPending}
        error={error}
        onConfirm={() => remove.mutate()}
      />

      <ConfirmDialog
        open={blocked !== null}
        onOpenChange={(open) => {
          if (!open) setBlocked(null);
        }}
        title={t("bank.deleteBlockedTitle")}
        description={t("bank.deleteBlockedBody")}
        confirmLabel={t("common.close")}
      >
        {blocked !== null && blocked.length > 0 ? (
          <div className="flex flex-col gap-2 text-sm">
            <p>{t("bank.deleteBlockedList", { count: blocked.length })}</p>
            <ul className="flex flex-col gap-1">
              {blocked.map((test) => (
                <li key={test.id}>
                  <Link
                    to={`/teacher/tests/${test.id}/edit`}
                    className="font-medium underline-offset-2 hover:underline"
                  >
                    {test.title}
                  </Link>
                </li>
              ))}
            </ul>
          </div>
        ) : null}
      </ConfirmDialog>
    </>
  );
}
