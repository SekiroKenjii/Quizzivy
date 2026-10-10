import { useState } from "react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { AudioLines, Check, Upload } from "lucide-react";
import { DialogShell, DialogShellHeader } from "@/components/shared/form/DialogShell";
import { LoadMoreSentinel } from "@/components/shared/LoadMoreSentinel";
import { SearchInput } from "@/components/shared/SearchInput";
import { Button } from "@/components/ui/button";
import { Segmented } from "@/components/ui/segmented";
import { listMedia, type LibraryAsset, type MediaKind } from "@/features/media/api";
import { assetMeta } from "@/features/media/format";
import { useDebounced } from "@/lib/useDebounced";
import { cn } from "@/lib/utils";

/** LibraryKind is the picker's kind filter: every file, or one kind. */
export type LibraryKind = "all" | MediaKind;

const PAGE_SIZE = 24;
const KINDS: readonly LibraryKind[] = ["all", "image", "audio"];
const KIND_LABELS: Record<LibraryKind, string> = {
  all: "media.picker.all",
  image: "media.picker.images",
  audio: "media.picker.audio",
};

/**
 * AssetLibraryDialog is "Choose from Media": the caller's own files, filtered
 * by kind with counts that follow the search, searched by name, paged as the
 * grid scrolls. One file is selected at a time; "Attach" or a double-click
 * picks it, and "Upload new" closes the dialog and hands its kind to
 * `onUploadNew`. It opens on `kind`, "all" unless the host names one. Picking
 * rather than re-uploading keeps one file shared across questions (§11.1).
 */
export function AssetLibraryDialog({
  open,
  onOpenChange,
  onPick,
  onUploadNew,
  kind = "all",
}: Readonly<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onPick: (asset: LibraryAsset) => void;
  onUploadNew?: ((kind: LibraryKind) => void) | undefined;
  kind?: LibraryKind;
}>) {
  const { t } = useTranslation();
  return (
    <DialogShell open={open} onOpenChange={onOpenChange} width={760}>
      <DialogShellHeader
        title={t("media.picker.title")}
        description={t("media.picker.description")}
      />
      {open ? (
        <Picker
          initialKind={kind}
          onCancel={() => onOpenChange(false)}
          onPick={(asset) => {
            onOpenChange(false);
            onPick(asset);
          }}
          onUploadNew={
            onUploadNew &&
            ((chosen) => {
              onOpenChange(false);
              onUploadNew(chosen);
            })
          }
        />
      ) : null}
    </DialogShell>
  );
}

function Picker({
  initialKind,
  onCancel,
  onPick,
  onUploadNew,
}: Readonly<{
  initialKind: LibraryKind;
  onCancel: () => void;
  onPick: (asset: LibraryAsset) => void;
  onUploadNew: ((kind: LibraryKind) => void) | undefined;
}>) {
  const { t } = useTranslation();
  const [kind, setKind] = useState<LibraryKind>(initialKind);
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<LibraryAsset | null>(null);
  const search = useDebounced(query.trim(), 300);
  const library = useInfiniteQuery({
    queryKey: ["admin-media", "picker", kind, search],
    initialPageParam: 1,
    queryFn: ({ pageParam, signal }) =>
      listMedia(
        {
          ...(kind === "all" ? {} : { kind }),
          ...(search === "" ? {} : { q: search }),
          page: pageParam,
          limit: PAGE_SIZE,
        },
        signal,
      ),
    getNextPageParam: (last) =>
      last.page * last.pageSize < last.total ? last.page + 1 : undefined,
  });
  const facets = library.data?.pages[0]?.facets;
  const items = library.data?.pages.flatMap((page) => page.items) ?? [];

  return (
    <>
      <div className="flex flex-wrap items-center justify-between gap-2.5 px-4.5 pt-2 pb-3">
        <Segmented
          label={t("media.picker.kind")}
          value={kind}
          onChange={(next) => {
            setKind(next as LibraryKind);
            setSelected(null);
          }}
          options={KINDS.map((value) => ({
            value,
            label: t(KIND_LABELS[value]),
            count: facets?.[value],
          }))}
        />
        <SearchInput
          value={query}
          onChange={setQuery}
          placeholder={t("media.picker.search")}
          dense
          className="w-auto min-w-45 flex-[0_1_260px]"
        />
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto px-4.5 pt-0.5 pb-4">
        <PickerBody
          status={library.status}
          items={items}
          empty={facets !== undefined && facets.all === 0 && search === ""}
          selected={selected}
          hasMore={library.hasNextPage}
          loadingMore={library.isFetchingNextPage}
          onRetry={() => void library.refetch()}
          onLoadMore={() => {
            if (library.hasNextPage)
              void library.fetchNextPage({ cancelRefetch: false });
          }}
          onSelect={setSelected}
          onPick={onPick}
        />
      </div>
      <div className="flex flex-wrap items-center gap-2 border-t px-4.5 py-3">
        {onUploadNew ? (
          <Button type="button" variant="outline" onClick={() => onUploadNew(kind)}>
            <Upload aria-hidden="true" />
            {t("media.picker.uploadNew")}
          </Button>
        ) : null}
        <span className="ml-auto flex gap-2">
          <Button type="button" variant="outline" onClick={onCancel}>
            {t("common.cancel")}
          </Button>
          <Button
            type="button"
            disabled={selected === null}
            onClick={() => {
              if (selected) onPick(selected);
            }}
          >
            {t("media.picker.attach")}
          </Button>
        </span>
      </div>
    </>
  );
}

function PickerBody({
  status,
  items,
  empty,
  selected,
  hasMore,
  loadingMore,
  onRetry,
  onLoadMore,
  onSelect,
  onPick,
}: Readonly<{
  status: "pending" | "error" | "success";
  items: LibraryAsset[];
  empty: boolean;
  selected: LibraryAsset | null;
  hasMore: boolean;
  loadingMore: boolean;
  onRetry: () => void;
  onLoadMore: () => void;
  onSelect: (asset: LibraryAsset) => void;
  onPick: (asset: LibraryAsset) => void;
}>) {
  const { t } = useTranslation();
  if (status === "pending")
    return (
      <div
        role="status"
        aria-label={t("media.loading")}
        className="grid grid-cols-[repeat(auto-fill,minmax(160px,1fr))] gap-2.5"
      >
        {Array.from({ length: 6 }, (_, index) => (
          <span key={index} className="bg-muted h-38 animate-pulse rounded-[10px]" />
        ))}
      </div>
    );
  if (status === "error")
    return (
      <div role="alert" className="flex flex-col items-center gap-3 py-6 text-center">
        <p className="text-sm">{t("media.loadFailed")}</p>
        <Button type="button" variant="outline" size="sm" onClick={onRetry}>
          {t("common.retry")}
        </Button>
      </div>
    );
  if (items.length === 0)
    return (
      <p className="text-muted-fg my-6 text-center text-[13.5px]">
        {t(empty ? "media.picker.emptyLibrary" : "media.picker.noMatch")}
      </p>
    );
  return (
    <ul className="grid grid-cols-[repeat(auto-fill,minmax(160px,1fr))] gap-2.5">
      {items.map((asset) => (
        <li key={asset.id} className="min-w-0">
          <AssetTile
            asset={asset}
            selected={selected?.id === asset.id}
            onSelect={() => onSelect(asset)}
            onPick={() => onPick(asset)}
          />
        </li>
      ))}
      <LoadMoreSentinel
        as="li"
        active={hasMore}
        loading={loadingMore}
        onVisible={onLoadMore}
      />
    </ul>
  );
}

function AssetTile({
  asset,
  selected,
  onSelect,
  onPick,
}: Readonly<{
  asset: LibraryAsset;
  selected: boolean;
  onSelect: () => void;
  onPick: () => void;
}>) {
  const { t } = useTranslation();
  return (
    <button
      type="button"
      aria-pressed={selected}
      onClick={onSelect}
      onDoubleClick={onPick}
      className={cn(
        "bg-card text-fg relative flex w-full flex-col overflow-hidden rounded-[10px] border text-left",
        selected ? "border-primary ring-primary ring-1" : "border-border",
      )}
    >
      <span className="bg-muted text-muted-fg relative grid h-23 place-items-center">
        {asset.kind === "image" ? (
          <img
            src={asset.url}
            alt=""
            loading="lazy"
            decoding="async"
            className="absolute inset-0 size-full object-cover"
          />
        ) : (
          <AudioLines aria-hidden="true" className="size-6" />
        )}
      </span>
      <span className="flex min-w-0 flex-col gap-px px-2.5 py-2">
        <span className="truncate text-[12.5px] font-medium">{asset.displayName}</span>
        <span className="text-muted-fg truncate text-[11.5px]">
          {assetMeta(asset, t)}
        </span>
        <span className="text-muted-fg truncate text-[11.5px]">
          {asset.questionCount === 0
            ? t("media.picker.notUsedYet")
            : t("media.usedInQuestions", { count: asset.questionCount })}
        </span>
      </span>
      {selected ? (
        <span
          aria-hidden="true"
          className="bg-primary text-primary-foreground absolute top-2 right-2 grid size-5.5 place-items-center rounded-full"
        >
          <Check className="size-3.25" />
        </span>
      ) : null}
    </button>
  );
}
