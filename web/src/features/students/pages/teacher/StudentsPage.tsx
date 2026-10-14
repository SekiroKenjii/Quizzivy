import { useMemo, useRef, useState, type RefObject } from "react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { keepPreviousData, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowRightLeft,
  Ban,
  ChartColumn,
  KeyRound,
  Trash2,
  UserMinus,
  UserPlus,
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
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/sonner";
import { useCan } from "@/features/auth/permissions";
import { fetchClasses } from "@/features/classes/api";
import {
  deleteStudent,
  getStudent,
  listStudents,
  updateStudent,
  type ListStudentsParams,
  type Student,
} from "@/features/students/api";
import { MoveDialog } from "@/features/students/components/MoveDialog";
import { NewStudentDialog } from "@/features/students/components/NewStudentDialog";
import {
  BULK_RESET_LIMIT,
  ResetPasswordFlow,
} from "@/features/students/components/ResetPasswordFlow";
import { ResetSheet } from "@/features/students/components/ResetSheet";
import { StudentDrawer } from "@/features/students/components/StudentDrawer";
import type { OneTimeCredential } from "@/features/students/credentials";
import { classesLeftBy, removeFromClasses } from "@/features/students/membership";
import { useBulkSelection } from "@/hooks/useBulkSelection";
import { useListFilters } from "@/hooks/useListFilters";
import { usePage, usePageSize } from "@/hooks/usePage";
import { PageHead } from "@/layouts/shell/PageHead";
import { failureMessage } from "@/lib/api/errors";
import { useDisplayTimeZone } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { useDebounced } from "@/lib/useDebounced";
import { StudentCard } from "./StudentCells";
import { studentColumns } from "./studentColumns";
import { statusOf } from "./studentText";

const QUERY_KEY = ["teacher-students"] as const;

const ACCOUNTS = ["active", "notSignedIn", "reset", "suspended"] as const;

const SEARCH =
  "w-auto min-w-0 flex-[0_1_300px] [&_input]:bg-card [&_input]:border-border [&_input]:h-8.5 [&_input]:pl-8.5 [&_input]:text-sm [&_svg]:top-[9.5px] [&_svg]:size-3.75";

type Origin = RefObject<HTMLElement | null> | undefined;

const SELECT_PAGE = '[data-slot="data-table-select-page"] input[type="checkbox"]';

/**
 * survivorOf is where focus goes when a dialog opened from the bulk bar
 * closes: the bar's own button while the selection keeps the bar, else the
 * table's select-all box, else the page's main.
 */
function survivorOf(bar: RefObject<HTMLElement | null>): RefObject<HTMLElement | null> {
  return {
    get current() {
      const button = bar.current?.querySelector<HTMLElement>("button");
      if (button?.isConnected) return button;
      return (
        document.querySelector<HTMLElement>(SELECT_PAGE) ??
        document.querySelector<HTMLElement>("main")
      );
    },
  };
}

type Acting =
  | { kind: "move"; students: readonly Student[] }
  | { kind: "reset"; students: readonly Student[] }
  | { kind: "remove"; student: Student }
  | null;

/**
 * accountQuery turns the Status facet into the list's query: the states
 * chosen, by the server's names, and the suspended accounts that Suspended
 * adds: alone they are the whole list, beside the others they join it.
 */
function accountQuery(
  chosen: readonly string[],
): Pick<ListStudentsParams, "accountStatus" | "status"> {
  const states = ACCOUNTS.filter((value) => chosen.includes(value));
  if (states.length === 0) return {};
  const query: Pick<ListStudentsParams, "accountStatus" | "status"> = {
    accountStatus: states.map(statusOf),
  };
  if (states.includes("suspended"))
    query.status = states.length === 1 ? "disabled" : "all";
  return query;
}

function describe(
  t: TFunction,
  students: number | undefined,
  classes: number | undefined,
): string {
  return students === undefined
    ? t("students.descriptionLoading")
    : t("students.description", { count: students, classes: classes ?? 0 });
}

function AddStudentButton({
  ref,
  onClick,
}: Readonly<{ ref?: RefObject<HTMLButtonElement | null>; onClick: () => void }>) {
  const { t } = useTranslation();
  return (
    <Button ref={ref} onClick={onClick}>
      <UserPlus aria-hidden="true" />
      {t("students.new")}
    </Button>
  );
}

function EmptyLine({
  filtered,
  canCreate,
  onAdd,
}: Readonly<{ filtered: boolean; canCreate: boolean; onAdd: () => void }>) {
  const { t } = useTranslation();
  if (filtered) return <>{t("students.noMatches")}</>;
  return (
    <span className="flex flex-col items-center gap-3">
      {t("students.empty")}
      {canCreate ? <AddStudentButton onClick={onAdd} /> : null}
    </span>
  );
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
        <div key={index} className="flex items-center gap-3 border-t px-4 py-3">
          <Skeleton className="size-8 flex-none rounded-full" />
          <Skeleton className="h-4 flex-1" />
          <Skeleton className="h-4 w-20 flex-none" />
        </div>
      ))}
    </div>
  );
}

function StudentMenuItems({
  student,
  canMove,
  canReset,
  onAct,
}: Readonly<{
  student: Student;
  canMove: boolean;
  canReset: boolean;
  onAct: (kind: "view" | "move" | "reset" | "remove") => void;
}>) {
  const { t } = useTranslation();
  return (
    <>
      <DropdownMenuItem onSelect={() => onAct("view")}>
        <ChartColumn aria-hidden="true" />
        {t("students.menu.results")}
      </DropdownMenuItem>
      {canMove ? (
        <DropdownMenuItem onSelect={() => onAct("move")}>
          <ArrowRightLeft aria-hidden="true" />
          {t("students.menu.move")}
        </DropdownMenuItem>
      ) : null}
      {canReset ? (
        <DropdownMenuItem onSelect={() => onAct("reset")}>
          <KeyRound aria-hidden="true" />
          {t("students.menu.reset")}
        </DropdownMenuItem>
      ) : null}
      {canMove ? (
        <>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            variant="destructive"
            disabled={student.classes.length === 0}
            onSelect={() => onAct("remove")}
          >
            <UserMinus aria-hidden="true" />
            {t("students.removeFromClass")}
          </DropdownMenuItem>
        </>
      ) : null}
    </>
  );
}

/**
 * StudentsBulkBar is the bulk bar of the students list: Move to class and
 * Remove with `teaching.classes.write`, Reset passwords with
 * `people.students.reset_password`, for up to 40 students at once, and, with
 * `people.users.manage`, suspending the accounts and deleting suspended ones
 * as the old console did (DG-166). It is not drawn below 768px, where the
 * list has no checkboxes. `moveRef` and `resetRef` hold the Move and Reset
 * buttons, where focus returns after their dialogs.
 */
function StudentsBulkBar({
  selected,
  canMove,
  canReset,
  canManage,
  moveRef,
  resetRef,
  onMove,
  onReset,
  onRemoved,
  onClear,
  onSettled,
}: Readonly<{
  selected: readonly Student[];
  canMove: boolean;
  canReset: boolean;
  canManage: boolean;
  moveRef: RefObject<HTMLSpanElement | null>;
  resetRef: RefObject<HTMLSpanElement | null>;
  onMove: () => void;
  onReset: () => void;
  onRemoved: (ids: string[]) => void;
  onClear: () => void;
  onSettled: () => Promise<unknown>;
}>) {
  const { t } = useTranslation();
  const left = classesLeftBy(selected);
  return (
    <BulkActions
      selected={selected}
      name={(student) => student.fullName}
      hideOnPhone
      actions={[
        ...(canMove
          ? [
              {
                label: t("students.bulk.remove"),
                description: t("students.bulk.removeBody", {
                  count: left.count,
                  classes: left.names,
                }),
                icon: UserMinus,
                run: removeFromClasses,
              },
            ]
          : []),
        ...(canManage
          ? [
              {
                label: t("common.disableSelected"),
                description: t("common.disableSelectedBody"),
                icon: Ban,
                run: (student: Student) =>
                  updateStudent(student.id, { disabled: true }),
              },
              {
                label: t("common.deletePermanently"),
                description: t("common.deleteInactiveBody"),
                icon: Trash2,
                run: (student: Student) => deleteStudent(student.id),
              },
            ]
          : []),
      ]}
      onRemoved={onRemoved}
      onClear={onClear}
      onSettled={onSettled}
    >
      {canMove ? (
        <span ref={moveRef} className="contents">
          <BulkBarButton icon={ArrowRightLeft} onClick={onMove}>
            {t("students.bulk.move")}
          </BulkBarButton>
        </span>
      ) : null}
      {canReset ? (
        <span ref={resetRef} className="contents">
          <BulkBarButton
            icon={KeyRound}
            onClick={() => {
              if (selected.length > BULK_RESET_LIMIT)
                toast.error(
                  t("students.bulk.resetTooMany", { count: BULK_RESET_LIMIT }),
                );
              else onReset();
            }}
          >
            {t("students.bulk.reset")}
          </BulkBarButton>
        </span>
      ) : null}
    </BulkActions>
  );
}

/**
 * RemoveStudentDialog asks before it takes `student` out of every class in
 * their row, which it names, and keeps a refusal in the dialog.
 */
function RemoveStudentDialog({
  student,
  returnFocus,
  onClose,
  onRemoved,
}: Readonly<{
  student: Student | null;
  returnFocus: Origin;
  onClose: () => void;
  onRemoved: () => Promise<unknown>;
}>) {
  const { t } = useTranslation();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function remove(chosen: Student) {
    setPending(true);
    setError(null);
    try {
      await removeFromClasses(chosen);
      onClose();
      toast(t("students.removedAll", { name: chosen.fullName }));
      await onRemoved();
    } catch (cause) {
      setError(failureMessage(cause, t("students.removeFailed")));
    } finally {
      setPending(false);
    }
  }

  return (
    <ConfirmDialog
      open={student !== null}
      {...(returnFocus === undefined ? {} : { returnFocus })}
      onOpenChange={(open) => {
        if (open || pending) return;
        setError(null);
        onClose();
      }}
      icon={UserMinus}
      title={t("students.removeAllTitle", { name: student?.fullName ?? "" })}
      description={t("students.removeAllBody", {
        name: student?.fullName ?? "",
        classes: student === null ? "" : classesLeftBy([student]).names,
      })}
      confirmLabel={t("students.removeAll")}
      destructive
      pending={pending}
      error={error}
      onConfirm={() => {
        if (student !== null) void remove(student);
      }}
    />
  );
}

/**
 * StudentsPage is the teacher's students as the deck's Students screen draws
 * them: "{n} students across {m} classes.", Add student, a search and the
 * Class and Status facets, and the students in a table whose columns drop as
 * the content narrows and which becomes cards below 768px, without checkboxes
 * or menus. The search (`q`), the classes (`classId`), the statuses
 * (`account`), the page and its size live in the URL, and so does the student
 * whose record is open (`studentId`). Each control shows only with its
 * permission: Add student with `people.students.create`, Reset password with
 * `people.students.reset_password`, Move to class and Remove from class with
 * `teaching.classes.write`. When a dialog closes, focus goes back to what
 * opened it: the row's menu, Add student, or the bulk bar's button while the
 * bar survives, else the table's select-all box, else the page.
 */
export default function StudentsPage() {
  useDisplayTimeZone();
  const { t } = useTranslation();
  const locale = useLocale();
  const client = useQueryClient();
  const canCreate = useCan("people.students.create");
  const canReset = useCan("people.students.reset_password");
  const canMove = useCan("teaching.classes.write");
  const canManage = useCan("people.users.manage");
  const { params, setParams, setFilter } = useListFilters();
  const query = params.get("q") ?? "";
  const classIds = params.getAll("classId");
  const accounts = params.getAll("account");
  const selectedId = params.get("studentId");
  const search = useDebounced(query.trim(), 300);
  const [size] = usePageSize();
  const scope = JSON.stringify({ classIds, accounts, search });
  const [page] = usePage(JSON.stringify({ classIds, accounts, search, size }));
  const bulk = useBulkSelection<Student>();
  const [selectionScope, setSelectionScope] = useState(scope);
  if (selectionScope !== scope) {
    setSelectionScope(scope);
    bulk.clear();
  }
  const [creating, setCreating] = useState(false);
  const [created, setCreated] = useState<OneTimeCredential | null>(null);
  const [acting, setActing] = useState<Acting>(null);
  const [origin, setOrigin] = useState<Origin>(undefined);
  const addButton = useRef<HTMLButtonElement>(null);
  const moveTrigger = useRef<HTMLSpanElement>(null);
  const resetTrigger = useRef<HTMLSpanElement>(null);

  const students = useQuery({
    queryKey: [...QUERY_KEY, { classIds, accounts, search, page, size }],
    queryFn: ({ signal }) =>
      listStudents(
        {
          limit: size,
          page,
          ...accountQuery(accounts),
          ...(classIds.length > 0 ? { classId: classIds } : {}),
          ...(search === "" ? {} : { q: search }),
        },
        signal,
      ),
    placeholderData: keepPreviousData,
  });
  const classes = useQuery({
    queryKey: ["teacher-classes", "picker", { limit: 100 }],
    queryFn: ({ signal }) => fetchClasses({ limit: 100 }, signal),
    staleTime: 60_000,
  });
  const ownClasses = classes.data?.items ?? [];
  const items = students.data?.items;
  const detail = useQuery({
    queryKey: ["teacher-student", selectedId],
    queryFn: ({ signal }) => getStudent(selectedId ?? "", signal),
    enabled: selectedId !== null,
    initialData: () => items?.find((student) => student.id === selectedId),
  });
  const selected = selectedId === null ? null : (detail.data ?? null);
  const columns = useMemo(() => studentColumns(t, locale), [t, locale]);

  const invalidate = () => client.invalidateQueries({ queryKey: QUERY_KEY });
  const open = (id: string | null) =>
    setParams((previous) => {
      const next = new URLSearchParams(previous);
      if (id === null) next.delete("studentId");
      else next.set("studentId", id);
      return next;
    });
  const act = (
    kind: "view" | "move" | "reset" | "remove",
    student: Student,
    from: Origin,
  ) => {
    setOrigin(from);
    if (kind === "view") open(student.id);
    else if (kind === "remove") setActing({ kind, student });
    else setActing({ kind, students: [student] });
  };
  const returnFocus = origin === undefined ? {} : { returnFocus: origin };

  const refreshAll = () =>
    Promise.all([
      invalidate(),
      client.invalidateQueries({ queryKey: ["teacher-classes"] }),
    ]);
  const data = students.data;
  const selecting = canMove || canReset || canManage;
  const selectedRows = [...bulk.selected.values()];
  const filtered = search !== "" || classIds.length > 0 || accounts.length > 0;
  const empty = (
    <EmptyLine
      filtered={filtered}
      canCreate={canCreate}
      onAdd={() => setCreating(true)}
    />
  );
  const description = describe(t, data?.facets.total, classes.data?.total);
  const selectionProps = selecting
    ? { selection: bulk, rowName: (student: Student) => student.fullName }
    : {};

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <PageHead
        title={t("teacherShell.nav.students")}
        description={description}
        actions={
          canCreate ? (
            <AddStudentButton ref={addButton} onClick={() => setCreating(true)} />
          ) : null
        }
      />

      <div className="flex flex-wrap items-center gap-2">
        <SearchInput
          className={SEARCH}
          value={query}
          onChange={(value) => setFilter("q", value)}
          placeholder={t("students.searchPlaceholder")}
        />
        <FacetFilter
          label={t("students.classFacet")}
          title={t("students.classFacetTitle")}
          options={ownClasses.map((klass) => ({ value: klass.id, label: klass.name }))}
          selected={classIds}
          onChange={(next) => setFilter("classId", next)}
        />
        <FacetFilter
          label={t("students.statusFacet")}
          title={t("students.statusFacetTitle")}
          options={ACCOUNTS.map((value) => ({
            value,
            label: t(`students.account.${value}`),
          }))}
          selected={accounts}
          onChange={(next) => setFilter("account", next)}
        />
      </div>

      {selecting ? (
        <StudentsBulkBar
          selected={selectedRows}
          canMove={canMove}
          canReset={canReset}
          canManage={canManage}
          moveRef={moveTrigger}
          resetRef={resetTrigger}
          onMove={() => {
            setOrigin(survivorOf(moveTrigger));
            setActing({ kind: "move", students: selectedRows });
          }}
          onReset={() => {
            setOrigin(survivorOf(resetTrigger));
            setActing({ kind: "reset", students: selectedRows });
          }}
          onRemoved={bulk.remove}
          onClear={bulk.clear}
          onSettled={refreshAll}
        />
      ) : null}

      {students.isError ? (
        <LoadError error={students.error} onRetry={() => void students.refetch()}>
          {t("students.loadFailed")}
        </LoadError>
      ) : null}
      {data === undefined ? (
        students.isPending && <ListSkeleton />
      ) : (
        <DataTable
          label={t("students.table")}
          columns={columns}
          rows={data.items}
          rowSize={{ height: 56 }}
          onOpen={(student) => act("view", student, undefined)}
          {...selectionProps}
          menu={(student, { triggerRef }) => (
            <StudentMenuItems
              student={student}
              canMove={canMove}
              canReset={canReset}
              onAct={(kind) => act(kind, student, triggerRef)}
            />
          )}
          menuLabel={(student) => t("students.more", { name: student.fullName })}
          card={(student) => <StudentCard student={student} locale={locale} />}
          cardLayout="joined"
          empty={empty}
          footer={
            <Pager
              page={data.page}
              pageSize={data.pageSize}
              total={data.total}
              noun={(count) => t("students.noun", { count })}
            />
          }
        />
      )}

      {selected === null ? null : (
        <StudentDrawer
          key={selected.id}
          student={selected}
          onClose={() => open(null)}
        />
      )}

      <NewStudentDialog
        open={creating}
        classes={ownClasses}
        onOpenChange={setCreating}
        onCreated={setCreated}
      />
      <ResetSheet
        open={created !== null}
        title={t("students.createdTitle")}
        credentials={created === null ? [] : [created]}
        returnFocus={addButton}
        onDone={() => setCreated(null)}
      />
      <MoveDialog
        students={acting?.kind === "move" ? acting.students : []}
        classes={ownClasses}
        {...returnFocus}
        onOpenChange={(isOpen) => {
          if (!isOpen) setActing(null);
        }}
        onMoved={bulk.clear}
      />
      <ResetPasswordFlow
        students={acting?.kind === "reset" ? acting.students : []}
        {...returnFocus}
        onClose={() => setActing(null)}
      />
      <RemoveStudentDialog
        student={acting?.kind === "remove" ? acting.student : null}
        returnFocus={origin}
        onClose={() => setActing(null)}
        onRemoved={refreshAll}
      />
    </div>
  );
}
