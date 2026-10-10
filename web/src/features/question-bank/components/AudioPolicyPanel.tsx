import { useId } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import type { AudioPolicy } from "@/features/question-bank/audioPolicy";
import { cn } from "@/lib/utils";

type Plays = AudioPolicy["maxPlays"];

const DRAWN: readonly Plays[] = [1, 2, null];

interface AudioPolicyPanelProps {
  policy: AudioPolicy;
  transcript: string;
  onPolicyChange: (policy: AudioPolicy) => void;
  onTranscriptChange: (transcript: string) => void;
}

/**
 * AudioPolicyPanel is the listening settings of a question's audio, as the
 * media block draws them under the player: "Plays" as a radiogroup of Once,
 * Twice and Unlimited, with a stored count outside the three shown as a
 * fourth option while it is the value (DG-111); "Allow skipping ahead" and
 * "Show transcript after submitting"; and the optional transcript.
 */
export function AudioPolicyPanel({
  policy,
  transcript,
  onPolicyChange,
  onTranscriptChange,
}: Readonly<AudioPolicyPanelProps>) {
  const { t } = useTranslation();
  const playsLabel = useId();
  const name = useId();
  const choices = DRAWN.includes(policy.maxPlays) ? DRAWN : [...DRAWN, policy.maxPlays];
  return (
    <>
      <div className="flex flex-wrap items-center gap-x-4.5 gap-y-2.5 border-t px-3 py-2.5">
        <span className="flex items-center gap-2 text-[12.5px]">
          <span id={playsLabel}>{t("questionEditor.questionMedia.plays")}</span>
          <span
            role="radiogroup"
            aria-labelledby={playsLabel}
            className="bg-muted flex gap-0.5 rounded-[7px] p-0.5"
          >
            {choices.map((plays) => {
              const on = plays === policy.maxPlays;
              return (
                <label
                  key={String(plays)}
                  className={cn(
                    "has-[:focus-visible]:outline-focus flex h-6.5 cursor-pointer items-center rounded-[5px] px-2.25 text-xs font-medium whitespace-nowrap has-[:focus-visible]:outline-2",
                    on ? "bg-card text-fg shadow-card" : "text-muted-fg",
                  )}
                >
                  <input
                    type="radio"
                    name={name}
                    className="sr-only"
                    checked={on}
                    onChange={() => onPolicyChange({ ...policy, maxPlays: plays })}
                  />
                  {playsName(plays, t)}
                </label>
              );
            })}
          </span>
        </span>
        <label className="flex cursor-pointer items-center gap-2 text-[12.5px]">
          <Switch
            checked={policy.allowSeek}
            onCheckedChange={(allowSeek) => onPolicyChange({ ...policy, allowSeek })}
          />
          {t("questionEditor.questionMedia.allowSeek")}
        </label>
        <label className="flex cursor-pointer items-center gap-2 text-[12.5px]">
          <Switch
            checked={policy.showTranscriptAfterSubmit}
            onCheckedChange={(showTranscriptAfterSubmit) =>
              onPolicyChange({ ...policy, showTranscriptAfterSubmit })
            }
          />
          {t("questionEditor.questionMedia.showTranscript")}
        </label>
      </div>
      <div className="px-3 pb-3">
        <label
          htmlFor={`${name}-transcript`}
          className="flex flex-col text-[12.5px] font-medium"
        >
          {t("questionEditor.questionMedia.transcript")}
          <span className="text-muted-fg font-normal">
            {t("questionEditor.questionMedia.optional")}
          </span>
        </label>
        <Textarea
          id={`${name}-transcript`}
          rows={2}
          value={transcript}
          placeholder={t("questionEditor.questionMedia.transcriptPlaceholder")}
          className="bg-background mt-1.5 min-h-0 resize-y rounded-[8px] px-2.5 py-2 leading-normal in-data-[scale=deck]:rounded-[8px]"
          onChange={(event) => onTranscriptChange(event.target.value)}
        />
      </div>
    </>
  );
}

function playsName(plays: Plays, t: TFunction): string {
  if (plays === null) return t("questionEditor.questionMedia.unlimited");
  if (plays === 1) return t("questionEditor.questionMedia.once");
  if (plays === 2) return t("questionEditor.questionMedia.twice");
  return t("questionEditor.questionMedia.playsCount", { count: plays });
}
