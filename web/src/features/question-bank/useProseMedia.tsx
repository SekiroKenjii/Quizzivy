import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { ReactNodeViewRenderer, type Editor } from "@tiptap/react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import {
  insertProseMedia,
  placeAudioMarker,
  removeAudioMarker,
  type MediaSource,
  type ProseMediaHost,
} from "@/components/shared/content/editor/proseMedia";
import {
  placesAudio,
  withoutAudioMarker,
  type ProseField,
} from "@/components/shared/content/questionContent";
import { ProseMediaMenu } from "@/components/shared/content/editor/ProseMediaMenu";
import {
  ProseAudioView,
  ProseImageView,
} from "@/components/shared/content/editor/ProseMediaViews";
import { AssetLibraryDialog } from "@/features/media/components/AssetLibraryDialog";
import { FileInput } from "@/features/media/components/FileInput";
import { UploadStatus } from "@/features/media/components/UploadStatus";
import { ProseMediaPreview } from "./components/ProseMediaPreview";
import { openFilePicker } from "@/features/media/filePicker";
import { kindOfName, type Rejection } from "@/features/media/limits";
import { useMediaUpload } from "@/features/media/useMediaUpload";
import type { LibraryAsset, MediaAsset, MediaKind } from "@/features/media/api";
import { notify } from "@/lib/toast";
import { startingAudioPolicy } from "./audioPolicy";
import type { MediaPatch } from "./components/QuestionMediaField";
import type { QuestionValues } from "./questionSchema";

const NODE_VIEWS = {
  proseImage: ReactNodeViewRenderer(ProseImageView),
  proseAudio: ReactNodeViewRenderer(ProseAudioView),
};

type Target = Readonly<{ kind: MediaKind; field: ProseField; editor: Editor }>;

/** ProseMedia is what useProseMedia hands a question's page. */
export interface ProseMedia {
  /** For ProseMediaContext, around the question's prompt and explanation. */
  host: ProseMediaHost;
  /** The question's onChange, for every field of the question: it carries an attach that a prompt edit made. */
  change: (next: QuestionValues) => void;
  /** QuestionMediaField's onMedia: an attach or a removal that keeps the prompt's audio marker possible. */
  media: (patch: MediaPatch) => void;
  /** The file picker and "Choose from Media", drawn once anywhere on the page. */
  dialogs: ReactNode;
}

function isImage(file: File): boolean {
  return file.type.startsWith("image/") || kindOfName(file.name) === "image";
}

function refusalTitle(rejection: Rejection, t: TFunction): string {
  if (rejection.reason === "size")
    return t(
      rejection.kind === "image"
        ? "questionEditor.questionMedia.imageTooBig"
        : "questionEditor.questionMedia.audioTooBig",
    );
  if (rejection.reason === "duration")
    return t("questionEditor.questionMedia.audioTooLong");
  if (rejection.reason === "type") return t("contentEditor.media.wrongType");
  return t("questionEditor.questionMedia.unreadable");
}

/**
 * useProseMedia runs the media of a question's prompt and explanation
 * (DG-109) for the page that edits the question. It signs nothing itself:
 * `assets` are the files the server signed for the editor
 * (`AdminQuestion.contentAssets`, or a group's assets), and a file uploaded
 * or picked here joins them and goes to `onAsset`. A prompt's "Audio" is the
 * question's own recording (D-064): attaching one sets `mediaAssetId` and its
 * play limit, in the same change as the marker it places. Removing the
 * question's audio, or replacing it with an image, takes the marker out.
 */
export function useProseMedia({
  value,
  onChange,
  asset,
  onAssetChange,
  assets,
  onAsset,
  onRefresh,
}: Readonly<{
  value: QuestionValues;
  onChange: (next: QuestionValues) => void;
  asset: MediaAsset | null;
  onAssetChange: (asset: MediaAsset | null) => void;
  assets?: ReadonlyMap<string, MediaAsset> | undefined;
  onAsset?: ((asset: MediaAsset) => void) | undefined;
  onRefresh?: (() => void) | undefined;
}>): ProseMedia {
  const { t } = useTranslation();
  const latest = useRef({ value, onChange, onAssetChange, onAsset });
  useEffect(() => {
    latest.current = { value, onChange, onAssetChange, onAsset };
  });
  const pending = useRef<MediaPatch | null>(null);
  const editors = useRef(new Map<ProseField, Editor>());
  const [added, setAdded] = useState<ReadonlyMap<string, MediaAsset>>(() => new Map());
  const target = useRef<Target | null>(null);
  const queue = useRef<File[]>([]);
  const [uploading, setUploading] = useState<Target | null>(null);
  const [library, setLibrary] = useState<Target | null>(null);
  const fileInput = useRef<HTMLInputElement>(null);
  const [pickKind, setPickKind] = useState<MediaKind>("image");

  const change = useCallback((next: QuestionValues) => {
    const patch = pending.current;
    pending.current = null;
    latest.current.onChange(patch ? { ...next, ...patch } : next);
  }, []);

  const settle = useCallback(() => {
    const patch = pending.current;
    pending.current = null;
    if (patch) latest.current.onChange({ ...latest.current.value, ...patch });
  }, []);

  const remember = useCallback((file: MediaAsset) => {
    setAdded((current) => new Map(current).set(file.id.toLowerCase(), file));
    latest.current.onAsset?.(file);
  }, []);

  const place = useCallback(
    (file: MediaAsset | LibraryAsset, to: Target) => {
      if (to.field === "prompt" && file.kind === "audio") {
        const current = latest.current.value;
        latest.current.onAssetChange(file);
        pending.current = {
          mediaAssetId: file.id,
          audio: current.audio ?? startingAudioPolicy(file),
          transcript: current.transcript,
          mediaAlt: null,
        };
        placeAudioMarker(to.editor);
        settle();
        return;
      }
      remember(file);
      const placed = insertProseMedia(to.editor, {
        type: file.kind === "image" ? "proseImage" : "proseAudio",
        attrs:
          file.kind === "image" ? { assetId: file.id, alt: "" } : { assetId: file.id },
      });
      if (!placed) notify.error(t("contentEditor.media.refused"));
    },
    [remember, settle, t],
  );

  const upload = useMediaUpload({
    onUploaded: (file) => {
      notify.success(
        t("questionEditor.questionMedia.saved", { name: file.originalFilename }),
      );
      const to = target.current;
      if (to) place(file, to);
      next();
    },
    onRejected: (rejection, message) => {
      notify.error(refusalTitle(rejection, t), { description: message });
      next();
    },
  });

  function next() {
    const to = target.current;
    const file = queue.current.shift();
    if (!to || !file) return;
    setUploading(to);
    void upload.start(file, { kind: to.kind });
  }

  function send(files: readonly File[], to: Target) {
    target.current = to;
    queue.current = [...files];
    next();
  }

  function pick(to: Target) {
    target.current = to;
    setPickKind(to.kind);
    openFilePicker(fileInput.current, to.kind);
  }

  const choose = (
    request: Readonly<{
      kind: MediaKind;
      source: MediaSource;
      field: ProseField;
      editor: Editor;
    }>,
  ) => {
    const to = { kind: request.kind, field: request.field, editor: request.editor };
    if (request.source === "library") {
      setLibrary(to);
      return;
    }
    pick(to);
  };

  const audio = asset?.kind === "audio" ? asset : null;
  const host: ProseMediaHost = {
    asset: (id) => {
      const key = id.toLowerCase();
      return (
        assets?.get(key) ??
        assets?.get(id) ??
        added.get(key) ??
        (audio?.id === key ? audio : undefined)
      );
    },
    questionAudio: audio,
    audioBlocked:
      asset?.kind === "image" ? t("contentEditor.media.audioBlocked") : null,
    choose,
    files: (files, field, editor) => {
      const images = files.filter(isImage);
      if (images.length === 0) {
        notify.error(t("contentEditor.media.onlyImages"));
        return;
      }
      send(images, { kind: "image", field, editor });
    },
    register: (field, editor) => {
      if (editor) editors.current.set(field, editor);
      else editors.current.delete(field);
    },
    preview: (node) => (
      <ProseMediaPreview
        node={node}
        file={host.asset("assetId" in node ? node.assetId : "")}
        onRefresh={onRefresh}
      />
    ),
    nodeViews: NODE_VIEWS,
    Menu: ProseMediaMenu,
    status: (field) =>
      uploading?.field === field ? (
        <div className="border-t px-3 py-2">
          <UploadStatus
            state={upload.state}
            kind={uploading.kind}
            onCancel={() => {
              queue.current = [];
              upload.cancel();
            }}
            onRetry={() => pick(uploading)}
          />
        </div>
      ) : null,
  };

  const media = useCallback(
    (patch: MediaPatch) => {
      const current = latest.current.value;
      const keepsAudio = patch.audio !== null;
      if (
        keepsAudio ||
        current.promptContent == null ||
        !placesAudio(current.promptContent)
      ) {
        latest.current.onChange({ ...current, ...patch });
        return;
      }
      const editor = editors.current.get("prompt");
      pending.current = patch;
      if (editor !== undefined && removeAudioMarker(editor)) {
        settle();
        return;
      }
      pending.current = null;
      latest.current.onChange({
        ...current,
        ...patch,
        promptContent: withoutAudioMarker(current.promptContent),
      });
    },
    [settle],
  );

  const dialogs = (
    <>
      <FileInput
        inputRef={fileInput}
        kind={pickKind}
        label={t("contentEditor.media.chooseFile")}
        onFile={(file) => {
          const to = target.current;
          if (to) send([file], to);
        }}
      />
      <AssetLibraryDialog
        open={library !== null}
        kind={library?.kind ?? "all"}
        onOpenChange={(open) => {
          if (!open) setLibrary(null);
        }}
        onPick={(file) => {
          const to = library;
          setLibrary(null);
          if (to) place(file, { ...to, kind: file.kind });
        }}
        onUploadNew={(kind) => {
          const to = library;
          if (!to) return;
          pick({ ...to, kind: kind === "all" ? to.kind : kind });
        }}
      />
    </>
  );

  return { host, change, media, dialogs };
}
