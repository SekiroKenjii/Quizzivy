import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
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
import { UploadWidget } from "@/features/media/components/UploadWidget";
import { deleteMedia, listMedia, type LibraryAsset } from "@/features/media/api";
import { formatBytes, formatDuration } from "@/features/media/format";
import { ApiError } from "@/lib/api/errors";

export default function MediaLibraryPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
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
      // Closed as well as reported: the message renders in the page body, and
      // Radix marks everything outside an open dialog aria-hidden.
      setConfirming(null);
      setError(cause instanceof ApiError ? cause.message : t("media.deleteFailed"));
    },
  });

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-semibold">{t("media.title")}</h1>

      <UploadWidget
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

      <section className="rounded-lg border p-6" aria-labelledby="library-heading">
        <h2 id="library-heading" className="sr-only">
          {t("media.title")}
        </h2>

        {library.isPending ? (
          <p className="text-muted-foreground text-sm" role="status" aria-live="polite">
            {t("media.loading")}
          </p>
        ) : library.isError ? (
          <p role="alert" className="text-destructive text-sm">
            {t("media.loadFailed")}
          </p>
        ) : library.data.items.length === 0 ? (
          <p className="text-muted-foreground text-sm">{t("media.empty")}</p>
        ) : (
          <AssetTable
            assets={library.data.items}
            onDelete={(asset) => {
              setError(null);
              setConfirming(asset);
            }}
          />
        )}
      </section>

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
          <TableHead>{t("media.columnName")}</TableHead>
          <TableHead>{t("media.columnDuration")}</TableHead>
          <TableHead>{t("media.columnSize")}</TableHead>
          <TableHead>{t("media.columnUsage")}</TableHead>
          <TableHead className="text-right" />
        </TableRow>
      </TableHeader>
      <TableBody>
        {assets.map((asset) => {
          const used = (asset.usageCount ?? 0) > 0;
          return (
            <TableRow key={asset.id}>
              <TableCell className="font-medium">{asset.originalFilename}</TableCell>
              <TableCell className="tabular-nums">
                {formatDuration(asset.durationMs)}
              </TableCell>
              <TableCell className="tabular-nums">{formatBytes(asset.bytes)}</TableCell>
              <TableCell>
                {used
                  ? t("media.usedIn", { count: asset.usageCount ?? 0 })
                  : t("media.notUsed")}
              </TableCell>
              <TableCell className="text-right">
                <Button
                  variant="ghost"
                  size="sm"
                  disabled={used}
                  title={used ? t("media.deleteBlocked") : undefined}
                  onClick={() => onDelete(asset)}
                >
                  {t("media.delete")}
                </Button>
              </TableCell>
            </TableRow>
          );
        })}
      </TableBody>
    </Table>
  );
}
