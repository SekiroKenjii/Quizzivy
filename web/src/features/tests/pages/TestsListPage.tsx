import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import {
  Archive,
  Copy,
  Ellipsis,
  Filter,
  Headphones,
  Plus,
  Search,
  SquarePen,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
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
  createTest,
  duplicateTest,
  listTests,
  type Test,
  type TestStatus,
} from "@/features/tests/api";
import { SUPPORTED_LOCALES, type Locale } from "@/lib/i18n";
import { formatRelative } from "@/lib/i18n/datetime";
import { useDebounced } from "@/lib/useDebounced";
import { ApiError } from "@/lib/api/errors";
import { PageHeader } from "@/components/shared/PageHeader";
import { Pager } from "@/components/shared/Pager";
import { usePage } from "@/hooks/usePage";

const TABS: (TestStatus | "all")[] = ["all", "draft", "published", "archived"];

const STATUS_VARIANT: Record<TestStatus, "success" | "secondary" | "outline"> = {
  published: "success",
  draft: "secondary",
  archived: "outline",
};

/** §8's tests list, as the deck's A-03. */
const PAGE_SIZE = 20;

export default function TestsListPage() {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [tab, setTab] = useState<TestStatus | "all">("all");
  const [query, setQuery] = useState("");
  const [tags, setTags] = useState<readonly string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const search = useDebounced(query, 300);
  const locale = currentLocale(i18n.language);

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

  const archive = useMutation({
    mutationFn: (test: Test) => archiveTest(test),
    onSuccess: invalidate,
    onError: (cause) => setError(message(cause, t("tests.archiveFailed"))),
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
                {value === "all" ? t("tests.all") : t(`builder.${value}`)}
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
            placeholder={t("tests.searchPlaceholder")}
            aria-label={t("tests.searchPlaceholder")}
            onChange={(event) => setQuery(event.target.value)}
          />
        </div>

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
        <p role="status" aria-live="polite" className="text-muted-foreground text-sm">
          {t("common.loading")}
        </p>
      ) : tests.isError ? (
        <div className="space-y-3">
          <p role="alert" className="text-sm">
            {t("tests.loadFailed")}
          </p>
          <Button variant="outline" size="sm" onClick={() => void tests.refetch()}>
            {t("common.retry")}
          </Button>
        </div>
      ) : items.length === 0 ? (
        <div className="space-y-3">
          <p className="text-muted-foreground text-sm">
            {tab === "all" && search.trim() === "" && tags.length === 0
              ? t("tests.empty")
              : t("tests.noMatches")}
          </p>
          <Button size="sm" disabled={create.isPending} onClick={() => create.mutate()}>
            {t("tests.new")}
          </Button>
        </div>
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
                        <button
                          type="button"
                          className="truncate text-left font-medium"
                          onClick={() => void navigate(`/admin/tests/${test.id}`)}
                        >
                          {test.title}
                        </button>
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
                      <Badge variant={STATUS_VARIANT[test.status]}>
                        {t(`builder.${test.status}`)}
                      </Badge>
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
                        onArchive={() => archive.mutate(test)}
                      />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </Card>

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
}: {
  test: Test;
  onEdit: () => void;
  onDuplicate: () => void;
  onArchive: () => void;
}) {
  const { t } = useTranslation();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon-xs"
          aria-label={t("tests.actionsFor", { title: test.title })}
        >
          <Ellipsis aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onSelect={onEdit}>
          <SquarePen aria-hidden="true" />
          {t("tests.edit")}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={onDuplicate}>
          <Copy aria-hidden="true" />
          {t("tests.duplicate")}
        </DropdownMenuItem>
        <DropdownMenuItem
          variant="destructive"
          disabled={test.status === "archived"}
          onSelect={onArchive}
        >
          <Archive aria-hidden="true" />
          {t("tests.archive")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function message(cause: unknown, fallback: string): string {
  return cause instanceof ApiError ? cause.message : fallback;
}

function currentLocale(language: string): Locale {
  return (SUPPORTED_LOCALES as readonly string[]).includes(language)
    ? (language as Locale)
    : "vi";
}
