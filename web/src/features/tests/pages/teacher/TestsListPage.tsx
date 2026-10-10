import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "react-router";
import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { Archive, ClipboardPaste, FileUp, History, Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DeckScale } from "@/components/ui/deck-scale";
import { Segmented } from "@/components/ui/segmented";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/sonner";
import { BulkActions } from "@/components/shared/BulkActions";
import { CardGrid } from "@/components/shared/CardGrid";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { DeleteItemDialog } from "@/components/shared/DeleteItemButton";
import { EmptyState, LoadError, QueryStates } from "@/components/shared/ListState";
import { Pager } from "@/components/shared/Pager";
import { SearchInput } from "@/components/shared/SearchInput";
import { useBulkSelection } from "@/hooks/useBulkSelection";
import { useListFilters } from "@/hooks/useListFilters";
import { usePage } from "@/hooks/usePage";
import { PageHead } from "@/layouts/shell/PageHead";
import { ApiError } from "@/lib/api/errors";
import { useDisplayTimeZone } from "@/lib/i18n/datetime";
import { useDebounced } from "@/lib/useDebounced";
import { useImportAvailability } from "@/features/imports/availability";
import {
  archiveTest,
  createTest,
  deleteTest,
  duplicateTest,
  listTests,
  restoreTest,
  type Test,
} from "@/features/tests/api";
import { TestCard } from "@/features/tests/pages/teacher/TestCard";

const PAGE_SIZE = 24;
const TABS = ["all", "published", "draft"] as const;
type Tab = (typeof TABS)[number];
const TAB_LABELS = {
  all: "tests.all",
  published: "status.test.published",
  draft: "tests.draftsTab",
} as const;

const SEARCH =
  "[&_input]:bg-card [&_input]:border-border max-w-[280px] flex-[0_1_280px] [&_input]:h-8.5 [&_input]:pl-8.25 [&_input]:text-sm [&_svg]:top-[9.5px] [&_svg]:size-3.75";

const GHOST =
  "text-muted-fg hover:bg-muted hover:text-fg dark:hover:bg-muted in-data-[scale=deck]:px-3";

function toTab(value: string | null): Tab {
  return TABS.find((tab) => tab === value) ?? "all";
}

function menuTrigger(id: string): HTMLElement | null {
  return document.querySelector<HTMLElement>(`[data-test-menu="${id}"]`);
}

function message(cause: unknown, fallback: string): string {
  return cause instanceof ApiError ? cause.message : fallback;
}

/**
 * TestsListPage is the teacher's tests, as the deck's Tests screen draws them:
 * the ways to start a test (Word or PDF, paste, or from nothing), the status
 * tabs with their counts, a search by title, and the tests as cards. Every
 * test, archived ones included, is under All. A card can be selected for the
 * bulk archive and delete, duplicated in place, and archived, restored or
 * deleted from its menu.
 */
export default function TestsListPage() {
  useDisplayTimeZone();
  const { t } = useTranslation();
  const navigate = useNavigate();
  const client = useQueryClient();
  const imports = useImportAvailability();
  const bulk = useBulkSelection<Test>();
  const { params, setParams, setFilter } = useListFilters();
  const tab = toTab(params.get("status"));
  const query = params.get("q") ?? "";
  const search = useDebounced(query.trim(), 300);
  const [page] = usePage(JSON.stringify({ tab, search }));
  const tests = useQuery({
    queryKey: ["admin-tests", { tab, search, page }],
    queryFn: ({ signal }) =>
      listTests(
        {
          limit: PAGE_SIZE,
          page,
          ...(tab === "all" ? {} : { status: tab }),
          ...(search === "" ? {} : { q: search }),
        },
        signal,
      ),
    placeholderData: keepPreviousData,
  });
  const newButton = useRef<HTMLButtonElement>(null);
  const returnTo = useRef<HTMLElement | null>(null);
  const [archiving, setArchiving] = useState<Test | null>(null);
  const [deleting, setDeleting] = useState<Test | null>(null);
  const [duplicated, setDuplicated] = useState<ReadonlySet<string>>(new Set());

  const invalidate = () => client.invalidateQueries({ queryKey: ["admin-tests"] });
  const besideRemoved = (removed: Test): HTMLElement | null => {
    const items = tests.data?.items ?? [];
    const index = items.findIndex((item) => item.id === removed.id);
    const neighbour = items[index + 1] ?? items[index - 1];
    return (
      (neighbour === undefined ? null : menuTrigger(neighbour.id)) ?? newButton.current
    );
  };
  const fromMenu = (open: (test: Test) => void) => (test: Test) => {
    returnTo.current = menuTrigger(test.id);
    open(test);
  };

  const create = useMutation({
    mutationFn: () => createTest(t("tests.untitled")),
    onSuccess: async (test) => {
      await invalidate();
      void navigate(`/teacher/tests/${test.id}/edit`);
    },
    onError: (cause) => toast(message(cause, t("tests.createFailed"))),
  });

  const duplicate = useMutation({
    mutationFn: (test: Test) => duplicateTest(test.id),
    onSuccess: async (copy, source) => {
      setDuplicated((current) => new Set([...current, source.id, copy.id]));
      await invalidate();
      toast(t("common.justDuplicated"));
    },
    onError: (cause) => toast(message(cause, t("tests.duplicateFailed"))),
  });

  const restore = useMutation({
    mutationFn: (test: Test) => restoreTest(test),
    onSuccess: async () => {
      await invalidate();
      toast(t("tests.restored"));
    },
    onError: (cause) => toast(message(cause, t("tests.restoreFailed"))),
  });

  const archive = useMutation({
    mutationFn: (test: Test) => archiveTest(test),
    onSuccess: async (archived, test) => {
      if (tab !== "all") returnTo.current = besideRemoved(test);
      await invalidate();
      setArchiving(null);
      toast(
        t("tests.archived"),
        test.status === "draft"
          ? {
              action: {
                label: t("common.undo"),
                onClick: () => restore.mutate(archived),
              },
            }
          : undefined,
      );
    },
    onError: (cause) => {
      setArchiving(null);
      toast(message(cause, t("tests.archiveFailed")));
    },
  });

  const facets = tests.data?.facets;
  const filtered = search !== "" || tab !== "all";
  const clearFilters = () =>
    setParams(
      (current) => {
        const next = new URLSearchParams(current);
        next.delete("q");
        next.delete("status");
        next.delete("page");
        return next;
      },
      { replace: true },
    );
  const createNew = () => create.mutate();

  const results = (data: NonNullable<typeof tests.data>) => {
    if (data.items.length > 0)
      return (
        <>
          <CardGrid
            label={t("teacherShell.nav.tests")}
            items={data.items}
            itemKey={(test) => test.id}
            min={290}
          >
            {(test) => (
              <TestCard
                test={test}
                selection={bulk}
                duplicated={duplicated.has(test.id)}
                duplicating={duplicate.isPending}
                onDuplicate={(source) => duplicate.mutate(source)}
                onArchive={fromMenu(setArchiving)}
                onRestore={(archived) => restore.mutate(archived)}
                onDelete={fromMenu(setDeleting)}
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
            <Button size="sm" variant="outline" onClick={clearFilters}>
              {t("tests.clearFilters")}
            </Button>
          }
        >
          {t("tests.noMatches")}
        </EmptyState>
      );
    return (
      <EmptyState
        action={
          <Button size="sm" disabled={create.isPending} onClick={createNew}>
            <Plus aria-hidden="true" />
            {t("tests.new")}
          </Button>
        }
      >
        {t("tests.empty")}
      </EmptyState>
    );
  };

  return (
    <DeckScale className="mx-auto flex w-full max-w-[1320px] min-w-0 flex-col gap-4">
      <PageHead
        title={t("teacherShell.nav.tests")}
        description={t("tests.description")}
        actions={
          <>
            {imports === "reviewOnly" || imports === "on" ? (
              <Button asChild variant="ghost" className={GHOST}>
                <Link to="/teacher/imports">
                  <History aria-hidden="true" />
                  {t("tests.importHistory")}
                </Link>
              </Button>
            ) : null}
            {imports === "on" ? (
              <>
                <Button asChild variant="outline">
                  <Link to="/teacher/imports/new">
                    <FileUp aria-hidden="true" />
                    {t("tests.importWord")}
                  </Link>
                </Button>
                <Button asChild variant="outline">
                  <Link to="/teacher/imports/new?source=paste">
                    <ClipboardPaste aria-hidden="true" />
                    {t("tests.pasteTest")}
                  </Link>
                </Button>
              </>
            ) : null}
            <Button ref={newButton} disabled={create.isPending} onClick={createNew}>
              <Plus aria-hidden="true" />
              {t("tests.new")}
            </Button>
          </>
        }
      />

      <div className="flex flex-wrap items-center justify-between gap-2.5">
        <Segmented
          label={t("tests.statusFilter")}
          scroll
          value={tab}
          options={TABS.map((value) => ({
            value,
            label: t(TAB_LABELS[value]),
            count: facets?.[value],
          }))}
          onChange={(value) => setFilter("status", value === "all" ? null : value)}
        />
        <SearchInput
          className={SEARCH}
          value={query}
          onChange={(value) => setFilter("q", value)}
          placeholder={t("tests.searchPlaceholder")}
        />
      </div>

      <BulkActions
        selected={[...bulk.selected.values()]}
        name={(item) => item.title}
        actions={[
          {
            label: t("common.archiveSelected"),
            description: t("common.archiveSelectedBody"),
            icon: Archive,
            run: archiveTest,
          },
          {
            label: t("common.deletePermanently"),
            description: t("common.deleteInactiveBody"),
            icon: Trash2,
            run: (item) => deleteTest(item.id),
          },
        ]}
        onRemoved={bulk.remove}
        onClear={bulk.clear}
        onSettled={invalidate}
      />

      {tests.data === undefined ? (
        <QueryStates
          query={tests}
          skeleton={<TestsSkeleton />}
          failed={t("tests.loadFailed")}
        >
          {results}
        </QueryStates>
      ) : (
        <>
          {tests.isError ? (
            <LoadError error={tests.error} onRetry={() => void tests.refetch()}>
              {t("tests.loadFailed")}
            </LoadError>
          ) : null}
          {results(tests.data)}
        </>
      )}

      <ConfirmDialog
        open={archiving !== null}
        onOpenChange={(open) => !open && setArchiving(null)}
        title={t("tests.archiveConfirmTitle", { title: archiving?.title ?? "" })}
        description={t("tests.archiveConfirmBody")}
        confirmLabel={t("tests.archive")}
        destructive
        pending={archive.isPending}
        returnFocus={returnTo}
        onConfirm={() => archiving && archive.mutate(archiving)}
      />
      <DeleteItemDialog
        open={deleting !== null}
        onOpenChange={(open) => !open && setDeleting(null)}
        name={deleting?.title ?? ""}
        returnFocus={returnTo}
        onDelete={async () => {
          if (deleting === null) return;
          await deleteTest(deleting.id);
          returnTo.current = besideRemoved(deleting);
        }}
        onDeleted={invalidate}
      />
    </DeckScale>
  );
}

function TestsSkeleton() {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      aria-live="polite"
      aria-label={t("common.loading")}
      className="grid grid-cols-[repeat(auto-fill,minmax(min(290px,100%),1fr))] gap-3 focus-within:[&_[data-slot=skeleton]]:[animation-play-state:paused]! hover:[&_[data-slot=skeleton]]:[animation-play-state:paused]!"
    >
      {Array.from({ length: 6 }, (_, index) => (
        <Skeleton key={index} className="h-40 rounded-xl" />
      ))}
    </div>
  );
}
