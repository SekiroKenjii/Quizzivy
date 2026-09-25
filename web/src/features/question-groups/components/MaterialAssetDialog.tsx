import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Alert, AlertDescription } from "@/components/ui/alert";
import {
  UploadPanel,
  type UploadHandle,
} from "@/features/media/components/UploadPanel";
import {
  listMedia,
  uploadMedia,
  type MediaAsset,
  type MediaKind,
} from "@/features/media/api";
import { MAX_BYTES } from "@/features/media/limits";
import { ApiError } from "@/lib/api/errors";
import { useLazyList } from "@/hooks/useLazyList";
import { LoadMoreSentinel } from "@/components/shared/LoadMoreSentinel";
import { formatBytes } from "@/features/media/format";
import { CONTENT_LIMITS } from "@/components/shared/content/model";

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
  const [error, setError] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const controller = useRef<AbortController | null>(null);
  const imageInput = useRef<HTMLInputElement>(null);
  const audioUpload = useRef<UploadHandle>(null);
  const library = useLazyList({
    queryKey: ["admin-media", "group-picker", kind],
    fetchPage: (page, signal) => listMedia({ kind, page, limit: 20 }, signal),
  });
  useEffect(() => () => controller.current?.abort(), []);

  async function uploadImage(file: File) {
    setError(null);
    if (file.size > MAX_BYTES) {
      setError(t("groups.imageLimit"));
      return;
    }
    const abort = new AbortController();
    controller.current = abort;
    setUploading(true);
    try {
      const asset = await uploadMedia(file, { signal: abort.signal });
      if (!abort.signal.aborted) {
        if (asset.kind === "image") setChosen(asset);
        else setError(t("groups.imageLimit"));
      }
    } catch (cause) {
      if (!abort.signal.aborted)
        setError(cause instanceof ApiError ? cause.message : t("groups.uploadFailed"));
    } finally {
      if (!abort.signal.aborted) setUploading(false);
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="flex max-h-[85svh] flex-col overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>
            {t(kind === "image" ? "groups.insertImage" : "groups.insertAudio")}
          </DialogTitle>
          <DialogDescription>{t("groups.assetHint")}</DialogDescription>
        </DialogHeader>
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor="group-asset-label">
              {t(kind === "image" ? "groups.imageLabel" : "groups.audioLabel")}
            </FieldLabel>
            <Input
              id="group-asset-label"
              maxLength={
                kind === "image" ? CONTENT_LIMITS.imageAlt : CONTENT_LIMITS.audioLabel
              }
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
                <p className="text-muted-foreground text-sm">{t("media.empty")}</p>
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
              disabled={uploading}
              onClick={() =>
                kind === "audio"
                  ? audioUpload.current?.choose()
                  : imageInput.current?.click()
              }
            >
              {uploading ? t("common.saving") : t("groups.uploadAsset")}
            </Button>
            {kind === "audio" ? (
              <UploadPanel ref={audioUpload} onUploaded={setChosen} />
            ) : (
              <>
                <input
                  ref={imageInput}
                  type="file"
                  accept=".png,.jpg,.jpeg,.webp,image/png,image/jpeg,image/webp"
                  className="hidden"
                  aria-label={t("groups.uploadAsset")}
                  onChange={(event) => {
                    const file = event.target.files?.[0];
                    event.target.value = "";
                    if (file) void uploadImage(file);
                  }}
                />
                <FieldDescription>{t("groups.imageLimit")}</FieldDescription>
              </>
            )}
          </Field>
        </FieldGroup>
        {error || insertError ? (
          <Alert>
            <AlertDescription>{error ?? insertError}</AlertDescription>
          </Alert>
        ) : null}
        {chosen ? (
          <p role="status" className="text-sm">
            {t("groups.selectedAsset", { name: chosen.originalFilename })}
          </p>
        ) : null}
        <div className="flex justify-end gap-2">
          <Button variant="outline" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button
            disabled={!chosen || !label.trim() || uploading}
            onClick={() => chosen && onInsert(chosen, label.trim())}
          >
            {t("groups.insertAsset")}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
