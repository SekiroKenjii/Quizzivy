import { useTranslation } from "react-i18next";
import { useLazyList } from "@/hooks/useLazyList";
import { LoadMoreSentinel } from "@/components/shared/LoadMoreSentinel";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { listMedia, type MediaAsset } from "@/features/media/api";
import { formatBytes } from "@/features/media/format";
import { audioLength } from "@/lib/i18n/datetime";

interface AssetLibraryDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onPick: (asset: MediaAsset) => void;
}

/**
 * Picks an existing library asset.
 *
 * Picking rather than re-uploading is what keeps one file shared across
 * questions: assets are immutable and a re-upload creates a second row pointing
 * at a second object (§11.1).
 */
export function AssetLibraryDialog({
  open,
  onOpenChange,
  onPick,
}: Readonly<AssetLibraryDialogProps>) {
  const { t } = useTranslation();
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("media.pickTitle")}</DialogTitle>
        </DialogHeader>
        <AssetList
          onPick={(asset) => {
            onPick(asset);
            onOpenChange(false);
          }}
        />
      </DialogContent>
    </Dialog>
  );
}

// Separate so the query mounts when the dialog opens rather than on every
// question editor render.
function AssetList({ onPick }: Readonly<{ onPick: (asset: MediaAsset) => void }>) {
  const { t } = useTranslation();
  const library = useLazyList({
    queryKey: ["admin-media", "picker"],
    fetchPage: (page, signal) => listMedia({ kind: "audio", page, limit: 24 }, signal),
  });

  if (library.isPending) {
    return (
      <p className="text-muted-foreground text-sm" role="status" aria-live="polite">
        {t("media.loading")}
      </p>
    );
  }
  if (library.isError) {
    return (
      <p role="alert" className="text-destructive text-sm">
        {t("media.loadFailed")}
      </p>
    );
  }
  if (library.items.length === 0) {
    return <p className="text-muted-foreground text-sm">{t("media.empty")}</p>;
  }

  return (
    <ul className="max-h-80 space-y-1 overflow-y-auto">
      {library.items.map((asset) => (
        <li key={asset.id}>
          <button
            type="button"
            onClick={() => onPick(asset)}
            className="hover:bg-accent focus-visible:ring-ring flex w-full items-center justify-between gap-4 rounded-md px-3 py-2 text-left text-sm focus-visible:ring-2 focus-visible:outline-none"
          >
            <span className="truncate">{asset.originalFilename}</span>
            <span className="text-muted-foreground shrink-0 tabular-nums">
              {audioLength(asset.durationMs)} · {formatBytes(asset.bytes)}
            </span>
          </button>
        </li>
      ))}
      <LoadMoreSentinel
        as="li"
        active={library.hasMore}
        loading={library.loadingMore}
        onVisible={library.loadMore}
      />
    </ul>
  );
}
