import { useId, useRef, useState, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import type { Editor } from "@tiptap/react";
import { AudioLines, CircleSlash, FileText } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Segmented } from "@/components/ui/segmented";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import {
  DialogShell,
  DialogShellBody,
  DialogShellFooter,
  DialogShellHeader,
} from "@/components/shared/form/DialogShell";
import { removeGap } from "@/components/shared/content/editor/gapCommands";
import { ApiError } from "@/lib/api/errors";
import { notify } from "@/lib/toast";
import type { LibraryAsset, MediaAsset, MediaKind } from "@/features/media/api";
import { AssetLibraryDialog } from "@/features/media/components/AssetLibraryDialog";
import { FileInput } from "@/features/media/components/FileInput";
import { UploadStatus } from "@/features/media/components/UploadStatus";
import { openFilePicker } from "@/features/media/filePicker";
import { useMediaUpload, type MediaUpload } from "@/features/media/useMediaUpload";
import { DEFAULT_AUDIO_POLICY } from "@/features/question-bank/audioPolicy";
import { AudioPolicyPanel } from "@/features/question-bank/components/AudioPolicyPanel";
import type { GroupBundle } from "@/features/question-groups/api";
import { GapBindings } from "@/features/question-groups/components/GapBindings";
import { MaterialContent } from "@/features/question-groups/components/MaterialContent";
import {
  emptyMaterial,
  groupIssue,
  type GroupMaterial,
} from "@/features/question-groups/model";
import {
  audioContent,
  withSharedContent,
  type SharedContent,
} from "@/features/question-groups/sharedContent";

type Kind = SharedContent["kind"];
type AudioContent = Extract<SharedContent, { kind: "audio" }>;
type Listening = Pick<AudioContent["recording"], "policy" | "transcript">;

interface Draft {
  kind: Kind;
  passage: GroupMaterial;
  audio: AudioContent | null;
  attached: boolean;
  listening: Listening;
  listeningEdited: boolean;
  edited: boolean;
}

interface Loss {
  kind: "passage" | "audio";
  name: string;
  links: number;
}

const PASSAGE_KINDS: readonly MediaKind[] = ["image"];
const PASSAGE_FONT_SIZE = 13.5;
const PASSAGE_MIN_HEIGHT = 175;

/**
 * SharedContentDialog edits a shared-context group's shared content, which
 * `stored` holds: none, one passage or one recording, picked with a kind
 * switch whose other kind keeps its fields while the dialog is open. It works
 * on a copy taken when it opens and writes nothing itself: Save hands the
 * content to `onSave`, which saves it through the group's editor and rejects
 * with the reason a save failed, and the dialog closes only when that
 * resolves. Saving another kind over `stored` asks first. Save is refused,
 * with one sentence, while `bundle` with the copy would not save or
 * `copyRequired`. Cancel, Escape and the close button discard the copy,
 * asking first when it was edited. `bundle` is the group as the editor holds
 * it now, `assets` the files it knows by id; `onAsset` learns of a file the
 * dialog attaches.
 */
export function SharedContentDialog({
  open,
  title,
  stored,
  bundle,
  assets,
  copyRequired,
  returnFocus,
  onAsset,
  onRefresh,
  onSave,
  onClose,
}: Readonly<{
  open: boolean;
  title: string;
  stored: SharedContent;
  bundle: GroupBundle;
  assets: ReadonlyMap<string, MediaAsset>;
  copyRequired: boolean;
  returnFocus?: RefObject<HTMLElement | null> | undefined;
  onAsset: (asset: MediaAsset) => void;
  onRefresh: () => void;
  onSave: (content: SharedContent) => Promise<void>;
  onClose: () => void;
}>) {
  const { t } = useTranslation();
  const kinds = useRef<HTMLDivElement>(null);
  const reasonId = useId();
  const [session, setSession] = useState(() => ({ open, draft: initialDraft(stored) }));
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [asking, setAsking] = useState<"loss" | "discard" | null>(null);
  if (session.open !== open) {
    setSession({ open, draft: open ? initialDraft(stored) : session.draft });
    if (open) {
      setError(null);
      setAsking(null);
    }
  }
  const draft = session.draft;
  const upload = useMediaUpload({
    kind: "audio",
    onUploaded: (asset) => attach(asset),
  });

  function edit(change: (current: Draft) => Draft) {
    setSession((current) => ({
      ...current,
      draft: { ...change(current.draft), edited: true },
    }));
  }
  function attach(asset: MediaAsset | LibraryAsset) {
    onAsset(asset);
    edit((current) => {
      const audio = audioContent(asset, t("groups.audio"), current.audio ?? undefined);
      const keep = current.audio !== null || current.listeningEdited;
      return {
        ...current,
        audio,
        attached: true,
        listening: keep
          ? current.listening
          : { ...current.listening, policy: audio.recording.policy },
      };
    });
  }
  function close() {
    upload.cancel();
    upload.reset();
    setAsking(null);
    onClose();
  }
  function requestClose() {
    if (pending) return;
    if (draft.edited) setAsking("discard");
    else close();
  }

  const content = contentOf(draft);
  const reason = refusal(draft, content, bundle, copyRequired, t);
  const loss = lossOf(stored, draft.kind, assets, t);

  async function save() {
    if (pending || content === null || reason !== null) return;
    setAsking(null);
    setPending(true);
    setError(null);
    try {
      await onSave(content);
      notify.success(t("builder.sharedDialog.saved"));
      close();
    } catch (cause) {
      setError(
        cause instanceof ApiError && cause.message
          ? cause.message
          : t("groups.saveFailed"),
      );
    } finally {
      setPending(false);
    }
  }

  return (
    <DialogShell
      open={open}
      onOpenChange={(next) => {
        if (!next) requestClose();
      }}
      width={560}
      returnFocus={returnFocus}
      onOpenAutoFocus={(event) => {
        const pressed = kinds.current?.querySelector<HTMLElement>(
          '[aria-pressed="true"]',
        );
        if (!pressed) return;
        event.preventDefault();
        pressed.focus();
      }}
    >
      <DialogShellHeader
        title={t("builder.sharedDialog.title", { group: title })}
        description={t("builder.sharedDialog.description")}
      />
      <DialogShellBody inert={pending} className="max-h-[56vh] gap-3">
        <div ref={kinds}>
          <Segmented
            fill
            label={t("builder.sharedDialog.kind")}
            value={draft.kind}
            className="[&_svg]:size-3.5 [&>button]:text-[13px]"
            options={[
              {
                value: "none",
                label: t("builder.sharedDialog.none"),
                icon: CircleSlash,
              },
              { value: "passage", label: t("builder.sharedPassage"), icon: FileText },
              { value: "audio", label: t("builder.sharedAudio"), icon: AudioLines },
            ]}
            onChange={(kind) => edit((current) => ({ ...current, kind: kind as Kind }))}
          />
        </div>
        {draft.kind === "passage" ? (
          <PassageFields
            material={draft.passage}
            bundle={bundle}
            assets={assets}
            onAsset={onAsset}
            onRefresh={onRefresh}
            onChange={(passage) => edit((current) => ({ ...current, passage }))}
          />
        ) : null}
        {draft.kind === "audio" ? (
          <AudioFields
            audio={draft.audio}
            attached={draft.attached}
            listening={draft.listening}
            assets={assets}
            upload={upload}
            onPick={attach}
            onReplace={() => edit((current) => ({ ...current, attached: false }))}
            onChange={(listening) =>
              edit((current) => ({ ...current, listening, listeningEdited: true }))
            }
          />
        ) : null}
        {error ? (
          <p role="alert" className="text-danger-ink text-xs">
            {error}
          </p>
        ) : null}
      </DialogShellBody>
      <DialogShellFooter className="items-center">
        {reason ? (
          <p
            id={reasonId}
            className="text-muted-fg mr-auto min-w-0 flex-[1_1_12rem] text-xs"
          >
            {reason}
          </p>
        ) : null}
        <Button
          type="button"
          variant="outline"
          className="bg-card text-ui h-9 px-3.5 leading-normal shadow-none"
          onClick={requestClose}
        >
          {t("common.cancel")}
        </Button>
        <Button
          type="button"
          disabled={reason !== null}
          aria-disabled={pending || undefined}
          aria-describedby={reason ? reasonId : undefined}
          className="text-ui h-9 px-4 leading-normal shadow-none"
          onClick={() => {
            if (pending) return;
            if (loss) setAsking("loss");
            else void save();
          }}
        >
          {pending ? t("common.saving") : t("common.save")}
        </Button>
      </DialogShellFooter>
      <ConfirmDialog
        open={asking === "loss"}
        onOpenChange={(next) => !next && setAsking(null)}
        title={t(
          draft.kind === "none"
            ? "builder.sharedDialog.removeTitle"
            : "builder.sharedDialog.replaceTitle",
        )}
        description={loss ? lossSentence(loss, t) : undefined}
        confirmLabel={t(
          draft.kind === "none"
            ? "builder.sharedDialog.remove"
            : "builder.sharedDialog.replace",
        )}
        destructive
        onConfirm={() => void save()}
      />
      <ConfirmDialog
        open={asking === "discard"}
        onOpenChange={(next) => !next && setAsking(null)}
        title={t("builder.sharedDialog.discardTitle")}
        description={t("builder.sharedDialog.discardBody")}
        confirmLabel={t("builder.sharedDialog.discard")}
        cancelLabel={t("builder.sharedDialog.keepEditing")}
        destructive
        onConfirm={close}
      />
    </DialogShell>
  );
}

function PassageFields({
  material,
  bundle,
  assets,
  onAsset,
  onRefresh,
  onChange,
}: Readonly<{
  material: GroupMaterial;
  bundle: GroupBundle;
  assets: ReadonlyMap<string, MediaAsset>;
  onAsset: (asset: MediaAsset) => void;
  onRefresh: () => void;
  onChange: (material: GroupMaterial) => void;
}>) {
  const { t } = useTranslation();
  const titleId = useId();
  const [editor, setEditor] = useState<Editor | null>(null);
  return (
    <>
      <div className="flex flex-col gap-1.5">
        <label htmlFor={titleId} className="text-meta font-medium">
          {t("builder.sharedDialog.titleLabel")}
        </label>
        <Input
          id={titleId}
          value={material.title}
          maxLength={200}
          placeholder={t("builder.sharedDialog.titlePlaceholder")}
          className="h-9 rounded-[8px]"
          onChange={(event) => onChange({ ...material, title: event.target.value })}
        />
      </div>
      <div className="flex flex-col gap-1.5">
        <p aria-hidden="true" className="text-meta font-medium">
          {t("builder.sharedPassage")}
        </p>
        <MaterialContent
          material={material}
          preview={false}
          assets={assets}
          label={t("builder.sharedPassage")}
          placeholder={t("builder.sharedDialog.passagePlaceholder")}
          minHeight={PASSAGE_MIN_HEIGHT}
          fontSize={PASSAGE_FONT_SIZE}
          assetKinds={PASSAGE_KINDS}
          onChange={onChange}
          onAsset={onAsset}
          onRefresh={onRefresh}
          onEditor={setEditor}
        />
      </div>
      <GapBindings
        material={material}
        bundle={withSharedContent(bundle, { kind: "passage", material })}
        onChange={onChange}
        onRemove={editor ? (gapId) => removeGap(editor, gapId) : undefined}
      />
    </>
  );
}

function AudioFields({
  audio,
  attached,
  listening,
  assets,
  upload,
  onPick,
  onReplace,
  onChange,
}: Readonly<{
  audio: AudioContent | null;
  attached: boolean;
  listening: Listening;
  assets: ReadonlyMap<string, MediaAsset>;
  upload: MediaUpload<MediaAsset>;
  onPick: (asset: LibraryAsset) => void;
  onReplace: () => void;
  onChange: (listening: Listening) => void;
}>) {
  const { t } = useTranslation();
  const fileInput = useRef<HTMLInputElement>(null);
  const library = useRef<HTMLButtonElement>(null);
  const choose = useRef<HTMLButtonElement>(null);
  const [picking, setPicking] = useState(false);
  const file =
    audio && attached
      ? (assets.get(audio.recording.assetId)?.originalFilename ?? audio.material.title)
      : null;
  return (
    <>
      {file === null ? (
        <div className="flex flex-wrap items-center gap-2.5 rounded-[10px] border-[1.5px] border-dashed p-3">
          <AudioLines aria-hidden="true" className="text-muted-fg size-4.25 shrink-0" />
          <span className="min-w-0 flex-[1_1_10rem] text-[13px]">
            {t("builder.sharedDialog.audioLimits")}
          </span>
          <Button
            ref={choose}
            type="button"
            variant="outline"
            disabled={upload.busy}
            className={ZONE_BUTTON}
            onClick={() => openFilePicker(fileInput.current, "audio")}
          >
            {t("questionEditor.questionMedia.choose")}
          </Button>
          <Button
            ref={library}
            type="button"
            variant="outline"
            disabled={upload.busy}
            className={ZONE_BUTTON}
            onClick={() => setPicking(true)}
          >
            {t("questionEditor.questionMedia.fromMedia")}
          </Button>
        </div>
      ) : (
        <div className="flex items-center gap-2.5 rounded-[10px] border px-3 py-2.5">
          <AudioLines aria-hidden="true" className="size-4.25 shrink-0" />
          <span
            title={file}
            className="min-w-0 flex-1 truncate text-[13px] font-medium"
          >
            {file}
          </span>
          <Button
            type="button"
            variant="outline"
            aria-label={t("builder.sharedDialog.replaceFile", { name: file })}
            className="bg-card h-7 rounded-[7px] px-2.5 text-xs font-medium shadow-none"
            onClick={() => {
              onReplace();
              requestAnimationFrame(() => choose.current?.focus());
            }}
          >
            {t("questionEditor.questionMedia.replace")}
          </Button>
        </div>
      )}
      <UploadStatus
        state={upload.state}
        kind="audio"
        onCancel={upload.cancel}
        onRetry={() => openFilePicker(fileInput.current, "audio")}
      />
      <FileInput
        inputRef={fileInput}
        kind="audio"
        onFile={(next) => void upload.start(next)}
      />
      <div className="overflow-hidden rounded-[10px] border [&>:first-child]:border-t-0">
        <AudioPolicyPanel
          policy={listening.policy}
          transcript={listening.transcript ?? ""}
          transcriptHint={
            listening.policy.showTranscriptAfterSubmit
              ? t("builder.sharedDialog.transcriptHint")
              : undefined
          }
          onPolicyChange={(policy) => onChange({ ...listening, policy })}
          onTranscriptChange={(transcript) => onChange({ ...listening, transcript })}
        />
      </div>
      <AssetLibraryDialog
        open={picking}
        onOpenChange={setPicking}
        kind="audio"
        returnFocus={library}
        onPick={onPick}
        onUploadNew={() => openFilePicker(fileInput.current, "audio")}
      />
    </>
  );
}

const ZONE_BUTTON =
  "bg-card font-medium shadow-none h-7.5 rounded-[7px] px-2.5 text-meta";

function initialDraft(stored: SharedContent): Draft {
  return {
    kind: stored.kind,
    passage: stored.kind === "passage" ? stored.material : emptyMaterial(""),
    audio: stored.kind === "audio" ? stored : null,
    attached: stored.kind === "audio",
    listening:
      stored.kind === "audio"
        ? {
            policy: stored.recording.policy,
            transcript: stored.recording.transcript ?? null,
          }
        : { policy: { ...DEFAULT_AUDIO_POLICY }, transcript: null },
    listeningEdited: false,
    edited: false,
  };
}

function contentOf(draft: Draft): SharedContent | null {
  if (draft.kind === "none") return { kind: "none" };
  if (draft.kind === "passage") return { kind: "passage", material: draft.passage };
  if (!draft.audio || !draft.attached) return null;
  return {
    ...draft.audio,
    recording: { ...draft.audio.recording, ...draft.listening },
  };
}

function refusal(
  draft: Draft,
  content: SharedContent | null,
  bundle: GroupBundle,
  copyRequired: boolean,
  t: TFunction,
): string | null {
  if (copyRequired) return t("builder.sharedDialog.conflict");
  if (content === null) return t("builder.sharedDialog.noFile");
  const issue = groupIssue(withSharedContent(bundle, content));
  if (issue === null) return null;
  if (draft.kind === "passage" && issue === "groups.materialTitleRequired")
    return t("builder.sharedDialog.titleRequired");
  return t(issue);
}

function lossOf(
  stored: SharedContent,
  kind: Kind,
  assets: ReadonlyMap<string, MediaAsset>,
  t: TFunction,
): Loss | null {
  if (stored.kind === "passage" && kind !== "passage")
    return {
      kind: "passage",
      name: stored.material.title.trim() || t("builder.sharedUntitled"),
      links: stored.material.gaps.length,
    };
  if (stored.kind === "audio" && kind !== "audio")
    return {
      kind: "audio",
      name:
        assets.get(stored.recording.assetId)?.originalFilename ?? stored.material.title,
      links: 0,
    };
  return null;
}

function lossSentence(loss: Loss, t: TFunction): string {
  if (loss.kind === "audio")
    return t("builder.sharedDialog.loseAudio", { name: loss.name });
  if (loss.links === 0)
    return t("builder.sharedDialog.losePassage", { name: loss.name });
  return t("builder.sharedDialog.losePassageGaps", {
    name: loss.name,
    count: loss.links,
  });
}
