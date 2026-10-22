import { useTranslation } from "react-i18next";
import { NodeViewWrapper, type ReactNodeViewProps } from "@tiptap/react";
import { AudioLines, Image as ImageIcon, Trash2 } from "lucide-react";
import { cn } from "@/lib/utils";
import { altDraft } from "../questionContent";
import { QUESTION_MEDIA_LIMITS } from "../model";
import { useProseMediaHost } from "./proseMedia";

const FIGURE = "my-2.5 flex max-w-[560px] flex-col gap-1.5";
const REMOVE =
  "bg-card text-fg hover:bg-hover hover:text-danger-ink grid size-7 flex-none place-items-center rounded-[7px] border";

function RemoveButton({
  label,
  onRemove,
}: Readonly<{ label: string; onRemove: () => void }>) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      className={REMOVE}
      onClick={onRemove}
    >
      <Trash2 aria-hidden="true" className="size-3.5" />
    </button>
  );
}

/**
 * ProseImageView is an image in a prompt or an explanation as the editor
 * draws it (2026-10-04 deck): the picture in its frame with "Remove image",
 * and under it the alt text field. An empty alt marks the image decorative,
 * so the field asks for a description and says what leaving it empty means;
 * white space alone is kept as empty, never sent.
 */
export function ProseImageView({
  node,
  updateAttributes,
  deleteNode,
  selected,
}: ReactNodeViewProps) {
  const { t } = useTranslation();
  const host = useProseMediaHost();
  const assetId = String(node.attrs["assetId"] ?? "");
  const alt = String(node.attrs["alt"] ?? "");
  const hint = `prose-image-${assetId}-hint`;
  return (
    <NodeViewWrapper
      as="figure"
      className={FIGURE}
      data-media="image"
      contentEditable={false}
    >
      <div
        className={cn(
          "bg-muted relative overflow-hidden rounded-[10px] border",
          selected && "outline-focus outline-2 outline-offset-2",
        )}
      >
        {host?.preview({ type: "image", assetId, alt }) ?? (
          <div className="text-muted-fg flex h-42.5 flex-col items-center justify-center gap-1.5">
            <ImageIcon aria-hidden="true" className="size-6.5" />
          </div>
        )}
        <span className="absolute top-2 right-2">
          <RemoveButton
            label={t("contentEditor.media.removeImage")}
            onRemove={deleteNode}
          />
        </span>
      </div>
      <input
        value={alt}
        maxLength={QUESTION_MEDIA_LIMITS.alt}
        aria-label={t("contentEditor.media.alt")}
        aria-describedby={alt === "" ? hint : undefined}
        placeholder={t("contentEditor.media.altPlaceholder")}
        className="bg-bg text-fg focus:border-ring h-7.5 rounded-[7px] border px-2.5 text-[12.5px] outline-0"
        onChange={(event) => updateAttributes({ alt: altDraft(event.target.value) })}
      />
      {alt === "" ? (
        <figcaption id={hint} className="text-muted-fg text-xs leading-normal">
          {t("contentEditor.media.altNudge")}
        </figcaption>
      ) : null}
    </NodeViewWrapper>
  );
}

/**
 * ProseAudioView is audio in a prompt or an explanation as the editor draws
 * it. A prompt's marker is the question's own recording, named with what the
 * question's play limit means for it, or a warning while the question has no
 * audio; an explanation's recording shows its player. Both can be removed.
 */
export function ProseAudioView({ node, deleteNode, selected }: ReactNodeViewProps) {
  const { t } = useTranslation();
  const host = useProseMediaHost();
  const assetId = node.attrs["assetId"] as string | null;
  const audio = host?.questionAudio ?? null;
  let body;
  if (assetId !== null) body = host?.preview({ type: "audio", assetId }) ?? null;
  else
    body = (
      <span className="flex min-w-0 flex-1 items-center gap-3">
        <span
          aria-hidden="true"
          className="bg-primary text-primary-fg grid size-8.5 flex-none place-items-center rounded-full"
        >
          <AudioLines className="size-4" />
        </span>
        <span className="min-w-0 flex-1">
          <span className="block truncate text-[13px] font-medium">
            {t("contentEditor.media.questionAudio")}
          </span>
          <span
            className={cn("block text-xs", audio ? "text-muted-fg" : "text-danger-ink")}
          >
            {audio
              ? t("contentEditor.media.questionAudioHint", {
                  name: audio.originalFilename,
                })
              : t("contentEditor.media.questionAudioMissing")}
          </span>
        </span>
      </span>
    );
  return (
    <NodeViewWrapper
      className={cn(
        "bg-sidebar my-2.5 flex max-w-[560px] items-center gap-3 rounded-[10px] border px-3 py-2.5",
        selected && "outline-focus outline-2 outline-offset-2",
      )}
      data-media="audio"
      contentEditable={false}
    >
      <span className="min-w-0 flex-1">{body}</span>
      <RemoveButton
        label={t("contentEditor.media.removeAudio")}
        onRemove={deleteNode}
      />
    </NodeViewWrapper>
  );
}
