import { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { Link } from "react-router";
import {
  ArchiveRestore,
  ArrowUpRight,
  Ellipsis,
  Inbox,
  Plus,
  Search,
  Send,
  UserPlus,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ArchiveClassDialog } from "@/features/classes/components/ArchiveClassDialog";
import { NewClassDialog } from "@/features/classes/components/NewClassDialog";
import {
  fetchClasses,
  isJoinOpen,
  updateClass,
  type Class,
  type ClassStatus,
} from "@/features/classes/api";
import { invalidateClass } from "@/features/classes/invalidate";
import { PageHeader } from "@/components/shared/PageHeader";
import { Pager } from "@/components/shared/Pager";
import { usePage } from "@/hooks/usePage";
import { SUPPORTED_LOCALES, type Locale } from "@/lib/i18n";
import { formatDayMonth } from "@/lib/i18n/datetime";
import { useDebounced } from "@/lib/useDebounced";
import { cn } from "@/lib/utils";

const PAGE_SIZE = 20;
type Tab = Extract<ClassStatus, "all" | "joinable" | "archived">;
const TABS: Tab[] = ["all", "joinable", "archived"];

/** §8's classes list, as the deck's G-08: create and archive, never delete. */
export default function ClassesListPage() {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const [query, setQuery] = useState("");
  const [tab, setTab] = useState<Tab>("all");
  const [creating, setCreating] = useState(false);
  const [archiving, setArchiving] = useState<Class | null>(null);
  const search = useDebounced(query, 300).trim();
  const locale = currentLocale(i18n.language);

  const [page] = usePage(JSON.stringify({ search, tab }));
  const classes = useQuery({
    queryKey: ["admin-classes", { q: search, status: tab, limit: PAGE_SIZE, page }],
    queryFn: ({ signal }) =>
      fetchClasses(
        {
          limit: PAGE_SIZE,
          page,
          status: tab,
          ...(search === "" ? {} : { q: search }),
        },
        signal,
      ),
    placeholderData: keepPreviousData,
  });
  const restore = useMutation({
    mutationFn: (id: string) => updateClass(id, { archived: false }),
    onSuccess: (_, id) => invalidateClass(queryClient, id),
  });

  const items = classes.data?.items ?? [];
  const facets = classes.data?.facets;
  const nothingYet = facets !== undefined && facets.all === 0 && search === "";

  return (
    <div className="space-y-4">
      <PageHeader
        variant="title"
        title={t("nav.classes")}
        subtitle={
          facets
            ? t("classes.summary", { count: facets.all, students: facets.students })
            : " "
        }
        actions={
          nothingYet ? null : (
            <Button size="sm" onClick={() => setCreating(true)}>
              <Plus aria-hidden="true" />
              {t("classes.new")}
            </Button>
          )
        }
      />

      {nothingYet ? (
        <div className="rounded-lg border border-dashed p-8 text-center">
          <p className="text-sm">{t("classes.empty")}</p>
          <Button size="sm" className="mt-3" onClick={() => setCreating(true)}>
            <Plus aria-hidden="true" />
            {t("classes.createFirst")}
          </Button>
        </div>
      ) : (
        <>
          <div className="flex items-center gap-2">
            <Tabs value={tab} onValueChange={(next) => setTab(next as Tab)}>
              <TabsList aria-label={t("classes.statusFilter")}>
                {TABS.map((value) => (
                  <TabsTrigger key={value} value={value}>
                    {t(`classes.tabs.${value}`)}
                    {facets ? (
                      <span className="text-muted-foreground ml-1 tabular-nums">
                        {facets[value]}
                      </span>
                    ) : null}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>
            <div className="relative ml-auto w-72">
              <Search
                className="text-muted-foreground pointer-events-none absolute top-2.5 left-2.5 size-4"
                aria-hidden="true"
              />
              <Input
                className="pl-9"
                value={query}
                placeholder={t("classes.searchPlaceholder")}
                aria-label={t("classes.searchPlaceholder")}
                onChange={(event) => setQuery(event.target.value)}
              />
            </div>
          </div>

          {classes.isPending ? (
            <p
              role="status"
              aria-live="polite"
              className="text-muted-foreground text-sm"
            >
              {t("common.loading")}
            </p>
          ) : classes.isError ? (
            <div className="space-y-3">
              <p role="alert" className="text-sm">
                {t("classes.loadFailed")}
              </p>
              <Button
                variant="outline"
                size="sm"
                onClick={() => void classes.refetch()}
              >
                {t("common.retry")}
              </Button>
            </div>
          ) : items.length === 0 ? (
            <p className="text-muted-foreground text-sm">{t("classes.noMatches")}</p>
          ) : (
            <>
              <Card className="gap-0 overflow-hidden py-0">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-[36%]">
                        {t("classes.columns.class")}
                      </TableHead>
                      <TableHead className="text-right">
                        {t("classes.columns.students")}
                      </TableHead>
                      <TableHead className="text-right">
                        {t("classes.columns.openAssignments")}
                      </TableHead>
                      <TableHead>{t("classes.columns.join")}</TableHead>
                      <TableHead>{t("classes.columns.created")}</TableHead>
                      <TableHead className="w-10" />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {items.map((klass) => (
                      <Row
                        key={klass.id}
                        klass={klass}
                        locale={locale}
                        onArchive={() => setArchiving(klass)}
                        onRestore={() => restore.mutate(klass.id)}
                      />
                    ))}
                  </TableBody>
                </Table>
              </Card>
              {classes.data && (
                <Pager
                  page={classes.data.page}
                  pageSize={classes.data.pageSize}
                  total={classes.data.total}
                />
              )}
            </>
          )}
        </>
      )}

      <NewClassDialog open={creating} onOpenChange={setCreating} />
      <ArchiveClassDialog
        klass={archiving}
        onOpenChange={(open) => {
          if (!open) setArchiving(null);
        }}
      />
    </div>
  );
}

function Row({
  klass,
  locale,
  onArchive,
  onRestore,
}: {
  klass: Class;
  locale: Locale;
  onArchive: () => void;
  onRestore: () => void;
}) {
  const { t } = useTranslation();
  const archived = klass.archivedAt !== null;
  const muted = archived ? "text-muted-foreground" : undefined;
  const href = `/admin/classes/${klass.id}`;

  return (
    <TableRow>
      <TableCell>
        <div className="flex items-center gap-2">
          <Link to={href} className={cn("font-medium hover:underline", muted)}>
            {klass.name}
          </Link>
          {archived && <Badge variant="outline">{t("classes.archivedBadge")}</Badge>}
        </div>
        {klass.description ? (
          <p className="text-muted-foreground text-xs">{klass.description}</p>
        ) : null}
      </TableCell>
      <TableCell className={cn("text-right tabular-nums", muted)}>
        {klass.studentCount}
      </TableCell>
      <TableCell
        className={cn(
          "text-right tabular-nums",
          (archived || klass.openAssignmentCount === 0) && "text-muted-foreground",
        )}
      >
        {klass.openAssignmentCount}
      </TableCell>
      <TableCell>
        {archived ? (
          <span className="text-muted-foreground">—</span>
        ) : isJoinOpen(klass) ? (
          <span className="flex items-center gap-2">
            <Badge variant="success">{t("classes.joinOpen")}</Badge>
            <span className="text-muted-foreground font-mono text-xs">
              {t("classes.codeHint", { hint: klass.joinCode?.hint })}
            </span>
          </span>
        ) : (
          <Badge variant="outline">{t("classes.joinClosed")}</Badge>
        )}
      </TableCell>
      <TableCell className="text-muted-foreground tabular-nums">
        {formatDayMonth(klass.createdAt, locale)}
      </TableCell>
      <TableCell className="text-right">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="size-7"
              aria-label={t("classes.actions")}
            >
              <Ellipsis className="size-3.5" aria-hidden="true" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-52">
            <DropdownMenuItem asChild>
              <Link to={href}>
                <ArrowUpRight className="text-muted-foreground" aria-hidden="true" />
                {t("classes.open")}
              </Link>
            </DropdownMenuItem>
            {archived ? null : (
              <>
                <DropdownMenuItem asChild>
                  <Link to={`/admin/assignments/new?classId=${klass.id}`}>
                    <Send className="text-muted-foreground" aria-hidden="true" />
                    {t("classes.assign")}
                  </Link>
                </DropdownMenuItem>
                <DropdownMenuItem asChild>
                  <Link to={href}>
                    <UserPlus className="text-muted-foreground" aria-hidden="true" />
                    {t("classes.addStudents")}
                  </Link>
                </DropdownMenuItem>
              </>
            )}
            <DropdownMenuSeparator />
            {archived ? (
              <DropdownMenuItem onSelect={onRestore}>
                <ArchiveRestore className="text-muted-foreground" aria-hidden="true" />
                {t("classes.restore")}
              </DropdownMenuItem>
            ) : (
              <DropdownMenuItem className="text-muted-foreground" onSelect={onArchive}>
                <Inbox aria-hidden="true" />
                {t("classes.archive")}
              </DropdownMenuItem>
            )}
          </DropdownMenuContent>
        </DropdownMenu>
      </TableCell>
    </TableRow>
  );
}

function currentLocale(language: string): Locale {
  return (SUPPORTED_LOCALES as readonly string[]).includes(language)
    ? (language as Locale)
    : "vi";
}
