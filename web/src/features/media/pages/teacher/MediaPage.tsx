import { useState } from "react";
import { useTranslation } from "react-i18next";
import { keepPreviousData, useQuery, useQueryClient } from "@tanstack/react-query";
import { Upload } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Segmented } from "@/components/ui/segmented";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/sonner";
import { CardGrid } from "@/components/shared/CardGrid";
import { EmptyState, LoadError, QueryStates } from "@/components/shared/ListState";
import { Pager } from "@/components/shared/Pager";
import { SearchInput } from "@/components/shared/SearchInput";
import { PageHead } from "@/layouts/shell/PageHead";
import { useFileDrop } from "@/hooks/useFileDrop";
import { useListFilters } from "@/hooks/useListFilters";
import { usePage } from "@/hooks/usePage";
import { useDebounced } from "@/lib/useDebounced";
import { listMedia, type LibraryAsset, type MediaUsage } from "@/features/media/api";
import { DeleteMediaDialog } from "@/features/media/components/DeleteMediaDialog";
import { MediaCard } from "@/features/media/components/MediaCard";
import { RenameDialog } from "@/features/media/components/RenameDialog";
import { ReplaceDialog } from "@/features/media/components/ReplaceDialog";
import { UploadDialog } from "@/features/media/components/UploadDialog";
import { formatStorage } from "@/features/media/format";

const PAGE_SIZE = 24;
const TABS = ["all", "audio", "image", "unused"] as const;
type Tab = (typeof TABS)[number];
const TAB_LABELS = {
  all: "media.tabAll",
  audio: "media.tabAudio",
  image: "media.tabImages",
  unused: "media.tabUnused",
} as const;
const EDITED_BY_REPLACE = [
  "admin-media",
  "admin-question",
  "admin-questions",
  "admin-group",
  "admin-groups",
  "admin-test",
  "admin-test-preview",
] as const;

function toTab(value: string | null): Tab {
  return TABS.find((tab) => tab === value) ?? "all";
}

/**
 * MediaPage is the teacher's media library: storage against the quota, the
 * files as cards filtered by kind or by use and searched by name, and the
 * upload, rename, replace, download and delete of a file.
 */
export default function MediaPage() {
  const { t } = useTranslation();
  const client = useQueryClient();
  const { params, setParams, setFilter } = useListFilters();
  const query = params.get("q") ?? "";
  const search = useDebounced(query.trim(), 300);
  const tab = toTab(params.get("tab"));
  const [page] = usePage(JSON.stringify({ search, tab }));
  const library = useQuery({
    queryKey: ["admin-media", { search, tab, page }],
    queryFn: ({ signal }) =>
      listMedia(
        {
          page,
          limit: PAGE_SIZE,
          ...(search === "" ? {} : { q: search }),
          ...(tab === "audio" || tab === "image" ? { kind: tab } : {}),
          ...(tab === "unused" ? { unused: true } : {}),
        },
        signal,
      ),
    placeholderData: keepPreviousData,
  });
  const [uploading, setUploading] = useState<{ file: File | null } | null>(null);
  const [renaming, setRenaming] = useState<LibraryAsset | null>(null);
  const [replacing, setReplacing] = useState<LibraryAsset | null>(null);
  const [deleting, setDeleting] = useState<LibraryAsset | null>(null);
  const [playing, setPlaying] = useState<string | null>(null);
  const dialogOpen =
    uploading !== null || renaming !== null || replacing !== null || deleting !== null;
  const dragging = useFileDrop((files) => {
    if (files.length === 1) setUploading({ file: files[0]! });
    else toast(t(files.length === 0 ? "media.rejectFolder" : "media.rejectMany"));
  }, !dialogOpen);

  const refresh = () => void client.invalidateQueries({ queryKey: ["admin-media"] });
  const refreshEdited = () => {
    for (const key of EDITED_BY_REPLACE)
      void client.invalidateQueries({ queryKey: [key] });
  };
  const upload = () => setUploading({ file: null });
  const facets = library.data?.facets;
  const filtered = search !== "" || tab !== "all";

  const results = (data: NonNullable<typeof library.data>) => {
    if (data.items.length > 0)
      return (
        <>
          <CardGrid
            label={t("media.title")}
            items={data.items}
            itemKey={(asset) => asset.id}
            min={240}
          >
            {(asset) => (
              <MediaCard
                asset={asset}
                active={playing === asset.id}
                onPlay={setPlaying}
                onExpired={() => void library.refetch()}
                onRename={setRenaming}
                onReplace={setReplacing}
                onDelete={setDeleting}
              />
            )}
          </CardGrid>
          <Pager page={data.page} pageSize={data.pageSize} total={data.total} />
        </>
      );
    if (filtered)
      return (
        <EmptyState
          action={
            <Button
              size="sm"
              variant="outline"
              onClick={() =>
                setParams(
                  (current) => {
                    const next = new URLSearchParams(current);
                    next.delete("q");
                    next.delete("tab");
                    next.delete("page");
                    return next;
                  },
                  { replace: true },
                )
              }
            >
              {t("media.clearFilters")}
            </Button>
          }
        >
          {t("media.noMatches")}
        </EmptyState>
      );
    return (
      <EmptyState
        hint={t("media.emptyHint")}
        action={
          <Button size="sm" onClick={upload}>
            <Upload aria-hidden="true" />
            {t("media.upload")}
          </Button>
        }
      >
        {t("media.emptyTitle")}
      </EmptyState>
    );
  };

  return (
    <div
      className="mx-auto flex w-full max-w-[1320px] min-w-0 flex-col gap-4"
      data-scale="deck"
    >
      <PageHead
        title={t("media.title")}
        description={t("media.description")}
        actions={
          <Button onClick={upload}>
            <Upload aria-hidden="true" />
            {t("media.upload")}
          </Button>
        }
      />

      {dragging ? (
        <p className="border-primary bg-muted rounded-lg border border-dashed p-3 text-center text-sm">
          {t("media.dropToUpload")}
        </p>
      ) : null}

      {library.data === undefined ? null : <StorageCard usage={library.data.usage} />}

      <div className="flex flex-wrap items-center justify-between gap-2.5">
        <Segmented
          label={t("media.filter")}
          scroll
          value={tab}
          options={TABS.map((value) => ({
            value,
            label: t(TAB_LABELS[value]),
            count: facets?.[value],
          }))}
          onChange={(value) => setFilter("tab", value === "all" ? null : value)}
        />
        <SearchInput
          className="[&_input]:bg-card [&_input]:border-border max-w-[280px] flex-[0_1_280px] [&_input]:h-8.5 [&_input]:pl-8.25 [&_input]:text-sm [&_svg]:top-[9.5px] [&_svg]:size-3.75"
          value={query}
          onChange={(value) => setFilter("q", value)}
          placeholder={t("media.searchPlaceholder")}
        />
      </div>

      {library.data === undefined ? (
        <QueryStates
          query={library}
          skeleton={<MediaSkeleton />}
          failed={t("media.loadFailed")}
        >
          {results}
        </QueryStates>
      ) : (
        <>
          {library.isError ? (
            <LoadError error={library.error} onRetry={() => void library.refetch()}>
              {t("media.loadFailed")}
            </LoadError>
          ) : null}
          {results(library.data)}
        </>
      )}

      <UploadDialog
        open={uploading !== null}
        initialFile={uploading?.file ?? null}
        onOpenChange={(open) => !open && setUploading(null)}
        onUploaded={refresh}
      />
      <RenameDialog
        asset={renaming}
        onOpenChange={(open) => !open && setRenaming(null)}
        onRenamed={refresh}
      />
      <ReplaceDialog
        asset={replacing}
        onOpenChange={(open) => !open && setReplacing(null)}
        onSettled={refreshEdited}
      />
      <DeleteMediaDialog
        asset={deleting}
        onOpenChange={(open) => !open && setDeleting(null)}
        onDeleted={() => {
          toast(t("media.deleted"));
          refresh();
        }}
      />
    </div>
  );
}

function StorageCard({ usage }: Readonly<{ usage: MediaUsage }>) {
  const { t } = useTranslation();
  const used = usage.audioBytes + usage.imageBytes;
  const share = (bytes: number) =>
    `${Math.min(100, (bytes / usage.quotaBytes) * 100)}%`;
  return (
    <section
      aria-label={t("media.storage")}
      className="bg-card shadow-card flex flex-wrap items-center gap-3.5 rounded-xl border px-4 py-3"
    >
      <span className="text-sm font-medium whitespace-nowrap">
        {t("media.storage")}
      </span>
      <span
        aria-hidden="true"
        className="bg-muted flex h-2 flex-[1_1_200px] overflow-hidden rounded-md"
      >
        <span className="bg-brand" style={{ width: share(usage.audioBytes) }} />
        <span className="bg-info" style={{ width: share(usage.imageBytes) }} />
      </span>
      <span className="text-meta text-muted-fg min-w-0 leading-normal tabular-nums">
        {t("media.storageUsage", {
          used: formatStorage(used),
          quota: formatStorage(usage.quotaBytes),
          audio: formatStorage(usage.audioBytes),
          image: formatStorage(usage.imageBytes),
        })}
      </span>
    </section>
  );
}

function MediaSkeleton() {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      aria-live="polite"
      aria-label={t("common.loading")}
      className="grid grid-cols-[repeat(auto-fill,minmax(min(240px,100%),1fr))] gap-3 focus-within:[&_[data-slot=skeleton]]:[animation-play-state:paused]! hover:[&_[data-slot=skeleton]]:[animation-play-state:paused]!"
    >
      {Array.from({ length: 6 }, (_, index) => (
        <Skeleton key={index} className="h-49 rounded-xl" />
      ))}
    </div>
  );
}
