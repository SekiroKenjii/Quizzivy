import type { RefObject } from "react";
import { BlankPromptField } from "./BlankPromptField";
import { AnswerArea } from "./AnswerArea";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { PageAside } from "@/components/shared/PageAside";
import { Separator } from "@/components/ui/separator";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { MediaAsset } from "@/features/media/api";
import { DEFAULT_AUDIO_POLICY } from "@/features/question-bank/audioPolicy";
import { AudioPolicyPanel } from "@/features/question-bank/components/AudioPolicyPanel";
import { QuestionMediaField } from "@/features/question-bank/components/QuestionMediaField";
import { QuestionProseField } from "@/features/question-bank/components/QuestionProseField";
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

interface QuestionEditorProps {
  value: QuestionValues;
  clearPromptOnFocus?: boolean;
  asset: MediaAsset | null;
  /** "Câu 2 · Phần 1" when the builder hosts this; absent on the bank's own page. */
  contextLabel?: string | null;
  onChange: (value: QuestionValues) => void;
  onAssetChange: (asset: MediaAsset | null) => void;
  /** Refetches the question so an expired media URL can be replaced. */
  onRefresh?: (() => void) | undefined;
  /** The host controls when question settings use its dialog. */
  settings?: {
    hideBelow: "lg";
    open: boolean;
    onOpenChange: (open: boolean) => void;
    always?: boolean;
    triggerRef?: RefObject<HTMLElement | null> | undefined;
  };
}

/**
 * QuestionEditor is §7's five question types in one controlled editor: the
 * prompt, the type's answer block with its grading note, and the explanation in
 * the middle column; points, tags and media in the settings rail, which a host
 * can present in its dialog instead.
 */
export function QuestionEditor({
  value,
  clearPromptOnFocus = false,
  asset,
  onRefresh,
  contextLabel = null,
  settings,
  onChange,
  onAssetChange,
}: Readonly<QuestionEditorProps>) {
  const { t } = useTranslation();
  const isAudio = asset?.kind === "audio";
  const locked = typeLocked(value);

  function switchType(type: QuestionType) {
    if (type === value.type || (type !== "fill_blank" && locked)) return;
    onChange(retype(value, type));
  }

  return (
    <>
      <div className="space-y-5">
        <div className="flex flex-wrap items-center gap-2">
          {contextLabel === null ? null : (
            <span className="text-muted-foreground text-xs">{contextLabel}</span>
          )}
          <Tabs
            className="ml-auto max-w-full"
            value={value.type}
            onValueChange={(next) => switchType(next as QuestionType)}
          >
            <TabsList
              className="h-auto flex-wrap justify-start"
              aria-label={t("questionEditor.typeLabel")}
            >
              {QUESTION_TYPES.map((type) => (
                <TabsTrigger
                  key={type}
                  value={type}
                  disabled={type !== "fill_blank" && locked}
                >
                  {t(`questionEditor.type.${type}`)}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
        </div>

        {locked && (
          <p className="text-muted-foreground text-xs">
            {t("questionEditor.richBlankSwitch")}
          </p>
        )}
        <div>
          {value.type === "fill_blank" ? (
            <BlankPromptField value={value} onChange={onChange} />
          ) : (
            <QuestionProseField
              id="question-prompt"
              text={value.prompt}
              content={value.promptContent}
              label={t("questionEditor.prompt")}
              prompt
              clearOnFocus={clearPromptOnFocus}
              onChange={(prompt, promptContent) =>
                onChange({ ...value, prompt, promptContent })
              }
            />
          )}
        </div>

        <AnswerArea value={value} onChange={onChange} />

        <div>
          <QuestionProseField
            id="question-explanation"
            text={value.explanation ?? ""}
            content={value.explanationContent}
            label={t("questionEditor.explanation")}
            hint={t("questionEditor.explanationHint")}
            onChange={(explanation, explanationContent) =>
              onChange({ ...value, explanation, explanationContent })
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
              sheet: {
                open: settings.open,
                onOpenChange: settings.onOpenChange,
                always: settings.always ?? false,
                triggerRef: settings.triggerRef,
              },
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
      </PageAside>
    </>
  );
}
