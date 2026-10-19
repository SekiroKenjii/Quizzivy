import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { LoadError } from "@/components/shared/ListState";
import { SearchInput, TOOLBAR_SEARCH } from "@/components/shared/SearchInput";
import { ChipBox } from "@/components/shared/form/ChipInput";
import { Button } from "@/components/ui/button";
import { Segmented } from "@/components/ui/segmented";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/sonner";
import {
  ADMIN_CLASS_PAGE_SIZES,
  ADMIN_CLASS_SEARCH_MAX,
  adminClassesQuery,
  invalidateAdminClasses,
  type AdminClass,
  type AdminClassPageSize,
  type AdminClassTab,
} from "@/features/classes/adminApi";
import { updateClass } from "@/features/classes/api";
import { AdminClassDialog } from "@/features/classes/components/admin/AdminClassDialog";
import {
  AdminClassesTable,
  type AdminClassActions,
} from "@/features/classes/components/admin/AdminClassesTable";
import { ChangeTeacherDialog } from "@/features/classes/components/admin/ChangeTeacherDialog";
import {
  ArchiveAdminClassDialog,
  RotateCodeDialog,
} from "@/features/classes/components/admin/ClassConfirms";
import { JoinCodesOffCallout } from "@/features/classes/components/JoinCodesOffCallout";
import { invalidateClass } from "@/features/classes/invalidate";
import { useCopyJoinCode } from "@/features/classes/joinLink";
import { useJoinCodesOff } from "@/features/organization/api";
import { useDebouncedInput } from "@/hooks/useDebouncedInput";
import { useListFilters } from "@/hooks/useListFilters";
import { usePage, usePageSize } from "@/hooks/usePage";
import { PageHead } from "@/layouts/shell/PageHead";
import { ApiError } from "@/lib/api/errors";
import { useDisplayTimeZone } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { pageRange } from "@/lib/pagination";
import { cn } from "@/lib/utils";

const PAGE_MAX = 1_000_000;

function searchOf(typed: string): string {
  return [...typed.normalize("NFC").trim()]
    .slice(0, ADMIN_CLASS_SEARCH_MAX)
    .join("")
    .trim();
}

function limitOf(size: number): AdminClassPageSize {
  return (
    ADMIN_CLASS_PAGE_SIZES.find((option) => option === size) ??
    ADMIN_CLASS_PAGE_SIZES[0]
  );
}

function message(cause: unknown, fallback: string): string {
  return cause instanceof ApiError ? cause.message : fallback;
}

/**
 * AdminClassesPage is the Admin console's Classes screen (T-R5.25): every
 * teacher's classes in one table, split into Active and Archived with their
 * counts, narrowed by a search of the class's or the teacher's name and, when
 * the Overview links here with `?teacher=disabled`, to the classes of disabled
 * teachers, shown as a removable chip. The tab (`tab`), the search (`q`), the
 * filter, the page and its size live in the URL; the search reaches it once
 * the Admin has paused for 300 ms. A page past the end goes to the last one.
 * New class creates a class for a teacher; a row's menu edits it, changes its
 * teacher, rotates its join code, and archives or restores it. Edit, rotate,
 * archive and restore are the teacher workspace's own operations. The join
 * codes are read in full, so the list leaves memory with the page.
 */
export default function AdminClassesPage() {
  useDisplayTimeZone();
  const { t } = useTranslation();
  const locale = useLocale();
  const queryClient = useQueryClient();
  const joinOff = useJoinCodesOff();
  const copyCode = useCopyJoinCode();
  const { params, setParams, setFilter } = useListFilters();
  const tab: AdminClassTab = params.get("tab") === "archived" ? "archived" : "active";
  const disabledTeacher = params.get("teacher") === "disabled";
  const applied = params.get("q") ?? "";
  const applyQuery = useCallback((value: string) => setFilter("q", value), [setFilter]);
  const [typed, setTyped] = useDebouncedInput(applied, applyQuery, searchOf);
  const q = searchOf(applied);
  const [size] = usePageSize(ADMIN_CLASS_PAGE_SIZES);
  const [asked, setPage] = usePage(JSON.stringify({ tab, disabledTeacher, q, size }));
  const page = Math.min(asked, PAGE_MAX);
  const [now] = useState(() => Date.now());
  const searchRef = useRef<HTMLInputElement>(null);
  const newButton = useRef<HTMLButtonElement>(null);
  const returnTo = useRef<HTMLElement | null>(null);
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<AdminClass | null>(null);
  const [reassigning, setReassigning] = useState<AdminClass | null>(null);
  const [rotating, setRotating] = useState<AdminClass | null>(null);
  const [archiving, setArchiving] = useState<AdminClass | null>(null);

  const classes = useQuery({
    ...adminClassesQuery({ tab, q, disabledTeacher, page, limit: limitOf(size) }),
    placeholderData: keepPreviousData,
  });
  const data = classes.data;
  const pastEnd = data !== undefined && data.items.length === 0 && data.total > 0;
  const lastPage =
    pastEnd && !classes.isPlaceholderData
      ? pageRange(page, data.pageSize, data.total).page
      : null;
  const toLastPage = pastEnd && (classes.isPlaceholderData || lastPage !== page);
  useEffect(() => {
    if (lastPage !== null && lastPage !== page) setPage(lastPage);
  }, [lastPage, page, setPage]);

  const restore = useMutation({
    mutationFn: (klass: AdminClass) => updateClass(klass.id, { archived: false }),
    onSuccess: async (_, klass) => {
      toast(t("classes.restored"));
      searchRef.current?.focus();
      await invalidateClass(queryClient, klass.id);
      await invalidateAdminClasses(queryClient);
    },
    onError: async (cause) => {
      toast.error(message(cause, t("classes.restoreFailed")));
      await invalidateAdminClasses(queryClient);
    },
  });

  const actions: AdminClassActions = {
    onEdit: (klass, from) => {
      returnTo.current = from;
      setEditing(klass);
    },
    onChangeTeacher: (klass, from) => {
      returnTo.current = from;
      setReassigning(klass);
    },
    onRotate: (klass, from) => {
      returnTo.current = from;
      setRotating(klass);
    },
    onArchive: (klass, from) => {
      returnTo.current = from;
      setArchiving(klass);
    },
    onRestore: (klass) => {
      if (!restore.isPending) restore.mutate(klass);
    },
    onCopyCode: copyCode,
  };

  const removeFilter = () => {
    setFilter("teacher", null);
    searchRef.current?.focus();
  };
  const clearFilters = () => {
    setParams(
      (previous) => {
        const next = new URLSearchParams(previous);
        for (const name of ["q", "teacher", "page"]) next.delete(name);
        return next;
      },
      { replace: true },
    );
    searchRef.current?.focus();
  };
  const filtered = q !== "" || disabledTeacher;
  const emptyLine = t(
    tab === "archived" ? "classes.admin.emptyArchived" : "classes.admin.emptyActive",
  );
  const empty = filtered ? (
    <span className="flex flex-col items-center gap-3">
      {t("classes.admin.emptyFiltered")}
      <Button variant="outline" size="sm" onClick={clearFilters}>
        {t("classes.admin.clearFilters")}
      </Button>
    </span>
  ) : (
    emptyLine
  );

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <PageHead
        title={t("adminShell.nav.classes")}
        description={t("classes.admin.lead")}
        actions={
          <Button ref={newButton} onClick={() => setCreating(true)}>
            <Plus aria-hidden="true" />
            {t("classes.new")}
          </Button>
        }
      />
      <p role="status" className="sr-only">
        {data === undefined ? "" : t("classes.admin.result", { count: data.total })}
      </p>

      <JoinCodesOffCallout />

      <div className="flex flex-wrap items-center justify-between gap-2.5">
        <div className="max-w-full min-w-0">
          <Segmented
            label={t("classes.statusFilter")}
            value={tab}
            options={[
              {
                value: "active",
                label: t("classes.tabs.active"),
                count: data?.facets.active,
              },
              {
                value: "archived",
                label: t("classes.tabs.archived"),
                count: data?.facets.archived,
              },
            ]}
            onChange={(value) => setFilter("tab", value === "archived" ? value : null)}
            scroll
          />
        </div>
        <SearchInput
          className={cn(TOOLBAR_SEARCH, "flex-[0_1_280px]")}
          value={typed}
          onChange={setTyped}
          placeholder={t("classes.admin.search")}
          inputRef={searchRef}
          maxLength={ADMIN_CLASS_SEARCH_MAX}
        />
      </div>

      {disabledTeacher ? (
        <div role="group" aria-label={t("classes.admin.filterLabel")}>
          <ChipBox
            values={[t("classes.admin.disabledTeachers")]}
            onRemove={removeFilter}
            removeLabel={(value) => t("classes.admin.removeFilter", { label: value })}
            frame={false}
          >
            {null}
          </ChipBox>
        </div>
      ) : null}

      {classes.isError ? (
        <LoadError error={classes.error} onRetry={() => void classes.refetch()}>
          {t("classes.loadFailed")}
        </LoadError>
      ) : null}
      {data === undefined || toLastPage ? (
        (classes.isPending || toLastPage) && <AdminClassesSkeleton />
      ) : (
        <AdminClassesTable
          classes={data.items}
          tab={data.tab}
          page={data.page}
          pageSize={data.pageSize}
          total={data.total}
          joinOff={joinOff}
          now={now}
          locale={locale}
          empty={empty}
          actions={actions}
        />
      )}

      <AdminClassDialog
        open={creating || editing !== null}
        klass={editing}
        onClose={() => {
          setCreating(false);
          setEditing(null);
        }}
        onChangeTeacher={(klass) => {
          setEditing(null);
          setReassigning(klass);
        }}
        returnFocus={creating ? newButton : returnTo}
      />
      <ChangeTeacherDialog
        klass={reassigning}
        onClose={() => setReassigning(null)}
        returnFocus={returnTo}
      />
      <RotateCodeDialog
        klass={rotating}
        onClose={() => setRotating(null)}
        returnFocus={returnTo}
      />
      <ArchiveAdminClassDialog
        klass={archiving}
        onClose={() => setArchiving(null)}
        onArchived={() => {
          returnTo.current = searchRef.current;
        }}
        returnFocus={returnTo}
      />
    </div>
  );
}

function AdminClassesSkeleton() {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      aria-live="polite"
      aria-label={t("common.loading")}
      className="bg-card shadow-card overflow-hidden rounded-xl border"
    >
      <div className="bg-muted h-10" />
      {Array.from({ length: 6 }, (_, index) => (
        <div key={index} className="flex h-13.5 items-center gap-3 border-t px-4">
          <Skeleton className="h-4 flex-[2]" />
          <Skeleton className="size-6.5 flex-none rounded-full" />
          <Skeleton className="h-4 flex-1" />
          <Skeleton className="h-4 w-20 flex-none" />
        </div>
      ))}
    </div>
  );
}
