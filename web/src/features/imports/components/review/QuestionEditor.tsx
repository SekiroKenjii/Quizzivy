import {
  lazy,
  Suspense,
  useEffect,
  useId,
  useRef,
  useState,
  type RefObject,
} from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { Plus, Trash2 } from "lucide-react";
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
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { ContentView } from "@/components/shared/content/ContentView";
import type { SemanticContent } from "@/components/shared/content/model";
import { isOptionContent } from "@/components/shared/content/optionContent";
import { isQuestionContent } from "@/components/shared/content/questionContent";
import type {
  ImportDraftBlank,
  ImportDraftQuestion,
  ImportFieldOrigins,
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
  exclude,
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
import { FindingNotice } from "./FindingNotice";
import { Provenance } from "./Provenance";

const ContentEditor = lazy(() =>
  import("@/components/shared/content/editor/ContentEditor").then((module) => ({
    default: module.ContentEditor,
  })),
);

type Edit = (update: (question: ImportDraftQuestion) => ImportDraftQuestion) => void;
type Editing = { kind: "prompt" } | { kind: "option"; optionId: string } | null;

const FIELDS: readonly (keyof ImportFieldOrigins)[] = [
  "type",
  "prompt",
  "options",
  "answer",
  "points",
];

/**
 * QuestionEditor edits the selected question of the review. At most one rich
 * editor is mounted, for the field the teacher opened, and every field it
 * changes is marked teacher-entered. While `readOnly`, nothing can be changed
 * and any open rich editor is closed; locating and provenance still work.
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
  onReprocess: (paper: number) => void;
}>) {
  const { t } = useTranslation();
  const headingId = useId();
  const [opened, setEditing] = useState<Editing>(null);
  const [provenance, setProvenance] = useState(false);
  const [pendingType, setPendingType] = useState<QuestionType | null>(null);
  const typeTrigger = useRef<HTMLButtonElement>(null);
  const excluded = question.excluded !== undefined;
  const editing = readOnly || excluded ? null : opened;
  const [tooManyGaps, setTooManyGaps] = useState(false);
  const trueFalse: TrueFalseText = {
    truth: t("imports.review.trueOption"),
    falsity: t("imports.review.falseOption"),
  };
  const editingPrompt = editing?.kind === "prompt";

  const requestType = (type: QuestionType) => {
    if (dropsAnswerData(question, changeType(question, type, trueFalse)))
      setPendingType(type);
    else onEdit((current) => changeType(current, type, trueFalse));
  };

  return (
    <section
      data-question-id={question.id}
      aria-labelledby={headingId}
      tabIndex={-1}
      className="ring-ring/30 bg-card focus-visible:ring-ring space-y-5 rounded-lg border p-5 shadow-sm ring-2 outline-none"
    >
      <header className="space-y-1">
        <div className="flex flex-wrap items-center gap-2">
          <h3 id={headingId} className="text-base font-semibold">
            {t("imports.review.questionLabel", { label: question.label })}
          </h3>
          <span className="text-muted-foreground text-xs">{context}</span>
          <div className="ml-auto flex items-center gap-1">
            {question.source.length > 0 ? (
              <Button
                type="button"
                variant="ghost"
                size="xs"
                onClick={() => onLocate(question.source)}
              >
                {t("imports.review.showInSource")}
              </Button>
            ) : null}
            <Button
              type="button"
              variant="ghost"
              size="xs"
              aria-expanded={provenance}
              onClick={() => setProvenance((open) => !open)}
            >
              {t("imports.review.provenance")}
            </Button>
          </div>
        </div>
        {provenance ? (
          <dl className="grid grid-cols-[auto_1fr] items-center gap-x-3 gap-y-1.5 rounded-md border p-3 text-xs">
            {FIELDS.map((field) => (
              <div key={field} className="contents">
                <dt className="text-muted-foreground">{t(`imports.field.${field}`)}</dt>
                <dd>
                  <Provenance origin={question.origins[field]} />
                </dd>
              </div>
            ))}
          </dl>
        ) : null}
      </header>

      {findings.length > 0 ? (
        <div className="space-y-2">
          {findings.map((finding) => (
            <FindingNotice
              key={finding.id}
              finding={finding}
              current={finding.id === currentFindingId}
              readOnly={readOnly}
              onAcknowledge={onAcknowledge}
              onLocate={onLocate}
              onReprocess={onReprocess}
            >
              {finding.code === "CONFLICTING_ANSWER_KEYS" ||
              finding.code === "UNUSABLE_ANSWER_KEY" ? (
                <Candidates
                  question={question}
                  readOnly={readOnly}
                  roleOf={roleOf}
                  onLocate={onLocate}
                  onPick={(candidate) =>
                    onEdit((current) => pickCandidate(current, candidate) ?? current)
                  }
                />
              ) : null}
            </FindingNotice>
          ))}
        </div>
      ) : null}

      <ExclusionControl
        question={question}
        readOnly={readOnly}
        onExclude={(reason) => onEdit((current) => exclude(current, reason))}
        onRestore={() => onEdit(restore)}
      />

      <fieldset
        disabled={excluded || readOnly}
        className="space-y-5 disabled:opacity-60"
      >
        <div className="grid gap-4 sm:grid-cols-[minmax(0,1fr)_10rem]">
          <TypeField
            question={question}
            triggerRef={typeTrigger}
            onChange={requestType}
          />
          <PointsField question={question} onEdit={onEdit} />
        </div>

        <div className="space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm font-medium">{t("imports.field.prompt")}</span>
            <Provenance origin={question.origins.prompt} />
            <Button
              type="button"
              variant="ghost"
              size="xs"
              className="ml-auto"
              aria-expanded={editingPrompt}
              disabled={question.prompt.format !== "semantic_v1"}
              onClick={() => {
                setTooManyGaps(false);
                setEditing(editingPrompt ? null : { kind: "prompt" });
              }}
            >
              {editingPrompt
                ? t("imports.review.doneEditing")
                : t("imports.review.editPrompt")}
            </Button>
          </div>
          {editingPrompt && question.prompt.format === "semantic_v1" ? (
            <RichField
              key={`${question.id}:prompt`}
              document={question.prompt}
              label={t("imports.review.promptEditorLabel", { label: question.label })}
              profile={
                question.type === "fill_blank" || !isQuestionContent(question.prompt)
                  ? "prompt"
                  : "question"
              }
              onChange={(document) => {
                const over =
                  question.type === "fill_blank" && exceedsBlankLimit(document);
                setTooManyGaps(over);
                if (!over) onEdit((current) => setPrompt(current, document));
              }}
            />
          ) : (
            <ContentView
              document={question.prompt}
              className="rounded-md border p-3 text-sm"
            />
          )}
          {editingPrompt && tooManyGaps ? (
            <p role="alert" className="text-destructive text-xs">
              {t("imports.review.blankLimit", { max: BLANK_LIMIT })}
            </p>
          ) : null}
        </div>

        <AnswerFields
          question={question}
          editing={editing}
          onEditing={setEditing}
          onEdit={onEdit}
        />
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
    </section>
  );
}

function RichField({
  document,
  label,
  profile,
  onChange,
}: Readonly<{
  document: SemanticContent;
  label: string;
  profile: "question" | "prompt" | "option";
  onChange: (document: SemanticContent) => void;
}>) {
  const { t } = useTranslation();
  return (
    <Suspense
      fallback={
        <Skeleton className="h-32 w-full" aria-label={t("contentEditor.loading")} />
      }
    >
      <ContentEditor
        initialContent={document}
        label={label}
        profile={profile}
        onChange={onChange}
      />
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
    <div className="space-y-1.5">
      <div className="flex items-center gap-2">
        <Label htmlFor={id}>{t("imports.field.type")}</Label>
        <Provenance origin={question.origins.type} />
      </div>
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
    <div className="space-y-1.5">
      <div className="flex items-center gap-2">
        <Label htmlFor={id}>{t("imports.field.points")}</Label>
        <Provenance origin={question.origins.points} />
      </div>
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
        <p role="alert" className="text-destructive text-xs">
          {t("imports.review.pointsInvalid")}
        </p>
      ) : null}
    </div>
  );
}

type FocusTarget = "reason" | "toggle" | "restore";

function ExclusionControl({
  question,
  readOnly,
  onExclude,
  onRestore,
}: Readonly<{
  question: ImportDraftQuestion;
  readOnly: boolean;
  onExclude: (reason: string) => void;
  onRestore: () => void;
}>) {
  const { t } = useTranslation();
  const id = useId();
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("");
  const reasonField = useRef<HTMLTextAreaElement>(null);
  const toggle = useRef<HTMLButtonElement>(null);
  const restoreButton = useRef<HTMLButtonElement>(null);
  const focusNext = useRef<FocusTarget | null>(null);

  useEffect(() => {
    const target = focusNext.current;
    if (target === null) return;
    focusNext.current = null;
    if (target === "reason") reasonField.current?.focus();
    else if (target === "restore") restoreButton.current?.focus();
    else toggle.current?.focus();
  });

  if (question.excluded !== undefined)
    return (
      <div className="bg-muted/40 flex flex-wrap items-center gap-3 rounded-md border p-3 text-sm">
        <p className="min-w-0 flex-1">
          {t("imports.review.excludedBecause", { reason: question.excluded.reason })}
        </p>
        <Button
          ref={restoreButton}
          type="button"
          variant="outline"
          size="xs"
          disabled={readOnly}
          onClick={() => {
            focusNext.current = "toggle";
            onRestore();
          }}
        >
          {t("imports.review.restore")}
        </Button>
      </div>
    );
  if (!open)
    return (
      <div className="flex justify-end">
        <Button
          ref={toggle}
          type="button"
          variant="ghost"
          size="xs"
          disabled={readOnly}
          onClick={() => {
            focusNext.current = "reason";
            setOpen(true);
          }}
        >
          {t("imports.review.exclude")}
        </Button>
      </div>
    );
  return (
    <div className="space-y-2 rounded-md border p-3">
      <Label htmlFor={id}>{t("imports.review.excludeReason")}</Label>
      <Textarea
        ref={reasonField}
        id={id}
        value={reason}
        maxLength={500}
        disabled={readOnly}
        className="min-h-16"
        onChange={(event) => setReason(event.target.value)}
      />
      <p className="text-muted-foreground text-xs">{t("imports.review.excludeHint")}</p>
      <div className="flex justify-end gap-2">
        <Button
          type="button"
          variant="ghost"
          size="xs"
          onClick={() => {
            focusNext.current = "toggle";
            setOpen(false);
          }}
        >
          {t("common.cancel")}
        </Button>
        <Button
          type="button"
          size="xs"
          disabled={readOnly || reason.trim() === ""}
          onClick={() => {
            focusNext.current = "restore";
            onExclude(reason.trim());
            setOpen(false);
            setReason("");
          }}
        >
          {t("imports.review.excludeConfirm")}
        </Button>
      </div>
    </div>
  );
}

function AnswerFields({
  question,
  editing,
  onEditing,
  onEdit,
}: Readonly<{
  question: ImportDraftQuestion;
  editing: Editing;
  onEditing: (next: Editing) => void;
  onEdit: Edit;
}>) {
  const { t } = useTranslation();
  if (question.type === "unsupported")
    return <p className="text-sm">{t("imports.review.unsupportedHelp")}</p>;
  if (question.type === "fill_blank")
    return <BlanksField question={question} onEdit={onEdit} />;
  if (question.type === "short_answer")
    return <SampleField question={question} onEdit={onEdit} />;
  return (
    <OptionsField
      question={question}
      editing={editing}
      onEditing={onEditing}
      onEdit={onEdit}
    />
  );
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
  editing,
  onEditing,
  onEdit,
}: Readonly<{
  question: ImportDraftQuestion;
  editing: Editing;
  onEditing: (next: Editing) => void;
  onEdit: Edit;
}>) {
  const { t } = useTranslation();
  const multiple = question.type === "multiple_choice";
  const fixed = question.type === "true_false";
  const keys = new Set(question.answer.optionIds);
  const toggle = (optionId: string, on: boolean) =>
    onEdit((current) => setCorrectOptions(current, markedKeys(current, optionId, on)));
  return (
    <fieldset className="space-y-2">
      <legend className="flex w-full flex-wrap items-center gap-2 text-sm font-medium">
        <span>{t("imports.field.options")}</span>
        <Provenance origin={question.origins.options} />
        <span className="text-muted-foreground ml-auto text-xs font-normal">
          {multiple
            ? t("imports.review.markManyHint")
            : t("imports.review.markOneHint")}
        </span>
      </legend>
      <div className="flex flex-wrap items-center gap-2 text-xs">
        <span className="text-muted-foreground">{t("imports.field.answer")}</span>
        <Provenance origin={question.origins.answer} />
        {question.answer.state === "known" ? null : (
          <span className="text-muted-foreground">{answerGap(question, t)}</span>
        )}
      </div>
      <ul className="space-y-2">
        {question.options.map((option) => {
          const open = editing?.kind === "option" && editing.optionId === option.id;
          const editable = isOptionContent(option.content);
          return (
            <li key={option.id} className="flex items-start gap-2.5">
              <input
                type={multiple ? "checkbox" : "radio"}
                name={`correct-${question.id}`}
                checked={keys.has(option.id)}
                onChange={(event) => toggle(option.id, event.target.checked)}
                aria-label={t("imports.review.markCorrect", { label: option.label })}
                className="border-input accent-foreground mt-2.5 size-4 shrink-0"
              />
              <span className="text-muted-foreground mt-2 w-5 shrink-0 text-sm font-medium">
                {option.label}
              </span>
              <div className="min-w-0 flex-1">
                {open && option.content.format === "semantic_v1" && editable ? (
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
                  <ContentView
                    document={option.content}
                    className="rounded-md border px-3 py-2 text-sm"
                  />
                )}
              </div>
              <Button
                type="button"
                variant="ghost"
                size="xs"
                className="mt-1"
                aria-expanded={open}
                aria-label={
                  open
                    ? t("imports.review.doneOptionNamed", { label: option.label })
                    : t("imports.review.editOptionNamed", { label: option.label })
                }
                disabled={!editable}
                onClick={() =>
                  onEditing(open ? null : { kind: "option", optionId: option.id })
                }
              >
                {open
                  ? t("imports.review.doneEditing")
                  : t("imports.review.editOption")}
              </Button>
              {fixed ? null : (
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
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
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="text-muted-foreground"
          onClick={() => onEdit(addOption)}
        >
          <Plus aria-hidden="true" />
          {t("imports.review.addOption")}
        </Button>
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
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2 text-sm font-medium">
        <span>{t("imports.field.blanks")}</span>
        <Provenance origin={question.origins.answer} />
      </div>
      {question.blanks.length === 0 ? (
        <p className="text-muted-foreground text-sm">{t("imports.review.noBlanks")}</p>
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
    <div className="space-y-2 rounded-md border p-3">
      <Label htmlFor={id}>{t("imports.review.blankLabel", { label })}</Label>
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
        <p role="alert" className="text-destructive text-xs">
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
    <div className="space-y-1.5">
      <div className="flex items-center gap-2">
        <Label htmlFor={id}>{t("imports.field.sample")}</Label>
        <Provenance origin={question.origins.answer} />
      </div>
      <Textarea
        id={id}
        value={question.answer.text ?? ""}
        maxLength={10000}
        className="min-h-20"
        onChange={(event) =>
          onEdit((current) => setSample(current, event.target.value))
        }
      />
      <p className="text-muted-foreground text-xs">{t("imports.review.sampleHint")}</p>
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
  const candidates = question.answer.candidates ?? [];
  if (candidates.length === 0) return null;
  return (
    <ul className="space-y-1.5 pt-1" aria-label={t("imports.review.candidates")}>
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
          <li
            key={candidate.value}
            className="flex flex-wrap items-center gap-2 rounded-md border px-2.5 py-1.5"
          >
            <span className="font-medium">{candidate.value}</span>
            {roles.length > 0 ? (
              <span className="text-muted-foreground text-xs">
                {roles.map((role) => t(`imports.role.${role}`)).join(" · ")}
              </span>
            ) : null}
            <span className="ml-auto flex items-center gap-1.5">
              {candidate.evidence.length > 0 ? (
                <Button
                  type="button"
                  variant="ghost"
                  size="xs"
                  onClick={() => onLocate(candidate.evidence)}
                >
                  {t("imports.review.showEvidence")}
                </Button>
              ) : null}
              <Button
                type="button"
                variant="outline"
                size="xs"
                disabled={readOnly || !usable}
                aria-label={t("imports.review.useCandidateNamed", {
                  value: candidate.value,
                })}
                onClick={() => onPick(candidate)}
              >
                {t("imports.review.useCandidate")}
              </Button>
            </span>
            {usable ? null : (
              <p className="text-muted-foreground w-full text-xs">
                {t("imports.review.candidateUnusable")}
              </p>
            )}
          </li>
        );
      })}
    </ul>
  );
}
