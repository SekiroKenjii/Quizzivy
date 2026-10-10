import { useId, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import {
  AudioLines,
  FileAudio,
  Image as ImageIcon,
  Library,
  Trash2,
  Upload,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { AssetLibraryDialog } from "@/features/media/components/AssetLibraryDialog";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import { FileInput } from "@/features/media/components/FileInput";
import { openFilePicker } from "@/features/media/filePicker";
import { UploadStatus } from "@/features/media/components/UploadStatus";
import { useMediaUpload } from "@/features/media/useMediaUpload";
import { assetMeta, formatBytes } from "@/features/media/format";
import { kindOfName, type Rejection } from "@/features/media/limits";
import type { LibraryAsset, MediaAsset, MediaKind } from "@/features/media/api";
import {
  DEFAULT_AUDIO_POLICY,
  type AudioPolicy,
} from "@/features/question-bank/audioPolicy";
import type { QuestionValues } from "@/features/question-bank/questionSchema";
import { useFileDrop } from "@/hooks/useFileDrop";
import { notify } from "@/lib/toast";
import { cn } from "@/lib/utils";
import { AudioPolicyPanel } from "./AudioPolicyPanel";

type Attachable = MediaAsset | LibraryAsset;

interface QuestionMediaFieldProps {
  value: QuestionValues;
  asset: MediaAsset | null;
  onChange: (value: QuestionValues) => void;
  onAssetChange: (asset: MediaAsset | null) => void;
  /** Refetches the question, minting a fresh signed URL when one expires. */
  onRefresh?: (() => void) | undefined;
}

/**
 * QuestionMediaField is the question's media block: "Question media", then a
 * drop zone with "Choose file" and "From Media" while nothing is attached,
 * or the attached file's card with "Replace" (a file of the same kind for
 * this question) and "Remove media". Audio adds the player and the listening
 * settings, an image its preview and alt text. A file is pre-checked against
 * `limits.ts` before it is sent, and a refusal is an error toast naming the
 * file; an upload shows its progress and its failure, and a success says it
 * was saved to Media. Attaching audio starts its play limit from the file's
 * default, else Twice, unless the question already has one (DG-69).
 */
export function QuestionMediaField({
  value,
  asset,
  onChange,
  onAssetChange,
  onRefresh,
}: Readonly<QuestionMediaFieldProps>) {
  const { t } = useTranslation();
  const fileInput = useRef<HTMLInputElement>(null);
  const requested = useRef<MediaKind | null>(null);
  const [picking, setPicking] = useState(false);
  const [sent, setSent] = useState<{ kind: MediaKind; asked: MediaKind | null }>({
    kind: "audio",
    asked: null,
  });

  function attach(next: Attachable) {
    onAssetChange(next);
    onChange({
      ...value,
      mediaAssetId: next.id,
      audio: next.kind === "audio" ? (value.audio ?? startingPolicy(next)) : null,
      transcript: next.kind === "audio" ? value.transcript : null,
      mediaAlt: null,
    });
  }

  function remove() {
    onAssetChange(null);
    onChange({
      ...value,
      mediaAssetId: null,
      audio: null,
      transcript: null,
      mediaAlt: null,
    });
  }

  const upload = useMediaUpload({
    onUploaded: (uploaded) => {
      attach(uploaded);
      notify.success(
        t("questionEditor.questionMedia.saved", { name: uploaded.originalFilename }),
      );
    },
    onRejected: (rejection, message) =>
      notify.error(refusalTitle(rejection, t), { description: message }),
  });

  function send(files: readonly File[], kind: MediaKind | null) {
    const first = files[0];
    const checkedAs = kind ?? (first ? kindOfName(first.name) : null);
    if (first && files.length === 1 && checkedAs === null) {
      notify.error(t("questionEditor.questionMedia.chooseKind"), {
        description: t("questionEditor.questionMedia.notMedia", { name: first.name }),
      });
      return;
    }
    if (checkedAs !== null) setSent({ kind: checkedAs, asked: kind });
    upload.dropped(files, checkedAs === null ? undefined : { kind: checkedAs });
  }

  const dragging = useFileDrop((files) => send(files, null), !upload.busy);

  function pick(kind: MediaKind | null) {
    requested.current = kind;
    openFilePicker(fileInput.current, kind ?? "any");
  }

  return (
    <div className="flex flex-col gap-2">
      <p className="text-[12.5px] font-medium">
        {t("questionEditor.questionMedia.label")}{" "}
        <span className="text-muted-fg font-normal">
          · {t("questionEditor.questionMedia.hint")}
        </span>
      </p>

      {asset === null ? (
        <DropZone
          dragging={dragging}
          onChoose={() => pick(null)}
          onLibrary={() => setPicking(true)}
        />
      ) : (
        <AttachedCard
          value={value}
          asset={asset}
          onChange={onChange}
          onReplace={() => pick(asset.kind)}
          onRemove={remove}
          onRefresh={onRefresh}
        />
      )}

      {upload.state.status === "checking" ? (
        <CheckingCard
          name={upload.state.name}
          bytes={upload.state.bytes}
          kind={upload.state.kind}
          onCancel={upload.cancel}
        />
      ) : (
        <UploadStatus
          state={upload.state}
          kind={sent.kind}
          onCancel={upload.cancel}
          onRetry={() => pick(sent.asked)}
        />
      )}

      <FileInput
        inputRef={fileInput}
        onFile={(file) => send([file], requested.current)}
      />
      <AssetLibraryDialog
        open={picking}
        onOpenChange={setPicking}
        onPick={attach}
        onUploadNew={(kind) => pick(kind === "all" ? null : kind)}
      />
    </div>
  );
}

function startingPolicy(asset: Attachable): AudioPolicy {
  const stored = "defaultMaxPlays" in asset ? asset.defaultMaxPlays : null;
  if (stored === null) return DEFAULT_AUDIO_POLICY;
  return { ...DEFAULT_AUDIO_POLICY, maxPlays: stored === 0 ? null : stored };
}

function refusalTitle(rejection: Rejection, t: TFunction): string {
  const image = rejection.kind === "image";
  switch (rejection.reason) {
    case "type":
      return t("questionEditor.questionMedia.chooseKind");
    case "size":
      return t(
        image
          ? "questionEditor.questionMedia.imageTooBig"
          : "questionEditor.questionMedia.audioTooBig",
      );
    case "duration":
      return t("questionEditor.questionMedia.audioTooLong");
    default:
      return t("questionEditor.questionMedia.unreadable");
  }
}

function DropZone({
  dragging,
  onChoose,
  onLibrary,
}: Readonly<{ dragging: boolean; onChoose: () => void; onLibrary: () => void }>) {
  const { t } = useTranslation();
  return (
    <div
      className={cn(
        "flex flex-wrap items-center gap-x-3.5 gap-y-3 rounded-[10px] border-[1.5px] border-dashed px-4 py-3.5",
        dragging ? "border-primary bg-muted" : "border-border",
      )}
    >
      <span
        aria-hidden="true"
        className="bg-muted grid size-9 flex-none place-items-center rounded-[9px]"
      >
        <AudioLines className="size-4.25" />
      </span>
      <span className="min-w-0 flex-[1_1_220px]">
        <span className="block text-[13.5px] font-medium">
          {t("questionEditor.questionMedia.drop")}
        </span>
        <span className="text-muted-fg block text-xs leading-normal">
          {t("questionEditor.questionMedia.limits")}
        </span>
      </span>
      <span className="flex flex-wrap gap-1.5">
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="h-8 rounded-[8px] px-3 text-[13px] in-data-[scale=deck]:h-8 in-data-[scale=deck]:rounded-[8px] in-data-[scale=deck]:text-[13px]"
          onClick={onChoose}
        >
          <Upload aria-hidden="true" className="size-3.5" />
          {t("questionEditor.questionMedia.choose")}
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="h-8 rounded-[8px] px-3 text-[13px] in-data-[scale=deck]:h-8 in-data-[scale=deck]:rounded-[8px] in-data-[scale=deck]:text-[13px]"
          onClick={onLibrary}
        >
          <Library aria-hidden="true" className="size-3.5" />
          {t("questionEditor.questionMedia.fromMedia")}
        </Button>
      </span>
    </div>
  );
}

function CheckingCard({
  name,
  bytes,
  kind,
  onCancel,
}: Readonly<{ name: string; bytes: number; kind: MediaKind; onCancel: () => void }>) {
  const { t } = useTranslation();
  const size = formatBytes(bytes);
  return (
    <div
      role="status"
      className="flex items-center gap-3 rounded-[10px] border px-3 py-2.5"
    >
      <FileTile kind={kind} />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-[13.5px] font-medium">{name}</span>
        <span className="text-muted-fg block text-xs">
          {kind === "audio"
            ? t("questionEditor.questionMedia.checkingLength", { size })
            : size}
        </span>
      </span>
      <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
        {t("common.cancel")}
      </Button>
    </div>
  );
}

function FileTile({ kind }: Readonly<{ kind: MediaKind }>) {
  const Icon = kind === "image" ? ImageIcon : FileAudio;
  return (
    <span
      aria-hidden="true"
      className="bg-info-soft text-info-ink grid size-9 flex-none place-items-center rounded-[9px]"
    >
      <Icon className="size-4.25" />
    </span>
  );
}

function AttachedCard({
  value,
  asset,
  onChange,
  onReplace,
  onRemove,
  onRefresh,
}: Readonly<{
  value: QuestionValues;
  asset: MediaAsset;
  onChange: (value: QuestionValues) => void;
  onReplace: () => void;
  onRemove: () => void;
  onRefresh: (() => void) | undefined;
}>) {
  const { t } = useTranslation();
  const [natural, setNatural] = useState<{
    url: string;
    width: number;
    height: number;
  }>();
  const known = "width" in asset ? (asset as LibraryAsset) : null;
  const dimensions =
    natural?.url === asset.url ? natural : (known ?? { width: null, height: null });
  const retry = onRefresh ? { onRetry: onRefresh } : {};
  return (
    <div className="overflow-hidden rounded-[10px] border">
      <div className="flex items-center gap-3 px-3 py-2.5">
        <FileTile kind={asset.kind} />
        <span className="min-w-0 flex-1">
          <span className="block truncate text-[13.5px] font-medium">
            {asset.originalFilename}
          </span>
          <span className="text-muted-fg block text-xs">
            {assetMeta({ ...asset, ...dimensions }, t)}
          </span>
        </span>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          aria-label={t("questionEditor.questionMedia.replaceLabel")}
          className="h-7.5 rounded-[7px] px-2.5 text-[12.5px] in-data-[scale=deck]:h-7.5 in-data-[scale=deck]:rounded-[7px] in-data-[scale=deck]:text-[12.5px]"
          onClick={onReplace}
        >
          {t("questionEditor.questionMedia.replace")}
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          aria-label={t("questionEditor.questionMedia.remove")}
          title={t("questionEditor.questionMedia.remove")}
          className="text-muted-fg hover:text-danger-ink size-7.5 rounded-[7px] in-data-[scale=deck]:size-7.5 in-data-[scale=deck]:rounded-[7px]"
          onClick={onRemove}
        >
          <Trash2 aria-hidden="true" className="size-3.75" />
        </Button>
      </div>

      {asset.kind === "audio" ? (
        <>
          <div className="bg-sidebar border-t px-3 py-2.5">
            <AudioPlayer
              src={asset.url}
              label={asset.originalFilename}
              durationMs={asset.durationMs}
              size="sm"
              {...retry}
            />
          </div>
          <AudioPolicyPanel
            policy={value.audio ?? DEFAULT_AUDIO_POLICY}
            transcript={value.transcript ?? ""}
            onPolicyChange={(audio) => onChange({ ...value, audio })}
            onTranscriptChange={(transcript) => onChange({ ...value, transcript })}
          />
        </>
      ) : (
        <ImageDetails
          value={value}
          asset={asset}
          onChange={onChange}
          onLoaded={(width, height) => setNatural({ url: asset.url, width, height })}
          onRefresh={onRefresh}
        />
      )}
    </div>
  );
}

function ImageDetails({
  value,
  asset,
  onChange,
  onLoaded,
  onRefresh,
}: Readonly<{
  value: QuestionValues;
  asset: MediaAsset;
  onChange: (value: QuestionValues) => void;
  onLoaded: (width: number, height: number) => void;
  onRefresh: (() => void) | undefined;
}>) {
  const { t } = useTranslation();
  const help = useId();
  const [brokenFor, setBrokenFor] = useState<string | null>(null);
  return (
    <div className="flex flex-col gap-2 border-t p-3">
      {brokenFor === asset.url ? (
        <div className="bg-muted text-muted-fg flex h-37.5 flex-col items-center justify-center gap-1.5 rounded-lg">
          <ImageIcon aria-hidden="true" className="size-6" />
          <span className="text-xs">{asset.originalFilename}</span>
        </div>
      ) : (
        <img
          src={asset.url}
          alt={value.mediaAlt ?? asset.originalFilename}
          className="bg-muted h-55 w-full rounded-lg object-contain"
          onLoad={(event) =>
            onLoaded(
              event.currentTarget.naturalWidth,
              event.currentTarget.naturalHeight,
            )
          }
          onError={() => {
            setBrokenFor(asset.url);
            onRefresh?.();
          }}
        />
      )}
      <Input
        value={value.mediaAlt ?? ""}
        maxLength={1000}
        aria-label={t("questionEditor.questionMedia.altLabel")}
        aria-describedby={help}
        placeholder={t("questionEditor.questionMedia.altPlaceholder")}
        className="bg-background h-8.5 rounded-lg in-data-[scale=deck]:h-8.5"
        onChange={(event) =>
          onChange({
            ...value,
            mediaAlt: event.target.value.trim() === "" ? null : event.target.value,
          })
        }
      />
      <p id={help} className="text-muted-fg text-xs leading-normal">
        {t("questionEditor.questionMedia.altHelp")}
      </p>
    </div>
  );
}
