import { lazy, Suspense, useId, useRef, useState, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { CircleSlash, Ellipsis, FileSearch, Plus, Trash2, Undo2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { ContentView } from "@/components/shared/content/ContentView";
import type { SemanticContent } from "@/components/shared/content/model";
import { isOptionContent } from "@/components/shared/content/optionContent";
import { isQuestionContent } from "@/components/shared/content/questionContent";
import { cn } from "@/lib/utils";
import type {
  ImportDraftBlank,
  ImportDraftQuestion,
  ImportFinding,
  ImportKeyValue,
  ImportSourceRef,
  ImportSourceRole,
} from "../../api";
import {
  ACCEPTED_LIMITS,
  addOption,
  changeType,
  clampAccepted,
  dropsAnswerData,
  EDITABLE_TYPES,
  exceedsBlankLimit,
  BLANK_LIMIT,
  pickCandidate,
  removeOption,
  restore,
  setBlanks,
  setCorrectOptions,
  setOptionContent,
  setPoints,
  setPrompt,
  setSample,
  validPoints,
  type QuestionType,
  type TrueFalseText,
} from "../../draft";
import { FINDING_ACTION, FindingNotice } from "./FindingNotice";
import { Provenance } from "./Provenance";

const ContentEditor = lazy(() =>
  import("@/components/shared/content/editor/ContentEditor").then((module) => ({
    default: module.ContentEditor,
  })),
);

type Edit = (update: (question: ImportDraftQuestion) => ImportDraftQuestion) => void;

const SMALL_BUTTON =
  "h-7 rounded-[7px] px-2.25 text-xs font-medium shadow-none in-data-[scale=deck]:h-7 in-data-[scale=deck]:px-2.25 in-data-[scale=deck]:text-xs";

/**
 * QuestionEditor is the body of the open question card: where it was printed
 * with "Show in source" and its actions (Exclude or Restore), the exclusion
 * and its reason, its findings with their resolutions, and its fields. The
 * "Question" field is the content editor, mounted only here, since one card
 * is open at a time. While `readOnly`, and for an excluded question, every
 * field is its read view; locating and the provenance chips still work.
 * `onExclude` names the element the exclusion dialog returns focus to: the
 * card's actions button when the menu asked, since its item unmounts, or
 * null when a button that stays asked.
 */
export function QuestionEditor({
  question,
  context,
  findings,
  currentFindingId,
  readOnly,
  roleOf,
  onEdit,
  onAcknowledge,
  onLocate,
  onReprocess,
  onExclude,
}: Readonly<{
  question: ImportDraftQuestion;
  context: string;
  findings: readonly ImportFinding[];
  currentFindingId: string | null;
  readOnly: boolean;
  roleOf: (sourceId: string) => ImportSourceRole | null;
  onEdit: Edit;
  onAcknowledge: (findingId: string, on: boolean) => void;
  onLocate: (refs: readonly ImportSourceRef[]) => void;
  onReprocess?: ((paper: number) => void) | undefined;
  onExclude: (questionId: string, returnTo: HTMLElement | null) => void;
}>) {
  const { t } = useTranslation();
  const menuTrigger = useRef<HTMLButtonElement>(null);
  const [pendingType, setPendingType] = useState<QuestionType | null>(null);
  const typeTrigger = useRef<HTMLButtonElement>(null);
  const excluded = question.excluded !== undefined;
  const locked = readOnly || excluded;
  const trueFalse: TrueFalseText = {
    truth: t("imports.review.trueOption"),
    falsity: t("imports.review.falseOption"),
  };

  const requestType = (type: QuestionType) => {
    if (dropsAnswerData(question, changeType(question, type, trueFalse)))
      setPendingType(type);
    else onEdit((current) => changeType(current, type, trueFalse));
  };

  return (
    <div className="flex min-w-0 flex-col gap-3.5 border-t px-3.5 pt-1 pb-4">
      <div className="flex flex-wrap items-center gap-2 pt-2">
        <span className="text-muted-fg min-w-0 flex-[1_1_200px] text-xs">
          {t("imports.review.printedAs", { label: question.label, context })}
        </span>
        {question.source.length > 0 ? (
          <Button
            type="button"
            variant="outline"
            className={SMALL_BUTTON}
            onClick={() => onLocate(question.source)}
          >
            <FileSearch aria-hidden="true" className="size-3.25" />
            {t("imports.review.showInSource")}
          </Button>
        ) : null}
        <QuestionMenu
          question={question}
          disabled={readOnly}
          triggerRef={menuTrigger}
          onExclude={() => onExclude(question.id, menuTrigger.current)}
          onRestore={() => onEdit(restore)}
        />
      </div>

      {excluded ? (
        <div className="bg-muted flex flex-wrap items-center gap-2.5 rounded-[10px] px-3 py-2.5 text-sm">
          <CircleSlash
            aria-hidden="true"
            className="text-muted-fg size-3.75 flex-none"
          />
          <span className="min-w-0 flex-[1_1_200px] [overflow-wrap:anywhere]">
            {t("imports.review.excludedBecause", {
              reason: question.excluded?.reason ?? "",
            })}
          </span>
          {readOnly ? null : (
            <Button
              type="button"
              variant="outline"
              className={SMALL_BUTTON}
              onClick={() => onEdit(restore)}
            >
              {t("imports.review.restore")}
            </Button>
          )}
        </div>
      ) : null}

      {findings.map((finding) => (
        <FindingNotice
          key={finding.id}
          finding={finding}
          current={finding.id === currentFindingId}
          readOnly={readOnly}
          onAcknowledge={onAcknowledge}
          onLocate={onLocate}
          onReprocess={onReprocess}
          actions={
            locked ? null : (
              <FindingExclude
                finding={finding}
                onExclude={() => onExclude(question.id, null)}
              />
            )
          }
        >
          {finding.code === "CONFLICTING_ANSWER_KEYS" ||
          finding.code === "UNUSABLE_ANSWER_KEY" ? (
            <Candidates
              question={question}
              readOnly={locked}
              roleOf={roleOf}
              onLocate={onLocate}
              onPick={(candidate) =>
                onEdit((current) => pickCandidate(current, candidate) ?? current)
              }
            />
          ) : null}
        </FindingNotice>
      ))}

      <fieldset
        disabled={locked}
        className="m-0 flex min-w-0 flex-col gap-3.5 border-0 p-0"
      >
        <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,200px),1fr))] gap-2.5">
          <TypeField
            question={question}
            triggerRef={typeTrigger}
            onChange={requestType}
          />
          <PointsField key={question.id} question={question} onEdit={onEdit} />
        </div>
        <PromptField question={question} editable={!locked} onEdit={onEdit} />
        <AnswerFields question={question} locked={locked} onEdit={onEdit} />
      </fieldset>

      <ConfirmDialog
        open={pendingType !== null}
        onOpenChange={(open) => !open && setPendingType(null)}
        title={t("imports.review.typeChangeTitle")}
        description={t("imports.review.typeChangeBody", {
          from: t(`imports.type.${question.type}`),
          to: pendingType === null ? "" : t(`imports.type.${pendingType}`),
        })}
        confirmLabel={t("imports.review.typeChangeConfirm")}
        returnFocus={typeTrigger}
        onConfirm={() => {
          const type = pendingType;
          setPendingType(null);
          if (type !== null) onEdit((current) => changeType(current, type, trueFalse));
        }}
      />
    </div>
  );
}

function FindingExclude({
  finding,
  onExclude,
}: Readonly<{ finding: ImportFinding; onExclude: () => void }>) {
  const { t } = useTranslation();
  if (finding.code === "UNSUPPORTED_INTERACTION")
    return (
      <Button
        type="button"
        className={cn(FINDING_ACTION, "bg-primary text-primary-fg")}
        onClick={onExclude}
      >
        <CircleSlash aria-hidden="true" className="size-3.25" />
        {t("imports.review.excludeWithReason")}
      </Button>
    );
  if (finding.code === "MISSING_ANSWER")
    return (
      <Button
        type="button"
        variant="outline"
        className={FINDING_ACTION}
        onClick={onExclude}
      >
        <CircleSlash aria-hidden="true" className="size-3.25" />
        {t("imports.review.excludeQuestion")}
      </Button>
    );
  return null;
}

function QuestionMenu({
  question,
  disabled,
  triggerRef,
  onExclude,
  onRestore,
}: Readonly<{
  question: ImportDraftQuestion;
  disabled: boolean;
  triggerRef: RefObject<HTMLButtonElement | null>;
  onExclude: () => void;
  onRestore: () => void;
}>) {
  const { t } = useTranslation();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild disabled={disabled}>
        <button
          ref={triggerRef}
          type="button"
          aria-label={t("imports.review.questionActions", { label: question.label })}
          className="bg-card hover:bg-muted data-[state=open]:bg-muted grid size-7 flex-none cursor-pointer place-items-center rounded-[7px] border disabled:cursor-default disabled:opacity-45"
        >
          <Ellipsis aria-hidden="true" className="size-3.5" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-60">
        {question.excluded === undefined ? (
          <DropdownMenuItem variant="destructive" onSelect={onExclude}>
            <CircleSlash aria-hidden="true" />
            {t("imports.review.excludeMenu")}
          </DropdownMenuItem>
        ) : (
          <DropdownMenuItem onSelect={onRestore}>
            <Undo2 aria-hidden="true" />
            {t("imports.review.restoreQuestion")}
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function FieldLabel({
  htmlFor,
  id,
  label,
  origin,
}: Readonly<{
  htmlFor?: string;
  id?: string;
  label: string;
  origin: ImportDraftQuestion["origins"]["type"];
}>) {
  return (
    <span className="text-meta flex flex-wrap items-center gap-1.5 font-medium">
      {htmlFor === undefined ? (
        <span id={id}>{label}</span>
      ) : (
        <Label htmlFor={htmlFor} className="text-meta font-medium">
          {label}
        </Label>
      )}
      <Provenance origin={origin} />
    </span>
  );
}

function PromptField({
  question,
  editable,
  onEdit,
}: Readonly<{ question: ImportDraftQuestion; editable: boolean; onEdit: Edit }>) {
  const { t } = useTranslation();
  const labelId = useId();
  const [tooManyGaps, setTooManyGaps] = useState(false);
  const live = editable && question.prompt.format === "semantic_v1";
  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <FieldLabel
        id={labelId}
        label={t("imports.field.prompt")}
        origin={question.origins.prompt}
      />
      {live && question.prompt.format === "semantic_v1" ? (
        <RichField
          key={`${question.id}:prompt`}
          document={question.prompt}
          label={t("imports.review.promptEditorLabel", { label: question.label })}
          placeholder={t("imports.review.promptPlaceholder")}
          profile={
            question.type === "fill_blank" || !isQuestionContent(question.prompt)
              ? "prompt"
              : "question"
          }
          onChange={(document) => {
            const over = question.type === "fill_blank" && exceedsBlankLimit(document);
            setTooManyGaps(over);
            if (!over) onEdit((current) => setPrompt(current, document));
          }}
        />
      ) : (
        <div
          role="group"
          aria-labelledby={labelId}
          className="bg-card rounded-[10px] border px-4 py-3"
        >
          <ContentView document={question.prompt} className="text-base" />
        </div>
      )}
      {live && tooManyGaps ? (
        <p role="alert" className="text-danger-ink m-0 text-xs">
          {t("imports.review.blankLimit", { max: BLANK_LIMIT })}
        </p>
      ) : null}
    </div>
  );
}

function RichField({
  document,
  label,
  profile,
  placeholder,
  onChange,
}: Readonly<{
  document: SemanticContent;
  label: string;
  profile: "question" | "prompt" | "option";
  placeholder?: string;
  onChange: (document: SemanticContent) => void;
}>) {
  const { t } = useTranslation();
  return (
    <Suspense
      fallback={
        <Skeleton
          className="h-32 w-full rounded-[10px]"
          aria-label={t("contentEditor.loading")}
        />
      }
    >
      {profile === "option" ? (
        <ContentEditor
          initialContent={document}
          label={label}
          profile={profile}
          onChange={onChange}
        />
      ) : (
        <ContentEditor
          initialContent={document}
          label={label}
          profile={profile}
          minHeight={64}
          fontSize={14}
          placeholder={placeholder}
          onChange={onChange}
        />
      )}
    </Suspense>
  );
}

function TypeField({
  question,
  triggerRef,
  onChange,
}: Readonly<{
  question: ImportDraftQuestion;
  triggerRef: RefObject<HTMLButtonElement | null>;
  onChange: (type: QuestionType) => void;
}>) {
  const { t } = useTranslation();
  const id = useId();
  return (
    <div className="flex flex-col gap-1.5">
      <FieldLabel
        htmlFor={id}
        label={t("imports.field.type")}
        origin={question.origins.type}
      />
      <Select
        value={question.type}
        onValueChange={(value) => onChange(value as QuestionType)}
      >
        <SelectTrigger ref={triggerRef} id={id} className="w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            {question.type === "unsupported" ? (
              <SelectItem value="unsupported" disabled>
                {t("imports.type.unsupported")}
              </SelectItem>
            ) : null}
            {EDITABLE_TYPES.map((type) => (
              <SelectItem key={type} value={type}>
                {t(`imports.type.${type}`)}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    </div>
  );
}

function PointsField({
  question,
  onEdit,
}: Readonly<{ question: ImportDraftQuestion; onEdit: Edit }>) {
  const { t } = useTranslation();
  const id = useId();
  const [text, setText] = useState(question.points);
  const normalize = (value: string) => value.trim().replace(",", ".");
  const invalid = !validPoints(normalize(text));
  return (
    <div className="flex flex-col gap-1.5">
      <FieldLabel
        htmlFor={id}
        label={t("imports.field.points")}
        origin={question.origins.points}
      />
      <Input
        id={id}
        inputMode="decimal"
        value={text}
        maxLength={9}
        aria-invalid={invalid || undefined}
        onChange={(event) => {
          const next = normalize(event.target.value);
          setText(event.target.value);
          if (validPoints(next)) onEdit((current) => setPoints(current, next));
        }}
      />
      {invalid ? (
        <p role="alert" className="text-danger-ink m-0 text-xs">
          {t("imports.review.pointsInvalid")}
        </p>
      ) : null}
    </div>
  );
}

function AnswerFields({
  question,
  locked,
  onEdit,
}: Readonly<{ question: ImportDraftQuestion; locked: boolean; onEdit: Edit }>) {
  const { t } = useTranslation();
  if (question.type === "unsupported")
    return <p className="m-0 text-sm">{t("imports.review.unsupportedHelp")}</p>;
  if (question.type === "fill_blank")
    return <BlanksField question={question} onEdit={onEdit} />;
  if (question.type === "short_answer")
    return <SampleField question={question} onEdit={onEdit} />;
  return <OptionsField question={question} locked={locked} onEdit={onEdit} />;
}

function answerGap(question: ImportDraftQuestion, t: TFunction): string {
  if (question.answer.state === "conflict") return t("imports.review.answerConflict");
  return question.origins.answer === "teacher_entered"
    ? t("imports.review.answerNotSet")
    : t("imports.review.answerMissing");
}

function markedKeys(
  question: ImportDraftQuestion,
  optionId: string,
  on: boolean,
): string[] {
  if (question.type !== "multiple_choice") return on ? [optionId] : [];
  const keys = new Set(question.answer.optionIds);
  if (on) keys.add(optionId);
  else keys.delete(optionId);
  return question.options
    .filter((option) => keys.has(option.id))
    .map((option) => option.id);
}

function OptionsField({
  question,
  locked,
  onEdit,
}: Readonly<{ question: ImportDraftQuestion; locked: boolean; onEdit: Edit }>) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState<string | null>(null);
  const multiple = question.type === "multiple_choice";
  const fixed = question.type === "true_false";
  const keys = new Set(question.answer.optionIds);
  const open = locked ? null : editing;
  const toggle = (optionId: string, on: boolean) =>
    onEdit((current) => setCorrectOptions(current, markedKeys(current, optionId, on)));
  return (
    <fieldset className="m-0 flex flex-col gap-1.5 border-0 p-0">
      <legend className="text-meta mb-1.5 flex w-full flex-wrap items-center gap-1.5 p-0 font-medium">
        <span>{t("imports.field.options")}</span>
        <Provenance origin={question.origins.options} />
        <span className="text-muted-fg font-normal">
          {multiple
            ? t("imports.review.markManyHint")
            : t("imports.review.markOneHint")}
        </span>
      </legend>
      <div className="text-meta flex flex-wrap items-center gap-2">
        <span className="text-muted-fg">{t("imports.field.answer")}</span>
        <Provenance origin={question.origins.answer} />
        {question.answer.state === "known" ? null : (
          <span className="text-muted-fg">{answerGap(question, t)}</span>
        )}
      </div>
      <ul className="m-0 flex list-none flex-col gap-1.5 p-0">
        {question.options.map((option) => {
          const opened = open === option.id;
          const editable = isOptionContent(option.content);
          const correct = keys.has(option.id);
          return (
            <li
              key={option.id}
              className={cn(
                "flex items-start gap-2 rounded-[9px] border py-1 pr-2 pl-1",
                correct ? "border-success bg-success-soft" : "bg-card",
              )}
            >
              <span className="grid size-7.5 flex-none place-items-center">
                <input
                  type={multiple ? "checkbox" : "radio"}
                  name={`correct-${question.id}`}
                  checked={correct}
                  onChange={(event) => toggle(option.id, event.target.checked)}
                  aria-label={t("imports.review.markCorrect", { label: option.label })}
                  className="accent-success size-4"
                />
              </span>
              <span className="text-muted-fg text-meta mt-1.5 w-3.5 flex-none font-semibold">
                {option.label}
              </span>
              <div className="min-w-0 flex-1 py-0.5">
                {opened && option.content.format === "semantic_v1" && editable ? (
                  <RichField
                    key={`${question.id}:${option.id}`}
                    document={option.content}
                    label={t("imports.review.optionEditorLabel", {
                      label: option.label,
                    })}
                    profile="option"
                    onChange={(document) =>
                      onEdit((current) =>
                        setOptionContent(current, option.id, document),
                      )
                    }
                  />
                ) : (
                  <ContentView document={option.content} className="py-1 text-base" />
                )}
              </div>
              {correct ? (
                <span className="text-success-ink text-caption mt-1.5 font-semibold whitespace-nowrap">
                  {t("imports.review.correct")}
                </span>
              ) : null}
              <Button
                type="button"
                variant="ghost"
                className={cn(SMALL_BUTTON, "mt-0.5")}
                aria-expanded={opened}
                aria-label={
                  opened
                    ? t("imports.review.doneOptionNamed", { label: option.label })
                    : t("imports.review.editOptionNamed", { label: option.label })
                }
                disabled={!editable}
                onClick={() => setEditing(opened ? null : option.id)}
              >
                {opened
                  ? t("imports.review.doneEditing")
                  : t("imports.review.editOption")}
              </Button>
              {fixed ? null : (
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  className="mt-0.5"
                  aria-label={t("imports.review.removeOptionNamed", {
                    label: option.label,
                  })}
                  onClick={() => onEdit((current) => removeOption(current, option.id))}
                >
                  <Trash2 aria-hidden="true" />
                </Button>
              )}
            </li>
          );
        })}
      </ul>
      {fixed || question.options.length >= 8 ? null : (
        <div>
          <Button
            type="button"
            variant="ghost"
            className={cn(SMALL_BUTTON, "text-muted-fg")}
            onClick={() => onEdit(addOption)}
          >
            <Plus aria-hidden="true" className="size-3.25" />
            {t("imports.review.addOption")}
          </Button>
        </div>
      )}
    </fieldset>
  );
}

function BlanksField({
  question,
  onEdit,
}: Readonly<{ question: ImportDraftQuestion; onEdit: Edit }>) {
  const { t } = useTranslation();
  const update = (gapId: string, patch: Partial<ImportDraftBlank>) =>
    onEdit((current) =>
      setBlanks(
        current,
        current.blanks.map((blank) =>
          blank.gapId === gapId ? { ...blank, ...patch } : blank,
        ),
      ),
    );
  return (
    <div className="flex flex-col gap-2">
      <span className="text-meta flex flex-wrap items-center gap-1.5 font-medium">
        {t("imports.field.blanks")}
        <Provenance origin={question.origins.answer} />
      </span>
      {question.blanks.length === 0 ? (
        <p className="text-muted-fg m-0 text-sm">{t("imports.review.noBlanks")}</p>
      ) : null}
      {question.blanks.map((blank, index) => (
        <BlankField
          key={blank.gapId}
          blank={blank}
          label={blank.label ?? String(index + 1)}
          onChange={(patch) => update(blank.gapId, patch)}
        />
      ))}
    </div>
  );
}

function sameList(a: readonly string[], b: readonly string[]): boolean {
  return a.length === b.length && a.every((value, index) => value === b[index]);
}

function BlankField({
  blank,
  label,
  onChange,
}: Readonly<{
  blank: ImportDraftBlank;
  label: string;
  onChange: (patch: Partial<ImportDraftBlank>) => void;
}>) {
  const { t } = useTranslation();
  const id = useId();
  const [text, setText] = useState(() => blank.accepted.join("\n"));
  const lines = text.split("\n");
  const shown = sameList(clampAccepted(lines), blank.accepted)
    ? text
    : blank.accepted.join("\n");
  const entered = [...new Set(lines.map((line) => line.trim()).filter(Boolean))];
  const clipped =
    shown === text &&
    (entered.length > ACCEPTED_LIMITS.count ||
      entered.some((line) => Array.from(line).length > ACCEPTED_LIMITS.length));
  return (
    <div className="bg-card flex flex-col gap-2 rounded-[10px] border p-3">
      <Label htmlFor={id} className="text-meta font-medium">
        {t("imports.review.blankLabel", { label })}
      </Label>
      <Textarea
        id={id}
        value={shown}
        className="min-h-16"
        placeholder={t("imports.review.acceptedHint")}
        aria-invalid={clipped || undefined}
        onChange={(event) => {
          setText(event.target.value);
          onChange({ accepted: clampAccepted(event.target.value.split("\n")) });
        }}
      />
      {clipped ? (
        <p role="alert" className="text-danger-ink m-0 text-xs">
          {t("imports.review.acceptedLimit", {
            count: ACCEPTED_LIMITS.count,
            length: ACCEPTED_LIMITS.length,
          })}
        </p>
      ) : null}
      <div className="flex items-center justify-between gap-3 text-sm">
        <span>{t("imports.review.caseSensitive")}</span>
        <Switch
          checked={blank.caseSensitive}
          aria-label={t("imports.review.caseSensitiveFor", { label })}
          onCheckedChange={(checked) => onChange({ caseSensitive: checked })}
        />
      </div>
    </div>
  );
}

function SampleField({
  question,
  onEdit,
}: Readonly<{ question: ImportDraftQuestion; onEdit: Edit }>) {
  const { t } = useTranslation();
  const id = useId();
  return (
    <div className="flex flex-col gap-1.5">
      <FieldLabel
        htmlFor={id}
        label={t("imports.field.sample")}
        origin={question.origins.answer}
      />
      <Textarea
        id={id}
        value={question.answer.text ?? ""}
        maxLength={10000}
        className="min-h-20"
        onChange={(event) =>
          onEdit((current) => setSample(current, event.target.value))
        }
      />
      <p className="text-muted-fg m-0 text-xs">{t("imports.review.sampleHint")}</p>
    </div>
  );
}

function Candidates({
  question,
  readOnly,
  roleOf,
  onLocate,
  onPick,
}: Readonly<{
  question: ImportDraftQuestion;
  readOnly: boolean;
  roleOf: (sourceId: string) => ImportSourceRole | null;
  onLocate: (refs: readonly ImportSourceRef[]) => void;
  onPick: (candidate: ImportKeyValue) => void;
}>) {
  const { t } = useTranslation();
  const name = useId();
  const candidates = question.answer.candidates ?? [];
  if (candidates.length === 0) return null;
  return (
    <div
      role="radiogroup"
      aria-label={t("imports.review.candidates")}
      className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,200px),1fr))] gap-2"
    >
      {candidates.map((candidate) => {
        const usable = pickCandidate(question, candidate) !== null;
        const roles = [
          ...new Set(
            candidate.evidence
              .map((ref) => roleOf(ref.sourceId))
              .filter((role): role is ImportSourceRole => role !== null),
          ),
        ];
        return (
          <div
            key={candidate.value}
            className="bg-card flex flex-col gap-1.5 rounded-[9px] border-[1.5px] px-3 py-2.5"
          >
            <label className="flex cursor-pointer items-start gap-2.5 has-disabled:cursor-default">
              <input
                type="radio"
                name={name}
                checked={false}
                disabled={readOnly || !usable}
                aria-label={t("imports.review.useCandidateNamed", {
                  value: candidate.value,
                })}
                onChange={() => onPick(candidate)}
                className="accent-primary mt-0.5 size-4 flex-none"
              />
              <span className="min-w-0">
                <span className="text-ui block font-semibold [overflow-wrap:anywhere]">
                  {candidate.value}
                </span>
                <span className="text-muted-fg block text-xs leading-[1.45]">
                  {usable
                    ? roles.map((role) => t(`imports.role.${role}`)).join(" · ")
                    : t("imports.review.candidateUnusable")}
                </span>
              </span>
            </label>
            {candidate.evidence.length > 0 ? (
              <button
                type="button"
                onClick={() => onLocate(candidate.evidence)}
                className="text-fg ml-6.5 cursor-pointer self-start rounded-sm text-xs underline underline-offset-2"
              >
                {t("imports.review.viewInSource")}
              </button>
            ) : null}
          </div>
        );
      })}
    </div>
  );
}
