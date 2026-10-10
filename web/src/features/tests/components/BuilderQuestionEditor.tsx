import { useTranslation } from "react-i18next";
import { ChevronDown, Copy, FolderInput, Trash2 } from "lucide-react";
import { RowMenu } from "@/components/shared/RowMenu";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuItemText,
  DropdownMenuLabel,
  DropdownMenuMeta,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import type { MediaAsset } from "@/features/media/api";
import { QuestionMediaField } from "@/features/question-bank/components/QuestionMediaField";
import {
  ExplanationField,
  QuestionPromptAnswers,
} from "@/features/question-bank/components/QuestionPromptAnswers";
import { QUESTION_TYPE_ICONS } from "@/features/question-bank/components/questionTypeIcons";
import { TagsField } from "@/features/question-bank/components/TagsField";
import type {
  QuestionType,
  QuestionValues,
} from "@/features/question-bank/questionSchema";
import {
  QUESTION_TYPES,
  retype,
  typeLocked,
} from "@/features/question-bank/questionType";

/** BuilderQuestionActions are the "…" menu's three commands, carried out by the builder. */
export type BuilderQuestionActions = Readonly<{
  onDuplicate: () => void;
  onMove: () => void;
  onDelete: () => void;
}>;

/**
 * BuilderQuestionEditor is the builder's editor pane for one standalone
 * question, in the deck's order: a header with "Question {n}", the type menu,
 * Points and the "…" menu (Duplicate, Move to section, Delete question); then
 * the prompt, the answer area with its grading note, the question media, and
 * "More options" holding the explanation above the tags. The type menu keeps
 * the other types disabled while gaps bind answers, as the bank does.
 */
export function BuilderQuestionEditor({
  value,
  asset,
  number,
  clearPromptOnFocus,
  actions,
  onChange,
  onAssetChange,
  onRefresh,
}: Readonly<{
  value: QuestionValues;
  asset: MediaAsset | null;
  number: number | null;
  clearPromptOnFocus: boolean;
  actions: BuilderQuestionActions;
  onChange: (value: QuestionValues) => void;
  onAssetChange: (asset: MediaAsset | null) => void;
  onRefresh?: (() => void) | undefined;
}>) {
  const { t } = useTranslation();
  return (
    <div className="flex min-w-0 flex-col">
      <div className="flex min-w-0 items-center gap-2.5 border-b px-4 py-3">
        <h2 className="shrink-0 text-[13px] font-semibold whitespace-nowrap">
          {number === null
            ? t("builder.editor.question")
            : t("builder.editor.questionNumber", { number })}
        </h2>
        <TypeMenu value={value} onChange={onChange} />
        <label className="text-muted-fg ml-auto flex shrink-0 items-center gap-2 text-[12.5px] whitespace-nowrap">
          {t("questionEditor.points")}
          <Input
            type="number"
            min={0.01}
            step={0.5}
            value={value.points}
            aria-invalid={value.points <= 0}
            className="bg-background h-7.5 w-12 rounded-[7px] px-0.5 text-center in-data-[scale=deck]:h-7.5 in-data-[scale=deck]:rounded-[7px] in-data-[scale=deck]:px-0.5 in-data-[scale=deck]:lg:text-[13px]"
            onChange={(event) =>
              onChange({ ...value, points: Number(event.target.value) })
            }
          />
        </label>
        <RowMenu label={t("builder.editor.more")} className="w-57.5">
          <DropdownMenuItem onSelect={actions.onDuplicate}>
            <Copy aria-hidden="true" />
            {t("builder.editor.duplicate")}
          </DropdownMenuItem>
          <DropdownMenuItem onSelect={actions.onMove}>
            <FolderInput aria-hidden="true" />
            {t("builder.editor.move")}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem variant="destructive" onSelect={actions.onDelete}>
            <Trash2 aria-hidden="true" />
            {t("builder.editor.delete")}
          </DropdownMenuItem>
        </RowMenu>
      </div>
      {value.points <= 0 ? (
        <p role="alert" className="text-danger-ink border-b px-4 py-2 text-xs">
          {t("questionEditor.pointsError")}
        </p>
      ) : null}

      <div className="flex min-w-0 flex-col gap-4.5 px-4 py-4.5">
        <QuestionPromptAnswers
          value={value}
          clearPromptOnFocus={clearPromptOnFocus}
          onChange={onChange}
        />

        <QuestionMediaField
          value={value}
          asset={asset}
          onChange={onChange}
          onAssetChange={onAssetChange}
          onRefresh={onRefresh}
        />

        <details className="group/more border-t pt-3.5">
          <summary className="cursor-pointer rounded-[6px] text-[13px] font-medium">
            {t("builder.editor.moreOptions")}
          </summary>
          <div className="mt-3 flex flex-col gap-3.5">
            <ExplanationField value={value} onChange={onChange} />
            <TagsField
              tags={value.tags}
              onChange={(tags) => onChange({ ...value, tags })}
            />
          </div>
        </details>
      </div>
    </div>
  );
}

function TypeMenu({
  value,
  onChange,
}: Readonly<{ value: QuestionValues; onChange: (value: QuestionValues) => void }>) {
  const { t } = useTranslation();
  const locked = typeLocked(value);
  const Current = QUESTION_TYPE_ICONS[value.type];
  const typeName = t(`questionEditor.type.${value.type}`);
  function choose(type: QuestionType) {
    if (type === value.type) return;
    onChange(retype(value, type));
  }
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          type="button"
          variant="outline"
          size="sm"
          aria-label={`${t("questionEditor.typeLabel")}: ${typeName}`}
          className="bg-card h-7.5 min-w-0 shrink gap-1.5 rounded-[7px] px-2.5 text-[12.5px] font-normal in-data-[scale=deck]:h-7.5 in-data-[scale=deck]:rounded-[7px] in-data-[scale=deck]:text-[12.5px]"
        >
          <Current aria-hidden="true" className="size-3.25" />
          <span className="truncate">{typeName}</span>
          <ChevronDown aria-hidden="true" className="text-muted-fg size-3.25" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-62.5 data-[scale=deck]:w-62.5">
        <DropdownMenuLabel>{t("questionEditor.typeLabel")}</DropdownMenuLabel>
        {QUESTION_TYPES.map((type) => {
          const Icon = QUESTION_TYPE_ICONS[type];
          const current = type === value.type;
          const blocked = !current && locked && type !== "fill_blank";
          return (
            <DropdownMenuItem
              key={type}
              disabled={blocked}
              onSelect={() => choose(type)}
            >
              <Icon aria-hidden="true" />
              <DropdownMenuItemText>
                {t(`questionEditor.type.${type}`)}
              </DropdownMenuItemText>
              {current ? (
                <DropdownMenuMeta>{t("builder.editor.current")}</DropdownMenuMeta>
              ) : null}
              {blocked ? (
                <DropdownMenuMeta>
                  {t("builder.editor.removeGapsFirst")}
                </DropdownMenuMeta>
              ) : null}
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
