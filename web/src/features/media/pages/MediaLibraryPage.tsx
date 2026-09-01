import { Fragment, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useInfiniteQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Eye, FileAudio, FileImage, Play, Trash2, Upload } from "lucide-react";
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
import { AudioPreviewRow } from "@/features/question-bank/components/AudioPreviewRow";
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
  const [blocked, setBlocked] = useState<LibraryAsset | null>(null);
  const [playing, setPlaying] = useState<string | null>(null);
  const [viewing, setViewing] = useState<LibraryAsset | null>(null);

  // Paged, not a flat limit of 100: a library grows for as long as the teacher
  // keeps teaching, and "100 tệp" was being printed as if it were the total.
  const library = useInfiniteQuery({
    queryKey: ["admin-media"],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) =>
      listMedia({ limit: 50, ...(pageParam ? { cursor: pageParam } : {}) }, signal),
    getNextPageParam: (page) => page.nextCursor ?? undefined,
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

  const assets = library.data?.pages.flatMap((page) => page.items) ?? [];
  const totalBytes = assets.reduce((sum, asset) => sum + asset.bytes, 0);
  const dragging = useFileDrop((files) => uploader.current?.dropped(files));

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
            {library.isSuccess
              ? t(library.hasNextPage ? "media.summarySoFar" : "media.summary", {
                  count: assets.length,
                  size: formatBytes(totalBytes),
                })
              : "\u00a0"}
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
            playing={playing}
            onRefresh={() => void library.refetch()}
            onBlocked={setBlocked}
            onView={setViewing}
            onTogglePlay={(asset) => setPlaying(playing === asset.id ? null : asset.id)}
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

      {library.hasNextPage ? (
        <Button
          variant="outline"
          size="sm"
          disabled={library.isFetchingNextPage}
          onClick={() => void library.fetchNextPage()}
        >
          {library.isFetchingNextPage ? t("common.loading") : t("bank.loadMore")}
        </Button>
      ) : null}

      {/* A-07 makes the blocked delete explain itself rather than sit inert. */}
      <Dialog
        open={blocked !== null}
        onOpenChange={(open) => !open && setBlocked(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("media.blockedTitle")}</DialogTitle>
            <DialogDescription>
              {blocked
                ? t("media.blockedBody", {
                    name: blocked.originalFilename,
                    count: blocked.usageCount ?? 0,
                  })
                : ""}
            </DialogDescription>
          </DialogHeader>
          <p className="text-muted-foreground text-sm leading-relaxed">
            {t("media.blockedNote")}
          </p>
          <DialogFooter>
            <Button className="w-full" onClick={() => setBlocked(null)}>
              {t("media.understood")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={viewing !== null}
        onOpenChange={(open) => !open && setViewing(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{viewing?.originalFilename ?? ""}</DialogTitle>
          </DialogHeader>
          {viewing ? (
            <img
              src={viewing.url}
              alt={viewing.originalFilename}
              className="max-h-[60vh] w-full rounded-md object-contain"
            />
          ) : null}
        </DialogContent>
      </Dialog>
    </div>
  );
}

function AssetTable({
  assets,
  playing,
  onDelete,
  onBlocked,
  onTogglePlay,
  onView,
  onRefresh,
}: {
  assets: LibraryAsset[];
  playing: string | null;
  onDelete: (asset: LibraryAsset) => void;
  onBlocked: (asset: LibraryAsset) => void;
  onTogglePlay: (asset: LibraryAsset) => void;
  onView: (asset: LibraryAsset) => void;
  onRefresh: () => void;
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
            <Fragment key={asset.id}>
              <TableRow>
                <TableCell>
                  <div className="flex items-center gap-2">
                    <Icon
                      className="text-muted-foreground size-3.5 shrink-0"
                      aria-hidden="true"
                    />
                    <span className="truncate font-medium">
                      {asset.originalFilename}
                    </span>
                  </div>
                </TableCell>
                <TableCell className="text-muted-foreground">
                  {asset.mimeType}
                </TableCell>
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
                  <div className="flex justify-end gap-0.5">
                    {asset.kind === "audio" ? (
                      <Button
                        variant="ghost"
                        size="icon-xs"
                        aria-label={t("media.previewNamed", {
                          name: asset.originalFilename,
                        })}
                        aria-pressed={playing === asset.id}
                        onClick={() => onTogglePlay(asset)}
                      >
                        <Play aria-hidden="true" />
                      </Button>
                    ) : (
                      <Button
                        variant="ghost"
                        size="icon-xs"
                        aria-label={t("media.viewNamed", {
                          name: asset.originalFilename,
                        })}
                        onClick={() => onView(asset)}
                      >
                        <Eye aria-hidden="true" />
                      </Button>
                    )}
                    {/* aria-disabled rather than disabled, per A-07: pressing it
                      explains why it cannot be deleted. A disabled button just
                      refuses and leaves the teacher guessing. */}
                    <Button
                      variant="ghost"
                      size="icon-xs"
                      className={used ? "text-muted-foreground" : undefined}
                      aria-disabled={used}
                      aria-label={t("media.deleteNamed", {
                        name: asset.originalFilename,
                      })}
                      onClick={() => (used ? onBlocked(asset) : onDelete(asset))}
                    >
                      <Trash2 aria-hidden="true" />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>

              {asset.kind === "audio" && playing === asset.id ? (
                <TableRow>
                  <TableCell colSpan={7} className="p-0">
                    <AudioPreviewRow
                      key={asset.url}
                      asset={asset}
                      onRetry={onRefresh}
                      onClose={() => onTogglePlay(asset)}
                    />
                  </TableCell>
                </TableRow>
              ) : null}
            </Fragment>
          );
        })}
      </TableBody>
    </Table>
  );
}
