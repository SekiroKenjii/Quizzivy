import type { RefObject } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import type { TFunction } from "i18next";
import { Library } from "lucide-react";
import { FormDialog } from "@/components/shared/form/FormDialog";
import { Button } from "@/components/ui/button";
import { listQuestions, type AdminQuestion } from "@/features/question-bank/api";

const LIMIT = 100;

interface QuestionPickerDialogProps {
  open: boolean;
  /** Already in the outline: offering them again invites a duplicate. */
  excluded: ReadonlySet<string>;
  /** The section the questions join, named in the description. */
  destination: string | null;
  onOpenChange: (open: boolean) => void;
  onPick: (questionIds: string[]) => void;
  onPickGroup?: () => void;
  /** Takes focus back when the dialog closes, if it is still in the page. */
  returnFocus?: RefObject<HTMLElement | null>;
}

/**
 * QuestionPickerDialog is "Add from question bank": the caller's bank, less
 * what the test already holds, as a searchable list with each question's
 * "{type} · {level} · used in {n} tests"; any number can be ticked, and
 * "Add {n} questions" adds them in the order shown. It reads the first 100
 * questions, the contract's largest page, and searches among them.
 */
export function QuestionPickerDialog({
  open,
  excluded,
  destination,
  onOpenChange,
  onPick,
  onPickGroup,
  returnFocus,
}: Readonly<QuestionPickerDialogProps>) {
  const { t } = useTranslation();
  const bank = useQuery({
    queryKey: ["admin-questions", "picker"],
    queryFn: ({ signal }) => listQuestions({ limit: LIMIT }, signal),
    enabled: open,
  });
  const items = (bank.data?.items ?? []).filter(
    (question) => !excluded.has(question.id),
  );
  const ready = bank.isSuccess && items.length > 0;

  return (
    <FormDialog
      open={open}
      onOpenChange={onOpenChange}
      returnFocus={returnFocus}
      title={t("builder.bank.title")}
      description={
        destination === null
          ? t("builder.bank.descriptionAny")
          : t("builder.bank.description", { section: destination })
      }
      icon={Library}
      width={560}
      initial={{ pick: [] as readonly string[] }}
      fields={
        ready
          ? [
              {
                kind: "list",
                name: "pick",
                label: t("builder.bank.list"),
                required: true,
                requiredText: t("builder.bank.pickOne"),
                searchPlaceholder: t("builder.bank.search"),
                options: items.map((question) => ({
                  value: question.id,
                  label: question.prompt,
                  meta: questionMeta(question, t),
                })),
              },
            ]
          : []
      }
      submitLabel={({ pick }) =>
        pick.length > 0
          ? t("builder.bank.add", { count: pick.length })
          : t("builder.bank.addNone")
      }
      disabled={!ready}
      onSubmit={({ pick }) => {
        const chosen = new Set(pick);
        onPick(items.filter((question) => chosen.has(question.id)).map((q) => q.id));
        onOpenChange(false);
      }}
    >
      {bank.isPending ? (
        <p className="text-muted-fg text-sm" role="status" aria-live="polite">
          {t("common.loading")}
        </p>
      ) : null}
      {bank.isError ? (
        <div role="alert" className="flex items-center gap-3 text-sm">
          <span className="text-danger-ink flex-1">{t("builder.bankFailed")}</span>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => void bank.refetch()}
          >
            {t("common.retry")}
          </Button>
        </div>
      ) : null}
      {bank.isSuccess && items.length === 0 ? (
        <p className="text-muted-fg text-sm">
          {t(
            bank.data.items.length === 0 ? "builder.bankEmpty" : "builder.bankAllAdded",
          )}
        </p>
      ) : null}
      {onPickGroup ? (
        <Button
          type="button"
          variant="outline"
          className="self-start"
          onClick={onPickGroup}
        >
          {t("builder.chooseWholeGroup")}
        </Button>
      ) : null}
    </FormDialog>
  );
}

function questionMeta(question: AdminQuestion, t: TFunction): string {
  return [
    t(`questionEditor.type.${question.type}`),
    question.level ? t(`bank.level.${question.level}`) : null,
    t("builder.bank.usedIn", { count: question.usedInTests ?? 0 }),
  ]
    .filter(Boolean)
    .join(" · ");
}
