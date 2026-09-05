import { useTranslation } from "react-i18next";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import type { AudioPolicy } from "@/features/question-bank/audioPolicy";

const PLAY_CHOICES = [1, 2, 3, 5] as const;

interface AudioPolicyPanelProps {
  policy: AudioPolicy;
  transcript: string;
  onPolicyChange: (policy: AudioPolicy) => void;
  onTranscriptChange: (transcript: string) => void;
}

export function AudioPolicyPanel({
  policy,
  transcript,
  onPolicyChange,
  onTranscriptChange,
}: Readonly<AudioPolicyPanelProps>) {
  const { t } = useTranslation();

  return (
    <div className="space-y-3">
      <Separator />

      <div className="space-y-2.5">
        <div className="flex items-center justify-between gap-3">
          <label className="text-sm" htmlFor="audio-max-plays">
            {t("questionEditor.audioMaxPlays")}
          </label>
          <Select
            value={policy.maxPlays === null ? "unlimited" : String(policy.maxPlays)}
            onValueChange={(next) =>
              onPolicyChange({
                ...policy,
                maxPlays: next === "unlimited" ? null : Number(next),
              })
            }
          >
            <SelectTrigger id="audio-max-plays" className="w-32">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {PLAY_CHOICES.map((count) => (
                <SelectItem key={count} value={String(count)}>
                  {t("questionEditor.audioPlaysCount", { count })}
                </SelectItem>
              ))}
              <SelectItem value="unlimited">
                {t("questionEditor.audioPlaysUnlimited")}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div className="flex items-center justify-between gap-3">
          <label className="text-sm" htmlFor="audio-allow-seek">
            {t("questionEditor.audioAllowSeek")}
          </label>
          <Switch
            id="audio-allow-seek"
            checked={policy.allowSeek}
            onCheckedChange={(checked) =>
              onPolicyChange({ ...policy, allowSeek: checked })
            }
          />
        </div>

        <div className="flex items-center justify-between gap-3">
          <label className="text-sm leading-snug" htmlFor="audio-show-transcript">
            {t("questionEditor.audioShowTranscript")}
          </label>
          <Switch
            id="audio-show-transcript"
            checked={policy.showTranscriptAfterSubmit}
            onCheckedChange={(checked) =>
              onPolicyChange({ ...policy, showTranscriptAfterSubmit: checked })
            }
          />
        </div>
      </div>

      <div>
        <label
          className="mb-1.5 block text-[0.8125rem] font-medium"
          htmlFor="audio-transcript"
        >
          {t("questionEditor.transcript")}
        </label>
        <Textarea
          id="audio-transcript"
          value={transcript}
          placeholder={t("questionEditor.transcriptPlaceholder")}
          className="min-h-16"
          onChange={(event) => onTranscriptChange(event.target.value)}
        />
      </div>
    </div>
  );
}
