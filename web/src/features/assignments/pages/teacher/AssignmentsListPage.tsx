import { useMemo, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "react-router";
import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import {
  ArrowUpRight,
  CircleStop,
  Clock,
  Copy,
  Download,
  Pencil,
  Plus,
  Trash2,
} from "lucide-react";
import { BulkActions, BulkBarButton } from "@/components/shared/BulkActions";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { DataTable } from "@/components/shared/data/DataTable";
import { FacetFilter } from "@/components/shared/data/FacetFilter";
import { Pager } from "@/components/shared/data/Pager";
import { LoadError } from "@/components/shared/ListState";
import { SearchInput } from "@/components/shared/SearchInput";
import { Button } from "@/components/ui/button";
import { DropdownMenuItem, DropdownMenuSeparator } from "@/components/ui/dropdown-menu";
import { Segmented } from "@/components/ui/segmented";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/sonner";
import { useCan } from "@/features/auth/permissions";
import {
  deleteAssignment,
  exportResults,
  listAssignments,
  updateAssignment,
  type Assignment,
} from "@/features/assignments/api";
import { CloseEarlyDialog } from "@/features/assignments/components/CloseEarlyDialog";
import { toInput } from "@/features/assignments/input";
import { statusAt } from "@/features/assignments/status";
import { fetchClasses } from "@/features/classes/api";
import { useBulkSelection } from "@/hooks/useBulkSelection";
import { useListFilters } from "@/hooks/useListFilters";
import { usePage, usePageSize } from "@/hooks/usePage";
import { PageHead } from "@/layouts/shell/PageHead";
import { ApiError } from "@/lib/api/errors";
import { useDisplayTimeZone } from "@/lib/i18n/datetime";
import { useDebounced } from "@/lib/useDebounced";
import { AssignmentCard } from "./AssignmentCells";
import { assignmentColumns } from "./assignmentColumns";
import { windowOf, type ListTab } from "./assignmentWindow";
import { DuplicateAssignmentDialog } from "./DuplicateAssignmentDialog";
import { ExtendAssignmentsDialog } from "./ExtendAssignmentsDialog";

const QUERY_KEY = ["admin-assignments"] as const;
const TABS: readonly ListTab[] = ["open", "scheduled", "closed", "draft"];
const EXPORT_LIMIT = 50;

const SEARCH =
  "w-auto min-w-0 flex-[0_1_260px] [&_input]:bg-card [&_input]:border-border [&_input]:h-8.5 [&_input]:pl-8.5 [&_input]:text-sm [&_svg]:top-[9.5px] [&_svg]:size-3.75";

type Acting = {
  kind: "extend" | "duplicate" | "close" | "delete";
  assignment: Assignment;
} | null;

function toTab(value: string | null): ListTab {
  return TABS.find((tab) => tab === value) ?? "open";
}

function failure(cause: unknown, fallback: string) {
  return cause instanceof ApiError ? cause.message : fallback;
}

function ListSkeleton() {
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
        <div key={index} className="flex items-center gap-3 border-t px-4 py-5">
          <Skeleton className="size-4 flex-none rounded-[0.25rem]" />
          <Skeleton className="h-4 flex-1" />
          <Skeleton className="h-4 w-24 flex-none" />
        </div>
      ))}
    </div>
  );
}

function AssignmentMenuItems({
  assignment,
  now,
  write,
  onAct,
}: Readonly<{
  assignment: Assignment;
  now: Date;
  write: boolean;
  onAct: (kind: NonNullable<Acting>["kind"], assignment: Assignment) => void;
}>): ReactNode {
  const { t } = useTranslation();
  const status = statusAt(assignment, now);
  const href = `/teacher/assignments/${assignment.id}`;
  return (
    <>
      <DropdownMenuItem asChild>
        <Link to={href}>
          <ArrowUpRight aria-hidden="true" />
          {t("assignments.list.open")}
        </Link>
      </DropdownMenuItem>
      {write ? (
        <>
          <DropdownMenuItem asChild>
            <Link to={`${href}?tab=settings`}>
              <Pencil aria-hidden="true" />
              {t("assignments.list.editSettings")}
            </Link>
          </DropdownMenuItem>
          <DropdownMenuItem
            disabled={status === "draft" || status === "closed"}
            onSelect={() => onAct("extend", assignment)}
          >
            <Clock aria-hidden="true" />
            {t("assignments.list.extend")}
          </DropdownMenuItem>
          <DropdownMenuItem onSelect={() => onAct("duplicate", assignment)}>
            <Copy aria-hidden="true" />
            {t("assignments.list.duplicate")}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          {status === "draft" ? (
            <DropdownMenuItem
              variant="destructive"
              onSelect={() => onAct("delete", assignment)}
            >
              <Trash2 aria-hidden="true" />
              {t("assignments.list.delete")}
            </DropdownMenuItem>
          ) : (
            <DropdownMenuItem
              variant="destructive"
              disabled={status !== "open"}
              onSelect={() => onAct("close", assignment)}
            >
              <CircleStop aria-hidden="true" />
              {t("assignments.list.closeEarly")}
            </DropdownMenuItem>
          )}
        </>
      ) : null}
    </>
  );
}

/**
 * AssignmentsBulkBar is the bulk bar of the assignments list: Extend deadline
 * (`onExtend` opens its dialog) and Close now with `write`, which close the
 * live ones one at a time and report the others; Export results with
 * `grading`, one CSV for up to 50, whose refusal is shown.
 */
function AssignmentsBulkBar({
  selected,
  write,
  grading,
  onExtend,
  onRemoved,
  onClear,
  onSettled,
}: Readonly<{
  selected: readonly Assignment[];
  write: boolean;
  grading: boolean;
  onExtend: () => void;
  onRemoved: (ids: string[]) => void;
  onClear: () => void;
  onSettled: () => Promise<unknown>;
}>) {
  const { t } = useTranslation();
  const download = useMutation({
    mutationFn: (chosen: readonly Assignment[]) => exportResults(chosen),
    onError: (cause) => toast.error(failure(cause, t("assignments.list.exportFailed"))),
  });
  const closeNow = (assignment: Assignment) =>
    statusAt(assignment, new Date()) === "open"
      ? updateAssignment(assignment.id, {
          ...toInput(assignment),
          draft: false,
          closeNow: true,
        })
      : Promise.reject(
          new ApiError({
            status: 409,
            code: "UNKNOWN",
            message: t("assignments.list.notLive"),
          }),
        );

  return (
    <BulkActions
      selected={selected}
      name={(assignment) => assignment.testTitle}
      hideOnPhone
      actions={
        write
          ? [
              {
                label: t("assignments.list.closeNow"),
                description: t("assignments.list.closeNowBody"),
                icon: CircleStop,
                run: closeNow,
              },
            ]
          : []
      }
      onRemoved={onRemoved}
      onClear={onClear}
      onSettled={onSettled}
    >
      {write ? (
        <BulkBarButton icon={Clock} onClick={onExtend}>
          {t("assignments.list.extend")}
        </BulkBarButton>
      ) : null}
      {grading ? (
        <BulkBarButton
          icon={Download}
          disabled={download.isPending}
          onClick={() => {
            if (selected.length > EXPORT_LIMIT)
              toast.error(t("assignments.list.exportTooMany"));
            else download.mutate(selected);
          }}
        >
          {t("assignments.list.export")}
        </BulkBarButton>
      ) : null}
    </BulkActions>
  );
}

/**
 * AssignmentsListPage is the teacher's assignments as the deck's Assignments
 * screen draws them: the Live, Scheduled, Closed and Drafts tabs with their
 * counts, a search and a class facet, and the assignments in a table whose
 * columns drop as the content narrows and which becomes cards below 768px.
 * The tab (`status`), the classes (`classId`), the search (`q`), the page and
 * its size live in the URL. A row opens the assignment. Its menu, and the
 * bulk bar's Extend deadline, Export results and Close now, show only with
 * the permissions they need; the bulk actions run one assignment at a time
 * and report each failure.
 */
export default function AssignmentsListPage() {
  useDisplayTimeZone();
  const { t } = useTranslation();
  const navigate = useNavigate();
  const client = useQueryClient();
  const write = useCan("teaching.assignments.write");
  const grading = useCan("teaching.grading");
  const { params, setFilter } = useListFilters();
  const tab = toTab(params.get("status"));
  const classIds = params.getAll("classId");
  const query = params.get("q") ?? "";
  const search = useDebounced(query.trim(), 300);
  const [size] = usePageSize();
  const [page] = usePage(JSON.stringify({ tab, classIds, search, size }));
  const bulk = useBulkSelection<Assignment>();
  const [acting, setActing] = useState<Acting>(null);
  const [extending, setExtending] = useState<readonly Assignment[]>([]);
  const now = new Date();

  const assignments = useQuery({
    queryKey: [...QUERY_KEY, { tab, classIds, search, page, size }],
    queryFn: ({ signal }) =>
      listAssignments(
        {
          status: tab,
          limit: size,
          page,
          ...(classIds.length > 0 ? { classId: classIds } : {}),
          ...(search === "" ? {} : { q: search }),
        },
        signal,
      ),
    placeholderData: keepPreviousData,
  });
  const classes = useQuery({
    queryKey: ["admin-classes", "picker", { limit: 100 }],
    queryFn: ({ signal }) => fetchClasses({ limit: 100 }, signal),
    staleTime: 60_000,
  });
  const columns = useMemo(() => assignmentColumns(t, tab, new Date()), [t, tab]);

  const invalidate = () => client.invalidateQueries({ queryKey: QUERY_KEY });
  const close = useMutation({
    mutationFn: (assignment: Assignment) =>
      updateAssignment(assignment.id, {
        ...toInput(assignment),
        draft: false,
        closeNow: true,
      }),
    onSuccess: async (closed) => {
      setActing(null);
      toast(
        t("assignments.list.closed", {
          submitted: closed.submittedCount ?? 0,
          target: closed.targetCount ?? 0,
        }),
      );
      await invalidate();
    },
  });
  const remove = useMutation({
    mutationFn: (assignment: Assignment) => deleteAssignment(assignment.id),
    onSuccess: async () => {
      setActing(null);
      toast(t("assignments.list.deleted"));
      await invalidate();
    },
  });

  const data = assignments.data;
  const selected = [...bulk.selected.values()];
  const selecting = write || grading;
  const selectionProps = selecting
    ? { selection: bulk, rowName: (assignment: Assignment) => assignment.testTitle }
    : {};
  const newAssignment = write ? (
    <Button asChild>
      <Link to="/teacher/assignments/new">
        <Plus aria-hidden="true" />
        {t("assignments.new")}
      </Link>
    </Button>
  ) : null;
  const single = extending.length === 1 ? extending[0] : undefined;
  const extendingWhen =
    single === undefined ? undefined : windowOf(single, "open", now, t);
  const act = (kind: NonNullable<Acting>["kind"], assignment: Assignment) => {
    if (kind === "extend") setExtending([assignment]);
    else setActing({ kind, assignment });
  };
  return (
    <div className="flex min-w-0 flex-col gap-4">
      <PageHead
        title={t("teacherShell.nav.assignments")}
        description={t("assignments.list.description")}
        actions={newAssignment}
      />

      <div className="flex flex-wrap items-center justify-between gap-2.5">
        <Segmented
          label={t("assignments.list.tabs")}
          value={tab}
          scroll
          onChange={(value) => setFilter("status", value === "open" ? null : value)}
          options={TABS.map((value) => ({
            value,
            label: t(`assignments.list.tab_${value}`),
            count: data?.facets[value],
          }))}
        />
        <div className="flex flex-[1_1_260px] justify-end gap-2">
          <SearchInput
            className={SEARCH}
            value={query}
            onChange={(value) => setFilter("q", value)}
            placeholder={t("assignments.list.search")}
          />
          <FacetFilter
            label={t("assignments.list.classFacet")}
            title={t("assignments.list.classFacetTitle")}
            options={(classes.data?.items ?? []).map((klass) => ({
              value: klass.id,
              label: klass.name,
            }))}
            selected={classIds}
            onChange={(next) => setFilter("classId", next)}
          />
        </div>
      </div>

      {selecting ? (
        <AssignmentsBulkBar
          selected={selected}
          write={write}
          grading={grading}
          onExtend={() => setExtending(selected)}
          onRemoved={bulk.remove}
          onClear={bulk.clear}
          onSettled={invalidate}
        />
      ) : null}

      {assignments.isError ? (
        <LoadError error={assignments.error} onRetry={() => void assignments.refetch()}>
          {t("assignments.loadFailed")}
        </LoadError>
      ) : null}
      {data === undefined ? (
        assignments.isPending && <ListSkeleton />
      ) : (
        <DataTable
          label={t("assignments.list.table")}
          columns={columns}
          rows={data.items}
          rowSize={{ minHeight: 60 }}
          rowHref={(assignment) => `/teacher/assignments/${assignment.id}`}
          {...selectionProps}
          menu={(assignment) => (
            <AssignmentMenuItems
              assignment={assignment}
              now={now}
              write={write}
              onAct={act}
            />
          )}
          card={(assignment) => (
            <AssignmentCard assignment={assignment} tab={tab} now={now} />
          )}
          empty={
            <span className="flex flex-col items-center gap-3">
              {t("assignments.list.empty")}
              {newAssignment}
            </span>
          }
          footer={
            <Pager
              page={data.page}
              pageSize={data.pageSize}
              total={data.total}
              noun={(count) => t("assignments.list.noun", { count })}
            />
          }
        />
      )}

      <ExtendAssignmentsDialog
        items={extending}
        when={extendingWhen}
        open={extending.length > 0}
        onOpenChange={(open) => {
          if (!open) setExtending([]);
        }}
        onExtended={() => void invalidate()}
      />
      <DuplicateAssignmentDialog
        assignment={acting?.kind === "duplicate" ? acting.assignment : null}
        open={acting?.kind === "duplicate"}
        onOpenChange={(open) => {
          if (!open) setActing(null);
        }}
        onDuplicated={(draft) => {
          void invalidate();
          toast(t("assignments.list.duplicated"), {
            action: {
              label: t("assignments.list.open"),
              onClick: () => void navigate(`/teacher/assignments/${draft.id}`),
            },
          });
        }}
      />
      {acting?.kind === "close" ? (
        <CloseEarlyDialog
          assignment={acting.assignment}
          open
          pending={close.isPending}
          failed={close.isError}
          onOpenChange={(open) => {
            if (!open) {
              setActing(null);
              close.reset();
            }
          }}
          onConfirm={() => close.mutate(acting.assignment)}
        />
      ) : null}
      <ConfirmDialog
        open={acting?.kind === "delete"}
        onOpenChange={(open) => {
          if (!open) {
            setActing(null);
            remove.reset();
          }
        }}
        icon={Trash2}
        title={t("assignments.list.deleteTitle")}
        description={t("assignments.list.deleteBody", {
          title: acting?.assignment.testTitle ?? "",
        })}
        confirmLabel={t("assignments.list.delete")}
        destructive
        pending={remove.isPending}
        error={
          remove.isError
            ? failure(remove.error, t("assignments.list.deleteFailed"))
            : null
        }
        onConfirm={() => {
          if (acting?.kind === "delete") remove.mutate(acting.assignment);
        }}
      />
    </div>
  );
}
