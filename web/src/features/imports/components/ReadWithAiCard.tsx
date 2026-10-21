import { useId } from "react";
import { useTranslation } from "react-i18next";
import { Sparkles } from "lucide-react";
import { IconTile } from "@/components/shared/IconTile";
import { Switch } from "@/components/ui/switch";
import { AI_BLOCK_KEYS, type AiBlock } from "@/features/ai/useAiAvailable";

/**
 * ReadWithAiCard is the "Read with AI" toggle card of a new import's Sources
 * pane (V-11). While `blocked` names a reason, or `locked` says the import
 * was already created with the choice, or `busy`, the switch stays focusable,
 * answers `aria-disabled` and ignores a press; a reason or the lock is said
 * under the hint.
 */
export function ReadWithAiCard({
  on,
  blocked,
  locked,
  busy,
  onChange,
}: Readonly<{
  on: boolean;
  blocked: AiBlock | null;
  locked: boolean;
  busy: boolean;
  onChange: (on: boolean) => void;
}>) {
  const { t } = useTranslation();
  const titleId = useId();
  const hintId = useId();
  const noteId = useId();
  const inert = blocked !== null || locked || busy;
  let note: string | null = null;
  if (blocked !== null) note = t(AI_BLOCK_KEYS[blocked]);
  else if (locked) note = t("imports.upload.aiLocked");
  return (
    <div className="flex items-center gap-3 rounded-[10px] border px-3.5 py-3">
      <IconTile icon={Sparkles} size={32} tone="accent" />
      <span className="min-w-0 flex-1">
        <span id={titleId} className="text-ui block leading-normal font-medium">
          {t("imports.upload.aiTitle")}
        </span>
        <span
          id={hintId}
          className="text-muted-fg text-meta block leading-normal text-pretty"
        >
          {t("imports.upload.aiHint")}
        </span>
        {note === null ? null : (
          <span
            id={noteId}
            className="text-muted-fg text-meta mt-1 block leading-normal text-pretty"
          >
            {note}
          </span>
        )}
      </span>
      <Switch
        checked={on}
        aria-labelledby={titleId}
        aria-describedby={note === null ? hintId : `${hintId} ${noteId}`}
        aria-disabled={inert || undefined}
        className="aria-disabled:cursor-default"
        onCheckedChange={(next) => {
          if (!inert) onChange(next);
        }}
      />
    </div>
  );
}
