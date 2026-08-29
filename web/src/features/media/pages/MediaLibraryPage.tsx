import { useCallback, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { FileAudio, FileImage, Trash2, Upload } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  UploadPanel,
  type UploadHandle,
} from "@/features/media/components/UploadPanel";
import { useFileDrop } from "@/features/media/useFileDrop";
import { deleteMedia, listMedia, type LibraryAsset } from "@/features/media/api";
import { formatBytes, formatDuration, formatUploadedAt } from "@/features/media/format";
import { ApiError } from "@/lib/api/errors";

export default function MediaLibraryPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const uploader = useRef<UploadHandle>(null);
  const [error, setError] = useState<string | null>(null);
  const [confirming, setConfirming] = useState<LibraryAsset | null>(null);

  const library = useQuery({
    queryKey: ["admin-media"],
    queryFn: ({ signal }) => listMedia({ limit: 100 }, signal),
  });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["admin-media"] });

  const remove = useMutation({
    mutationFn: (id: string) => deleteMedia(id),
    onSuccess: async () => {
      setConfirming(null);
      setError(null);
      await invalidate();
    },
    onError: (cause) => {
      setConfirming(null);
      setError(cause instanceof ApiError ? cause.message : t("media.deleteFailed"));
    },
  });

  const assets = library.data?.items ?? [];
  const totalBytes = assets.reduce((sum, asset) => sum + asset.bytes, 0);
  const dragging = useFileDrop(
    useCallback((file: File) => uploader.current?.accept(file), []),
  );

  return (
    <div className="space-y-4">
      {dragging ? (
        <p className="border-primary bg-accent rounded-lg border border-dashed p-3 text-center text-sm">
          {t("media.dropHere")}
        </p>
      ) : null}

      <div className="flex items-end justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">{t("media.title")}</h1>
          <p className="text-muted-foreground mt-0.5 text-sm">
            {t("media.summary", {
              count: assets.length,
              size: formatBytes(totalBytes),
            })}
          </p>
        </div>
        <Button size="sm" onClick={() => uploader.current?.choose()}>
          <Upload aria-hidden="true" />
          {t("media.upload")}
        </Button>
      </div>

      <UploadPanel
        ref={uploader}
        onUploaded={() => {
          setError(null);
          void invalidate();
        }}
      />

      {error ? (
        <p role="alert" className="text-destructive text-sm">
          {error}
        </p>
      ) : null}

      <div className="bg-card overflow-hidden rounded-lg border">
        {library.isPending ? (
          <p
            className="text-muted-foreground p-6 text-sm"
            role="status"
            aria-live="polite"
          >
            {t("media.loading")}
          </p>
        ) : library.isError ? (
          <p role="alert" className="text-destructive p-6 text-sm">
            {t("media.loadFailed")}
          </p>
        ) : assets.length === 0 ? (
          <p className="text-muted-foreground p-6 text-sm">{t("media.empty")}</p>
        ) : (
          <AssetTable
            assets={assets}
            onDelete={(asset) => {
              setError(null);
              setConfirming(asset);
            }}
          />
        )}
      </div>

      <Dialog
        open={confirming !== null}
        onOpenChange={(open) => !open && setConfirming(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("media.deleteConfirmTitle")}</DialogTitle>
            <DialogDescription>
              {confirming ? `${confirming.originalFilename} — ` : ""}
              {t("media.deleteConfirmBody")}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirming(null)}>
              {t("common.cancel")}
            </Button>
            <Button
              disabled={remove.isPending}
              onClick={() => confirming && remove.mutate(confirming.id)}
            >
              {remove.isPending ? t("common.loading") : t("media.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function AssetTable({
  assets,
  onDelete,
}: {
  assets: LibraryAsset[];
  onDelete: (asset: LibraryAsset) => void;
}) {
  const { t } = useTranslation();

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead className="w-[34%]">{t("media.columnFile")}</TableHead>
          <TableHead>{t("media.columnType")}</TableHead>
          <TableHead className="text-right">{t("media.columnDuration")}</TableHead>
          <TableHead className="text-right">{t("media.columnSize")}</TableHead>
          <TableHead>{t("media.columnUsage")}</TableHead>
          <TableHead>{t("media.columnUploaded")}</TableHead>
          <TableHead className="w-20" />
        </TableRow>
      </TableHeader>
      <TableBody>
        {assets.map((asset) => {
          const used = (asset.usageCount ?? 0) > 0;
          const Icon = asset.kind === "audio" ? FileAudio : FileImage;
          return (
            <TableRow key={asset.id}>
              <TableCell>
                <div className="flex items-center gap-2">
                  <Icon
                    className="text-muted-foreground size-3.5 shrink-0"
                    aria-hidden="true"
                  />
                  <span className="truncate font-medium">{asset.originalFilename}</span>
                </div>
              </TableCell>
              <TableCell className="text-muted-foreground">{asset.mimeType}</TableCell>
              <TableCell className="text-right tabular-nums">
                {formatDuration(asset.durationMs)}
              </TableCell>
              <TableCell className="text-right tabular-nums">
                {formatBytes(asset.bytes)}
              </TableCell>
              <TableCell>
                <Badge>
                  {used
                    ? t("media.usedInPublished", { count: asset.usageCount ?? 0 })
                    : t("media.notUsedBadge")}
                </Badge>
              </TableCell>
              <TableCell className="text-muted-foreground">
                {formatUploadedAt(asset.createdAt)}
              </TableCell>
              <TableCell className="text-right">
                <Button
                  variant="ghost"
                  size="icon-xs"
                  disabled={used}
                  title={used ? t("media.deleteBlocked") : undefined}
                  aria-label={t("media.delete")}
                  onClick={() => onDelete(asset)}
                >
                  <Trash2 aria-hidden="true" />
                </Button>
              </TableCell>
            </TableRow>
          );
        })}
      </TableBody>
    </Table>
  );
}
