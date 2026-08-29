import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { useInfiniteQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Archive,
  Copy,
  Ellipsis,
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

const TABS: (TestStatus | "all")[] = ["all", "draft", "published", "archived"];

const STATUS_VARIANT: Record<TestStatus, "success" | "secondary" | "outline"> = {
  published: "success",
  draft: "secondary",
  archived: "outline",
};

/** §8's tests list, as the deck's A-03. */
export default function TestsListPage() {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [tab, setTab] = useState<TestStatus | "all">("all");
  const [query, setQuery] = useState("");
  const [error, setError] = useState<string | null>(null);
  const search = useDebounced(query, 300);
  const locale = currentLocale(i18n.language);

  const tests = useInfiniteQuery({
    queryKey: ["admin-tests", { tab, search }],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) =>
      listTests(
        {
          limit: 50,
          ...(tab === "all" ? {} : { status: tab }),
          ...(search.trim() === "" ? {} : { q: search.trim() }),
          ...(pageParam ? { cursor: pageParam } : {}),
        },
        signal,
      ),
    getNextPageParam: (page) => page.nextCursor ?? undefined,
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

  const items = tests.data?.pages.flatMap((page) => page.items) ?? [];
  // Every page carries the same facets (they ignore the cursor), so the first
  // one is the answer -- and it is the only page guaranteed to exist.
  const facets = tests.data?.pages[0]?.facets;

  return (
    <div className="space-y-4">
      <div className="flex items-end justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">{t("nav.tests")}</h1>
          <p className="text-muted-foreground mt-0.5 text-sm">
            {facets
              ? t("tests.summary", { count: facets.all, drafts: facets.draft })
              : "\u00a0"}
          </p>
        </div>
        <Button size="sm" disabled={create.isPending} onClick={() => create.mutate()}>
          <Plus aria-hidden="true" />
          {t("tests.new")}
        </Button>
      </div>

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
            {tab === "all" && search.trim() === ""
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
                            <Headphones aria-hidden="true" />
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

          {tests.hasNextPage ? (
            <Button
              variant="outline"
              size="sm"
              disabled={tests.isFetchingNextPage}
              onClick={() => void tests.fetchNextPage()}
            >
              {tests.isFetchingNextPage ? t("common.loading") : t("bank.loadMore")}
            </Button>
          ) : null}
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
