import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { Search, UserPlus } from "lucide-react";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
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
  listStudents,
  scorePercent,
  type Student,
} from "@/features/students/api";
import { SUPPORTED_LOCALES, type Locale } from "@/lib/i18n";
import { formatRelative } from "@/lib/i18n/datetime";
import { useDebounced } from "@/lib/useDebounced";
import { PageHeader } from "@/components/shared/PageHeader";

const PAGE_SIZE = 50;

/** §8's students table, as the deck's G-07. */
export default function StudentsListPage() {
  const { t, i18n } = useTranslation();
  const [query, setQuery] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  // The default hides suspended accounts, which is right for the everyday
  // screen -- but without a way to ask for them a disable is a one-way door.
  const [showDisabled, setShowDisabled] = useState(false);
  const [creating, setCreating] = useState(false);
  const search = useDebounced(query, 300).trim();
  const locale = currentLocale(i18n.language);

  // The limit is part of the key on purpose: the two token pickers query
  // `["admin-students", { q }]` with limit 20, and sharing a cache entry across
  // two page sizes would truncate whichever screen painted second.
  const students = useInfiniteQuery({
    queryKey: ["admin-students", { q: search, showDisabled, limit: PAGE_SIZE }],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) =>
      listStudents(
        {
          limit: PAGE_SIZE,
          ...(showDisabled ? { status: "disabled" as const } : {}),
          ...(search === "" ? {} : { q: search }),
          ...(pageParam ? { cursor: pageParam } : {}),
        },
        signal,
      ),
    getNextPageParam: (page) => page.nextCursor ?? undefined,
  });

  const items = students.data?.pages.flatMap((page) => page.items) ?? [];
  const facets = students.data?.pages[0]?.facets;

  // The drawer fetches its own subject rather than reading it out of the loaded
  // page. Deriving it from the list tied the panel's lifetime to the search: a
  // teacher who reset a password and then typed in the search box changed the
  // query key, emptied `items` while the new page loaded, unmounted the drawer,
  // and destroyed the one-time password -- which is stored hashed and exists
  // nowhere else.
  const detail = useQuery({
    queryKey: ["admin-student", selectedId],
    queryFn: ({ signal }) => getStudent(selectedId!, signal),
    enabled: selectedId !== null,
    initialData: () => items.find((student) => student.id === selectedId),
  });
  const selected = selectedId === null ? null : (detail.data ?? null);

  return (
    <div className="-m-6 flex h-[calc(100svh-3.5rem)] overflow-hidden">
      <div className="min-w-0 flex-1 space-y-4 overflow-y-auto p-6">
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
          <div className="relative w-72">
            <Search
              className="text-muted-foreground pointer-events-none absolute top-2.5 left-2.5 size-4"
              aria-hidden="true"
            />
            <Input
              className="pl-9"
              value={query}
              placeholder={t("students.searchPlaceholder")}
              aria-label={t("students.searchPlaceholder")}
              onChange={(event) => setQuery(event.target.value)}
            />
          </div>
          <label className="flex items-center gap-2.5 text-sm">
            <Checkbox
              checked={showDisabled}
              onChange={(event) => {
                setShowDisabled(event.target.checked);
                setSelectedId(null);
              }}
            />
            {t("students.showDisabled")}
          </label>
        </div>

        {students.isPending ? (
          <p role="status" aria-live="polite" className="text-muted-foreground text-sm">
            {t("common.loading")}
          </p>
        ) : students.isError ? (
          <div className="space-y-3">
            <p role="alert" className="text-sm">
              {t("students.loadFailed")}
            </p>
            <Button variant="outline" size="sm" onClick={() => void students.refetch()}>
              {t("common.retry")}
            </Button>
          </div>
        ) : items.length === 0 ? (
          <p className="text-muted-foreground text-sm">
            {showDisabled
              ? t("students.noneDisabled")
              : search === ""
                ? t("students.empty")
                : t("students.noMatches")}
          </p>
        ) : (
          <>
            <Card className="gap-0 overflow-hidden py-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-[26%]">{t("students.student")}</TableHead>
                    <TableHead>{t("students.classes")}</TableHead>
                    <TableHead>{t("students.signInWith")}</TableHead>
                    <TableHead className="text-right">
                      {t("students.submitted")}
                    </TableHead>
                    <TableHead className="text-right">
                      {t("students.average")}
                    </TableHead>
                    <TableHead>{t("students.activity")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {items.map((student) => (
                    <Row
                      key={student.id}
                      student={student}
                      locale={locale}
                      expanded={student.id === selectedId}
                      onToggle={() =>
                        setSelectedId(student.id === selectedId ? null : student.id)
                      }
                    />
                  ))}
                </TableBody>
              </Table>
            </Card>

            {students.hasNextPage ? (
              <Button
                variant="outline"
                size="sm"
                disabled={students.isFetchingNextPage}
                onClick={() => void students.fetchNextPage()}
              >
                {students.isFetchingNextPage ? t("common.loading") : t("bank.loadMore")}
              </Button>
            ) : null}
          </>
        )}
      </div>

      {selected === null ? null : (
        // Keyed by student: without it React reuses the same fiber when the
        // teacher clicks the next row, and the drawer keeps its state -- so the
        // panel showed the NEW student's name above the PREVIOUS student's
        // one-time password, under a caption telling the teacher to hand it
        // over.
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

function Row({
  student,
  locale,
  expanded,
  onToggle,
}: {
  student: Student;
  locale: Locale;
  expanded: boolean;
  onToggle: () => void;
}) {
  const { t } = useTranslation();
  const percent = scorePercent(student.stats);

  return (
    <TableRow>
      <TableCell>
        {/* aria-expanded is what tints the row: table.tsx carries
          `has-aria-expanded:bg-muted/50`, which is the deck's selected-row
          highlight without a second piece of state to keep in step. */}
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
        {student.classes.length === 0
          ? "—"
          : student.classes.length === 1
            ? student.classes[0]!.name
            : t("students.classesPlus", {
                name: student.classes[0]!.name,
                more: student.classes.length - 1,
              })}
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
        ) : student.stats.activity.lastAttemptAt ? (
          formatRelative(student.stats.activity.lastAttemptAt, locale)
        ) : (
          "—"
        )}
      </TableCell>
    </TableRow>
  );
}

function currentLocale(language: string): Locale {
  return (SUPPORTED_LOCALES as readonly string[]).includes(language)
    ? (language as Locale)
    : "vi";
}
