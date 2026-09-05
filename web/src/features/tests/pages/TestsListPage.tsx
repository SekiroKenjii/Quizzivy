import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "react-router";
import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import {
  Archive,
  RotateCw,
  Copy,
  Eye,
  Filter,
  Headphones,
  History,
  Plus,
  Send,
  SquarePen,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuCheckboxItem,
} from "@/components/ui/dropdown-menu";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  archiveTest,
  restoreTest,
  createTest,
  duplicateTest,
  listTests,
  type Test,
  type TestStatus,
} from "@/features/tests/api";
import { useLocale } from "@/lib/i18n/useLocale";
import { formatRelative } from "@/lib/i18n/datetime";
import { useDebounced } from "@/lib/useDebounced";
import { ApiError } from "@/lib/api/errors";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { EmptyState, ListSkeleton, LoadError } from "@/components/shared/ListState";
import { PageHeader } from "@/components/shared/PageHeader";
import { RowMenu } from "@/components/shared/RowMenu";
import { SearchInput } from "@/components/shared/SearchInput";
import { toast } from "@/components/ui/sonner";
import { StatusBadge } from "@/components/shared/StatusBadge";
import { Pager } from "@/components/shared/Pager";
import { usePage } from "@/hooks/usePage";

const TABS: (TestStatus | "all")[] = ["all", "draft", "published", "archived"];

/** §8's tests list, as the deck's A-03. */
const PAGE_SIZE = 20;

export default function TestsListPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [tab, setTab] = useState<TestStatus | "all">("all");
  const [query, setQuery] = useState("");
  const [tags, setTags] = useState<readonly string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const search = useDebounced(query, 300);
  const locale = useLocale();

  const [page] = usePage(
    JSON.stringify({ tab, search: search.trim(), tags: [...tags] }),
  );
  const tests = useQuery({
    queryKey: ["admin-tests", { tab, search, tags, page }],
    queryFn: ({ signal }) =>
      listTests(
        {
          limit: PAGE_SIZE,
          page,
          ...(tab === "all" ? {} : { status: tab }),
          ...(tags.length > 0 ? { tag: [...tags] } : {}),
          ...(search.trim() === "" ? {} : { q: search.trim() }),
        },
        signal,
      ),
    placeholderData: keepPreviousData,
  });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["admin-tests"] });

  const create = useMutation({
    mutationFn: () => createTest(t("tests.untitled")),
    onSuccess: async (test) => {
      await invalidate();
      void navigate(`/admin/tests/${test.id}/edit`);
    },
    onError: (cause) => setError(message(cause, t("tests.createFailed"))),
  });

  const duplicate = useMutation({
    mutationFn: (id: string) => duplicateTest(id),
    onSuccess: async (test) => {
      await invalidate();
      void navigate(`/admin/tests/${test.id}/edit`);
    },
    onError: (cause) => setError(message(cause, t("tests.duplicateFailed"))),
  });

  const restore = useMutation({
    mutationFn: (test: Test) => restoreTest(test),
    onSuccess: async () => {
      await invalidate();
      toast(t("tests.restored"));
    },
    onError: (cause) => setError(message(cause, t("tests.restoreFailed"))),
  });

  const [archiving, setArchiving] = useState<Test | null>(null);
  const archive = useMutation({
    mutationFn: (test: Test) => archiveTest(test),
    onSuccess: async (archived, test) => {
      await invalidate();
      setArchiving(null);
      toast(
        t("tests.archived"),
        // Restoring only ever yields a draft, so undo is offered where that is the truth.
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
      setError(message(cause, t("tests.archiveFailed")));
    },
  });

  const items = tests.data?.items ?? [];
  const facets = tests.data?.facets;
  const offered = [...new Set([...tags, ...(tests.data?.tags ?? [])])].sort((a, b) =>
    a.localeCompare(b, "vi"),
  );

  return (
    <div className="space-y-4">
      <PageHeader
        variant="title"
        title={t("nav.tests")}
        subtitle={
          facets
            ? t("tests.summary", { count: facets.all, drafts: facets.draft })
            : "\u00a0"
        }
        actions={
          <Button size="sm" disabled={create.isPending} onClick={() => create.mutate()}>
            <Plus aria-hidden="true" />
            {t("tests.new")}
          </Button>
        }
      />

      <div className="flex items-center gap-2">
        <Tabs value={tab} onValueChange={(next) => setTab(next as TestStatus | "all")}>
          <TabsList aria-label={t("tests.statusFilter")}>
            {TABS.map((value) => (
              <TabsTrigger key={value} value={value}>
                {value === "all" ? t("tests.all") : t(`status.test.${value}`)}
                {facets ? (
                  <span className="text-muted-foreground ml-1 tabular-nums">
                    {facets[value]}
                  </span>
                ) : null}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        <SearchInput
          className="ml-auto"
          value={query}
          onChange={setQuery}
          placeholder={t("tests.searchPlaceholder")}
        />

        {/* A-03's "Thẻ". */}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" size="sm" disabled={offered.length === 0}>
              <Filter aria-hidden="true" />
              {tags.length === 0
                ? t("tests.tagFilter")
                : t("tests.tagFilterCount", { count: tags.length })}
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-56">
            {offered.map((tag) => (
              <DropdownMenuCheckboxItem
                key={tag}
                checked={tags.includes(tag)}
                onCheckedChange={() =>
                  setTags(
                    tags.includes(tag) ? tags.filter((x) => x !== tag) : [...tags, tag],
                  )
                }
              >
                {tag}
              </DropdownMenuCheckboxItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      {error === null ? null : (
        <p role="alert" className="text-destructive text-sm">
          {error}
        </p>
      )}

      {tests.isPending ? (
        <ListSkeleton />
      ) : tests.isError ? (
        <LoadError error={tests.error} onRetry={() => void tests.refetch()}>
          {t("tests.loadFailed")}
        </LoadError>
      ) : items.length === 0 ? (
        <EmptyState
          action={
            <Button
              size="sm"
              disabled={create.isPending}
              onClick={() => create.mutate()}
            >
              {t("tests.new")}
            </Button>
          }
        >
          {tab === "all" && search.trim() === "" && tags.length === 0
            ? t("tests.empty")
            : t("tests.noMatches")}
        </EmptyState>
      ) : (
        <>
          <Card className="gap-0 overflow-hidden py-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[40%]">{t("tests.title")}</TableHead>
                  <TableHead>{t("tests.status")}</TableHead>
                  <TableHead className="text-right">{t("tests.questions")}</TableHead>
                  <TableHead className="text-right">{t("tests.points")}</TableHead>
                  <TableHead>{t("tests.version")}</TableHead>
                  <TableHead>{t("tests.updated")}</TableHead>
                  <TableHead className="w-10">
                    <span className="sr-only">{t("tests.actions")}</span>
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((test) => (
                  <TableRow key={test.id}>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        {/* A-03: a draft opens the builder, anything else the read-only detail. */}
                        <Link
                          to={openHref(test)}
                          className="truncate font-medium hover:underline"
                        >
                          {test.title}
                        </Link>
                        {test.audioCount > 0 ? (
                          <Badge
                            variant="outline"
                            aria-label={t("tests.audioCount", {
                              count: test.audioCount,
                            })}
                          >
                            <Headphones aria-hidden="true" width="12" height="12" />
                            {test.audioCount}
                          </Badge>
                        ) : null}
                      </div>
                    </TableCell>
                    <TableCell>
                      <StatusBadge kind="test" status={test.status} />
                    </TableCell>
                    <TableCell className="text-right tabular-nums">
                      {test.questionCount}
                    </TableCell>
                    <TableCell className="text-right tabular-nums">
                      {test.totalPoints}
                    </TableCell>
                    <TableCell className="text-muted-foreground tabular-nums">
                      {test.currentVersion === 0
                        ? "—"
                        : t("tests.versionNumber", { n: test.currentVersion })}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {formatRelative(test.updatedAt, locale)}
                    </TableCell>
                    <TableCell className="text-right">
                      <RowActions
                        test={test}
                        onEdit={() => void navigate(`/admin/tests/${test.id}/edit`)}
                        onDuplicate={() => duplicate.mutate(test.id)}
                        onArchive={() => setArchiving(test)}
                        onRestore={() => restore.mutate(test)}
                      />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </Card>
          <ConfirmDialog
            open={archiving !== null}
            onOpenChange={(open) => !open && setArchiving(null)}
            title={t("tests.archiveConfirmTitle", { title: archiving?.title ?? "" })}
            description={t("tests.archiveConfirmBody")}
            confirmLabel={t("tests.archive")}
            destructive
            pending={archive.isPending}
            onConfirm={() => archiving && archive.mutate(archiving)}
          />

          {tests.data && (
            <Pager
              page={tests.data.page}
              pageSize={tests.data.pageSize}
              total={tests.data.total}
            />
          )}
        </>
      )}
    </div>
  );
}

function RowActions({
  test,
  onEdit,
  onDuplicate,
  onArchive,
  onRestore,
}: {
  test: Test;
  onEdit: () => void;
  onDuplicate: () => void;
  onArchive: () => void;
  onRestore: () => void;
}) {
  const { t } = useTranslation();

  if (test.status === "archived") {
    return (
      <RowMenu className="w-60">
        <DropdownMenuItem onSelect={onRestore}>
          <RotateCw aria-hidden="true" />
          {t("tests.restore")}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={onDuplicate}>
          <Copy aria-hidden="true" />
          {t("tests.duplicate")}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <p className="text-muted-foreground px-2 pt-0.5 pb-1 text-xs">
          {t("tests.restoreHint")}
        </p>
      </RowMenu>
    );
  }
  if (test.status === "draft") {
    return (
      <RowMenu>
        <DropdownMenuItem onSelect={onEdit}>
          <SquarePen aria-hidden="true" />
          {t("tests.edit")}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={onDuplicate}>
          <Copy aria-hidden="true" />
          {t("tests.duplicate")}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" onSelect={onArchive}>
          <Archive aria-hidden="true" />
          {t("tests.archive")}
        </DropdownMenuItem>
      </RowMenu>
    );
  }
  // A-03's menu for a published row, in the deck's order.
  return (
    <RowMenu className="w-60">
      <DropdownMenuItem asChild>
        <Link to={`/admin/tests/${test.id}`}>
          <Eye className="text-muted-foreground" aria-hidden="true" />
          {t("tests.preview")}
        </Link>
      </DropdownMenuItem>
      <DropdownMenuItem asChild>
        <Link to={`/admin/assignments/new?testId=${test.id}`}>
          <Send className="text-muted-foreground" aria-hidden="true" />
          {t("tests.assignToClass")}
        </Link>
      </DropdownMenuItem>
      <DropdownMenuItem onSelect={onDuplicate}>
        <Copy className="text-muted-foreground" aria-hidden="true" />
        {t("tests.duplicate")}
      </DropdownMenuItem>
      <DropdownMenuItem asChild>
        <Link to={`/admin/tests/${test.id}#versions`}>
          <History className="text-muted-foreground" aria-hidden="true" />
          {t("tests.versionHistory")}
        </Link>
      </DropdownMenuItem>
      <DropdownMenuSeparator />
      <DropdownMenuItem variant="destructive" onSelect={onArchive}>
        <Archive aria-hidden="true" />
        {t("tests.archive")}
      </DropdownMenuItem>
    </RowMenu>
  );
}

function openHref(test: Test): string {
  return test.status === "draft"
    ? `/admin/tests/${test.id}/edit`
    : `/admin/tests/${test.id}`;
}

function message(cause: unknown, fallback: string): string {
  return cause instanceof ApiError ? cause.message : fallback;
}
