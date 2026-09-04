import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { PageAside } from "@/components/shared/PageAside";
import { Separator } from "@/components/ui/separator";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import type { MediaAsset } from "@/features/media/api";
import { DEFAULT_AUDIO_POLICY } from "@/features/question-bank/audioPolicy";
import { AudioPolicyPanel } from "@/features/question-bank/components/AudioPolicyPanel";
import { BlanksEditor } from "@/features/question-bank/components/BlanksEditor";
import { QuestionMediaField } from "@/features/question-bank/components/QuestionMediaField";
import { OptionsEditor } from "@/features/question-bank/components/OptionsEditor";
import { PromptField } from "@/features/question-bank/components/PromptField";
import { TagsField } from "@/features/question-bank/components/TagsField";
import type {
  QuestionType,
  QuestionValues,
} from "@/features/question-bank/questionSchema";

const TYPES: QuestionType[] = [
  "single_choice",
  "multiple_choice",
  "true_false",
  "fill_blank",
  "short_answer",
];

const CHOICE_TYPES = new Set<QuestionType>([
  "single_choice",
  "multiple_choice",
  "true_false",
]);

interface QuestionEditorProps {
  value: QuestionValues;
  asset: MediaAsset | null;
  /** "Câu 2 · Phần 1" when the builder hosts this; absent on the bank's own page. */
  contextLabel?: string | null;
  onChange: (value: QuestionValues) => void;
  onAssetChange: (asset: MediaAsset | null) => void;
  /** Refetches the question so an expired media URL can be replaced. */
  onRefresh?: (() => void) | undefined;
  /** The builder has no room for a third column below 1024px, so it takes the panel as a sheet. */
  settings?: { hideBelow: "lg"; open: boolean; onOpenChange: (open: boolean) => void };
}

/**
 * §7's five question types in one editor, laid out as the deck's A-04: the
 * prompt and the answer in the middle column, everything about the question in
 * the settings rail.
 */
export function QuestionEditor({
  value,
  asset,
  onRefresh,
  contextLabel = null,
  settings,
  onChange,
  onAssetChange,
}: QuestionEditorProps) {
  const { t } = useTranslation();
  const isChoice = CHOICE_TYPES.has(value.type);
  const isAudio = asset?.kind === "audio";

  function switchType(type: QuestionType) {
    onChange({
      ...value,
      type,
      options: defaultOptionsFor(type, value.options),
      blanks: type === "fill_blank" ? value.blanks : [],
      sampleAnswer: type === "short_answer" ? value.sampleAnswer : null,
    });
  }

  return (
    <>
      <div className="space-y-5">
        <div className="flex items-center gap-2">
          {contextLabel === null ? null : (
            <span className="text-muted-foreground text-xs">{contextLabel}</span>
          )}
          <Tabs
            className="ml-auto"
            value={value.type}
            onValueChange={(next) => switchType(next as QuestionType)}
          >
            <TabsList aria-label={t("questionEditor.typeLabel")}>
              {TYPES.map((type) => (
                <TabsTrigger key={type} value={type}>
                  {t(`questionEditor.type.${type}`)}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
        </div>

        <div>
          <label
            className="mb-1.5 block text-[0.8125rem] font-medium"
            htmlFor="question-prompt"
          >
            {t("questionEditor.prompt")}
          </label>
          <PromptField
            id="question-prompt"
            value={value.prompt}
            onChange={(prompt) => onChange({ ...value, prompt })}
          />
        </div>

        {isChoice ? (
          <OptionsEditor
            options={value.options}
            multiple={value.type === "multiple_choice"}
            fixed={value.type === "true_false"}
            onChange={(options) => onChange({ ...value, options })}
          />
        ) : null}

        {value.type === "fill_blank" ? (
          <BlanksEditor
            prompt={value.prompt}
            blanks={value.blanks}
            onChange={(blanks) => onChange({ ...value, blanks })}
          />
        ) : null}

        {value.type === "short_answer" ? (
          <div>
            <label
              className="mb-1.5 block text-[0.8125rem] font-medium"
              htmlFor="question-sample-answer"
            >
              {t("questionEditor.sampleAnswer")}{" "}
              <span className="text-muted-foreground font-normal">
                {t("questionEditor.sampleAnswerHint")}
              </span>
            </label>
            <Textarea
              id="question-sample-answer"
              value={value.sampleAnswer ?? ""}
              className="min-h-14"
              onChange={(event) =>
                onChange({ ...value, sampleAnswer: event.target.value })
              }
            />
          </div>
        ) : null}

        <div>
          <label
            className="mb-1.5 block text-[0.8125rem] font-medium"
            htmlFor="question-explanation"
          >
            {t("questionEditor.explanation")}{" "}
            <span className="text-muted-foreground font-normal">
              {t("questionEditor.explanationHint")}
            </span>
          </label>
          <Textarea
            id="question-explanation"
            value={value.explanation ?? ""}
            className="min-h-14"
            onChange={(event) =>
              onChange({ ...value, explanation: event.target.value })
            }
          />
        </div>
      </div>

      <PageAside
        label={t("questionEditor.settings")}
        {...(settings === undefined
          ? {}
          : {
              hideBelow: settings.hideBelow,
              sheet: { open: settings.open, onOpenChange: settings.onOpenChange },
            })}
      >
        <div>
          <p className="text-muted-foreground mb-3 text-xs font-medium tracking-wide uppercase">
            {t("questionEditor.settings")}
          </p>
          <div className="space-y-3">
            <div>
              <label
                className="mb-1.5 block text-[0.8125rem] font-medium"
                htmlFor="question-points"
              >
                {t("questionEditor.points")}
              </label>
              <Input
                id="question-points"
                type="number"
                min={0.01}
                step={0.5}
                value={value.points}
                aria-invalid={value.points <= 0}
                onChange={(event) =>
                  onChange({ ...value, points: Number(event.target.value) })
                }
              />
              {value.points <= 0 ? (
                <p role="alert" className="text-destructive mt-1.5 text-xs">
                  {t("questionEditor.pointsError")}
                </p>
              ) : null}
            </div>

            <TagsField
              tags={value.tags}
              onChange={(tags) => onChange({ ...value, tags })}
            />
          </div>
        </div>

        <Separator />

        <div>
          <p className="text-muted-foreground mb-3 text-xs font-medium tracking-wide uppercase">
            {t("questionEditor.media")}
          </p>
          <QuestionMediaField
            value={asset}
            {...(onRefresh ? { onRefresh } : {})}
            onChange={(next) => {
              onAssetChange(next);
              onChange({
                ...value,
                mediaAssetId: next?.id ?? null,
                audio:
                  next?.kind === "audio" ? (value.audio ?? DEFAULT_AUDIO_POLICY) : null,
                transcript: next?.kind === "audio" ? value.transcript : null,
              });
            }}
          />

          {isAudio && value.audio ? (
            <AudioPolicyPanel
              policy={value.audio}
              transcript={value.transcript ?? ""}
              onPolicyChange={(audio) => onChange({ ...value, audio })}
              onTranscriptChange={(transcript) => onChange({ ...value, transcript })}
            />
          ) : null}
        </div>

        <Separator />

        <p className="text-muted-foreground text-xs leading-relaxed">
          {value.type === "short_answer"
            ? t("questionEditor.manualGraded")
            : t("questionEditor.autoGraded")}
        </p>
      </PageAside>
    </>
  );
}

// true_false has exactly two options a teacher never renames; the other choice
// types keep whatever was already typed so switching between them is free.
function defaultOptionsFor(
  type: QuestionType,
  current: QuestionValues["options"],
): QuestionValues["options"] {
  if (type === "true_false") {
    return [
      { id: null, text: "True", isCorrect: current[0]?.isCorrect ?? true },
      { id: null, text: "False", isCorrect: current[1]?.isCorrect ?? false },
    ];
  }
  if (!CHOICE_TYPES.has(type)) return [];
  if (current.length > 0) return current;
  return [
    { id: null, text: "", isCorrect: true },
    { id: null, text: "", isCorrect: false },
  ];
}
