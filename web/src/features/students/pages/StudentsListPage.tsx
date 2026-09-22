import { useBulkSelection } from "@/hooks/useBulkSelection";
import { BulkActions } from "@/components/shared/BulkActions";
import { BulkSelectAll, BulkSelectRow } from "@/components/shared/BulkSelection";
import { DeleteItemButton } from "@/components/shared/DeleteItemButton";
import type { ReactNode } from "react";
import { useListFilters } from "@/hooks/useListFilters";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { keepPreviousData, useQueryClient, useQuery } from "@tanstack/react-query";
import { UserPlus } from "lucide-react";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { NewStudentDialog } from "@/features/students/components/NewStudentDialog";
import { StudentDrawer } from "@/features/students/components/StudentDrawer";
import {
  getStudent,
  deleteStudent,
  updateStudent,
  listStudents,
  scorePercent,
  type Student,
} from "@/features/students/api";
import type { Locale } from "@/lib/i18n";
import { useLocale } from "@/lib/i18n/useLocale";
import { formatRelative } from "@/lib/i18n/datetime";
import { useDebounced } from "@/lib/useDebounced";
import { EmptyState, ListSkeleton, QueryStates } from "@/components/shared/ListState";
import { PageHeader } from "@/components/shared/PageHeader";
import { SearchInput } from "@/components/shared/SearchInput";
import { Pager } from "@/components/shared/Pager";
import { usePage } from "@/hooks/usePage";
import type { TFunction } from "i18next";

const PAGE_SIZE = 20;

/** §8's students table, as the deck's G-07. */
export default function StudentsListPage() {
  const { t } = useTranslation();
  const bulk = useBulkSelection<Student>();
  const queryClient = useQueryClient();
  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ["admin-students"] });
  const {
    params: searchParams,
    setParams: setSearchParams,
    setFilter,
  } = useListFilters();
  const query = searchParams.get("q") ?? "";
  const setQuery = (value: string) => setFilter("q", value);
  const selectedId = searchParams.get("studentId");
  const setSelectedId = (id: string | null) =>
    setSearchParams((previous) => {
      const next = new URLSearchParams(previous);
      if (id === null) next.delete("studentId");
      else next.set("studentId", id);
      return next;
    });
  const showDisabled = searchParams.get("status") === "disabled";
  const setShowDisabled = (value: boolean) =>
    setSearchParams(
      (previous) => {
        const next = new URLSearchParams(previous);
        next.delete("studentId");
        next.delete("page");
        if (value) next.set("status", "disabled");
        else next.delete("status");
        return next;
      },
      { replace: true },
    );
  const [creating, setCreating] = useState(false);
  const search = useDebounced(query, 300).trim();
  const locale = useLocale();

  const [page] = usePage(JSON.stringify({ search, showDisabled }));
  const students = useQuery({
    queryKey: ["admin-students", { q: search, showDisabled, limit: PAGE_SIZE, page }],
    queryFn: ({ signal }) =>
      listStudents(
        {
          limit: PAGE_SIZE,
          page,
          ...(showDisabled ? { status: "disabled" as const } : {}),
          ...(search === "" ? {} : { q: search }),
        },
        signal,
      ),
    placeholderData: keepPreviousData,
  });

  const items = students.data?.items ?? [];
  const facets = students.data?.facets;

  // The drawer fetches its own subject rather than reading it out of the loaded page.
  const detail = useQuery({
    queryKey: ["admin-student", selectedId],
    queryFn: ({ signal }) => getStudent(selectedId!, signal),
    enabled: selectedId !== null,
    initialData: () => items.find((student) => student.id === selectedId),
  });
  const selected = selectedId === null ? null : (detail.data ?? null);

  return (
    <div className="space-y-4">
      <PageHeader
        variant="title"
        title={t("nav.students")}
        subtitle={
          facets
            ? t("students.summary", {
                count: facets.total,
                active: facets.activeLast7Days,
              })
            : " "
        }
        actions={
          <Button size="sm" onClick={() => setCreating(true)}>
            <UserPlus aria-hidden="true" />
            {t("students.new")}
          </Button>
        }
      />

      <div className="flex items-center gap-4">
        <SearchInput
          value={query}
          onChange={setQuery}
          placeholder={t("students.searchPlaceholder")}
        />
        <label className="flex items-center gap-2.5 text-sm">
          <Checkbox
            checked={showDisabled}
            onChange={(event) => setShowDisabled(event.target.checked)}
          />
          {t("students.showDisabled")}
        </label>
      </div>

      <BulkActions
        selected={[...bulk.selected.values()]}
        name={(item) => item.fullName}
        actions={[
          {
            label: t("common.disableSelected"),
            description: t("common.disableSelectedBody"),
            run: (item) => updateStudent(item.id, { disabled: true }),
          },
          {
            label: t("common.deletePermanently"),
            description: t("common.deleteInactiveBody"),
            run: (item) => deleteStudent(item.id),
          },
        ]}
        onRemoved={bulk.remove}
        onClear={bulk.clear}
        onSettled={invalidate}
      />
      <QueryStates
        query={students}
        skeleton={<ListSkeleton />}
        failed={t("students.loadFailed")}
      >
        {(data) =>
          items.length === 0 ? (
            <EmptyState
              action={
                showDisabled || search !== "" ? undefined : (
                  <Button size="sm" onClick={() => setCreating(true)}>
                    <UserPlus aria-hidden="true" />
                    {t("students.new")}
                  </Button>
                )
              }
            >
              {t(emptyMessage(showDisabled, search))}
            </EmptyState>
          ) : (
            <>
              <StudentTable
                items={items}
                selectAll={<BulkSelectAll items={items} selection={bulk} />}
                selectRow={(item) => (
                  <BulkSelectRow item={item} name={item.fullName} selection={bulk} />
                )}
                deleteRow={(item) =>
                  item.disabledAt ? (
                    <DeleteItemButton
                      name={item.fullName}
                      onDelete={() => deleteStudent(item.id)}
                      onDeleted={invalidate}
                    />
                  ) : null
                }
                locale={locale}
                selectedId={selectedId}
                onSelect={setSelectedId}
              />

              {data && (
                <Pager page={data.page} pageSize={data.pageSize} total={data.total} />
              )}
            </>
          )
        }
      </QueryStates>

      {selected === null ? null : (
        <StudentDrawer
          key={selected.id}
          student={selected}
          onClose={() => setSelectedId(null)}
        />
      )}

      <NewStudentDialog open={creating} onOpenChange={setCreating} />
    </div>
  );
}

function StudentTable({
  selectAll,
  selectRow,
  deleteRow,
  items,
  locale,
  selectedId,
  onSelect,
}: Readonly<{
  selectAll: ReactNode;
  selectRow: (item: Student) => ReactNode;
  deleteRow: (item: Student) => ReactNode;
  items: Student[];
  locale: Locale;
  selectedId: string | null;
  onSelect: (id: string | null) => void;
}>) {
  const { t } = useTranslation();
  return (
    <Card className="gap-0 overflow-hidden py-0">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-10">{selectAll}</TableHead>
            <TableHead className="w-[26%]">{t("students.student")}</TableHead>
            <TableHead>{t("students.classes")}</TableHead>
            <TableHead>{t("students.signInWith")}</TableHead>
            <TableHead className="text-right">{t("students.submitted")}</TableHead>
            <TableHead className="text-right">{t("students.average")}</TableHead>
            <TableHead>{t("students.activity")}</TableHead>
            <TableHead className="w-10">
              <span className="sr-only">{t("common.actions")}</span>
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map((student) => (
            <Row
              key={student.id}
              student={student}
              selection={selectRow(student)}
              deleteAction={deleteRow(student)}
              locale={locale}
              expanded={student.id === selectedId}
              onToggle={() => onSelect(student.id === selectedId ? null : student.id)}
            />
          ))}
        </TableBody>
      </Table>
    </Card>
  );
}

function emptyMessage(showDisabled: boolean, search: string): string {
  if (showDisabled) return "students.noneDisabled";
  return search === "" ? "students.empty" : "students.noMatches";
}

function Row({
  selection,
  deleteAction,
  student,
  locale,
  expanded,
  onToggle,
}: Readonly<{
  selection: ReactNode;
  deleteAction: ReactNode;
  student: Student;
  locale: Locale;
  expanded: boolean;
  onToggle: () => void;
}>) {
  const { t } = useTranslation();
  const percent = scorePercent(student.stats);

  return (
    <TableRow>
      <TableCell>{selection}</TableCell>
      <TableCell>
        <button
          type="button"
          aria-expanded={expanded}
          className="flex items-center gap-2 text-left"
          onClick={onToggle}
        >
          <Avatar size="sm" name={student.fullName} />
          <span className="min-w-0">
            <span className="block truncate font-medium">{student.fullName}</span>
            <span className="text-muted-foreground block truncate text-xs">
              {student.email}
            </span>
          </span>
        </button>
      </TableCell>
      <TableCell className="text-muted-foreground">
        {classesText(student.classes, t)}
      </TableCell>
      <TableCell>
        <span className="flex flex-wrap gap-1">
          {student.linkedProviders.includes("google") ? (
            <Badge variant="outline">{t("students.google")}</Badge>
          ) : null}
          {student.hasPassword ? (
            <Badge variant="outline">{t("students.password")}</Badge>
          ) : null}
        </span>
      </TableCell>
      <TableCell className="text-right tabular-nums">
        {student.stats.submittedCount}
      </TableCell>
      <TableCell className="text-right tabular-nums">
        {percent === null ? (
          <span className="text-muted-foreground">—</span>
        ) : (
          t("students.percent", { value: percent })
        )}
      </TableCell>
      <TableCell className="text-muted-foreground">
        {student.stats.activity.live ? (
          <span className="text-success-ink">{t("students.takingNow")}</span>
        ) : (
          lastSeenText(student.stats.activity.lastAttemptAt, locale)
        )}
      </TableCell>
      <TableCell>{deleteAction}</TableCell>
    </TableRow>
  );
}

function classesText(
  classes: readonly { readonly name: string }[],
  t: TFunction,
): string {
  const [first] = classes;
  if (first === undefined) return "—";
  if (classes.length === 1) return first.name;
  return t("students.classesPlus", { name: first.name, more: classes.length - 1 });
}

function lastSeenText(
  lastAttemptAt: string | null | undefined,
  locale: Locale,
): string {
  return lastAttemptAt ? formatRelative(lastAttemptAt, locale) : "—";
}
