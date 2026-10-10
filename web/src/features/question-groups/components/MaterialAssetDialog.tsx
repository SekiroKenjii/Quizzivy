import { useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { FileAudio, FileImage } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Alert, AlertDescription } from "@/components/ui/alert";
import {
  DialogShell,
  DialogShellBody,
  DialogShellFooter,
  DialogShellHeader,
} from "@/components/shared/form/DialogShell";
import { LoadMoreSentinel } from "@/components/shared/LoadMoreSentinel";
import { ASSET_TEXT_LIMITS } from "@/components/shared/content/model";
import { FileInput } from "@/features/media/components/FileInput";
import { UploadStatus } from "@/features/media/components/UploadStatus";
import { useMediaUpload } from "@/features/media/useMediaUpload";
import { listMedia, type MediaAsset, type MediaKind } from "@/features/media/api";
import { formatBytes } from "@/features/media/format";
import { useLazyList } from "@/hooks/useLazyList";

/**
 * MaterialAssetDialog inserts an image or a recording into a group's material:
 * the label the student's reader announces, a file from the library, or a new
 * upload through the shared `FileInput` and `useMediaUpload`, so both kinds get
 * the same pre-check, progress, cancel and failure as the rest of the app.
 */
export function MaterialAssetDialog({
  kind,
  onClose,
  onInsert,
  insertError,
}: Readonly<{
  kind: MediaKind;
  insertError: string | null;
  onClose: () => void;
  onInsert: (asset: MediaAsset, label: string) => void;
}>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [chosen, setChosen] = useState<MediaAsset | null>(null);
  const [label, setLabel] = useState(
    t(kind === "image" ? "groups.image" : "groups.audio"),
  );
  const fileInput = useRef<HTMLInputElement>(null);
  const upload = useMediaUpload({ kind, onUploaded: setChosen });
  const library = useLazyList({
    queryKey: ["admin-media", "group-picker", kind],
    fetchPage: (page, signal) => listMedia({ kind, page, limit: 20 }, signal),
  });

  return (
    <DialogShell open onOpenChange={(open) => !open && onClose()} width={560}>
      <DialogShellHeader
        icon={kind === "image" ? FileImage : FileAudio}
        title={t(kind === "image" ? "groups.insertImage" : "groups.insertAudio")}
        description={t("groups.assetHint")}
      />
      <DialogShellBody>
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor="group-asset-label">
              {t(kind === "image" ? "groups.imageLabel" : "groups.audioLabel")}
            </FieldLabel>
            <Input
              id="group-asset-label"
              maxLength={ASSET_TEXT_LIMITS[kind]}
              value={label}
              onChange={(event) => setLabel(event.target.value)}
            />
            <FieldDescription>{t("groups.assetLabelHint")}</FieldDescription>
          </Field>
          <Field>
            <FieldLabel>{t("groups.chooseAsset")}</FieldLabel>
            <div className="flex max-h-56 flex-col gap-1 overflow-y-auto">
              {library.isPending ? <p role="status">{t("common.loading")}</p> : null}
              {library.isError ? (
                <Alert>
                  <AlertDescription>
                    {t("media.loadFailed")}
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() =>
                        void queryClient.invalidateQueries({
                          queryKey: ["admin-media", "group-picker", kind],
                        })
                      }
                    >
                      {t("common.retry")}
                    </Button>
                  </AlertDescription>
                </Alert>
              ) : null}
              {!library.isPending && !library.isError && library.items.length === 0 ? (
                <p className="text-muted-fg text-sm">{t("media.empty")}</p>
              ) : null}
              {library.items.map((asset) => (
                <Button
                  key={asset.id}
                  variant={chosen?.id === asset.id ? "secondary" : "ghost"}
                  className="h-auto justify-between gap-3 py-2 text-left whitespace-normal"
                  aria-pressed={chosen?.id === asset.id}
                  onClick={() => setChosen(asset)}
                >
                  <span className="min-w-0 break-words">{asset.originalFilename}</span>
                  <span className="shrink-0 text-xs">{formatBytes(asset.bytes)}</span>
                </Button>
              ))}
              <LoadMoreSentinel
                active={library.hasMore}
                loading={library.loadingMore}
                onVisible={library.loadMore}
              />
            </div>
          </Field>
          <Field>
            <Button
              variant="outline"
              disabled={upload.busy}
              onClick={() => fileInput.current?.click()}
            >
              {upload.busy ? t("common.saving") : t("groups.uploadAsset")}
            </Button>
            <FileInput
              inputRef={fileInput}
              kind={kind}
              onFile={(file) => void upload.start(file)}
            />
            <UploadStatus
              state={upload.state}
              kind={kind}
              onCancel={upload.cancel}
              onRetry={() => fileInput.current?.click()}
            />
            {kind === "image" && (
              <FieldDescription>{t("groups.imageLimit")}</FieldDescription>
            )}
          </Field>
        </FieldGroup>
        {insertError ? (
          <Alert>
            <AlertDescription>{insertError}</AlertDescription>
          </Alert>
        ) : null}
        {chosen ? (
          <p role="status" className="text-sm">
            {t("groups.selectedAsset", { name: chosen.originalFilename })}
          </p>
        ) : null}
      </DialogShellBody>
      <DialogShellFooter>
        <Button variant="outline" onClick={onClose}>
          {t("common.cancel")}
        </Button>
        <Button
          disabled={!chosen || !label.trim() || upload.busy}
          onClick={() => chosen && onInsert(chosen, label.trim())}
        >
          {t("groups.insertAsset")}
        </Button>
      </DialogShellFooter>
    </DialogShell>
  );
}
