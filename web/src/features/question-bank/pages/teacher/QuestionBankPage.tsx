import { useMemo, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { keepPreviousData, useQuery, useQueryClient } from "@tanstack/react-query";
import { FilePlus, Plus, SlidersHorizontal, Tag, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { BulkActions, BulkBarButton } from "@/components/shared/BulkActions";
import { DataTable } from "@/components/shared/data/DataTable";
import { Pager } from "@/components/shared/data/Pager";
import { LoadError } from "@/components/shared/ListState";
import { SearchInput } from "@/components/shared/SearchInput";
import { Sheet } from "@/components/shared/Sheet";
import { AddToTestDialog } from "@/features/question-bank/components/AddToTestDialog";
import { BulkTagDialog } from "@/features/question-bank/components/BulkTagDialog";
import {
  deleteQuestion,
  listQuestions,
  type AdminQuestion,
} from "@/features/question-bank/api";
import { BankNavigation } from "@/features/question-groups/components/BankNavigation";
import { useBulkSelection } from "@/hooks/useBulkSelection";
import { useListFilters } from "@/hooks/useListFilters";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { usePage, usePageSize } from "@/hooks/usePage";
import { PageHead } from "@/layouts/shell/PageHead";
import { useDisplayTimeZone } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { useDebounced } from "@/lib/useDebounced";
import { BankFilterPanel, type BankFilterChange } from "./BankFilterPanel";
import { activeFilterCount, promptLine, readBankFilters } from "./bankFilters";
import { BANK_ASIDE_WIDTH, bankColumns } from "./bankColumns";

const QUERY_KEY = ["admin-questions"] as const;

const SEARCH =
  "w-auto min-w-0 flex-[1_1_240px] [&_input]:bg-card [&_input]:border-border [&_input]:h-8.5 [&_input]:pl-8.5 [&_input]:text-sm [&_svg]:top-[9.5px] [&_svg]:size-3.75";

const FILTERS_BUTTON =
  "bg-card hover:bg-muted inline-flex h-8.5 flex-none cursor-pointer items-center gap-1.5 rounded-lg border px-3 text-sm leading-4";

const COUNT =
  "bg-primary text-primary-fg text-caption rounded-full px-1.5 leading-normal tabular-nums";

const EMPTY_ACTION = "text-fg cursor-pointer underline underline-offset-2";

function filterParam(
  change: BankFilterChange,
): [string, string | readonly string[] | null] {
  if ("types" in change) return ["type", change.types];
  if ("levels" in change) return ["level", change.levels];
  if ("skills" in change) return ["skill", change.skills];
  if ("tags" in change) return ["tag", change.tags];
  if ("tagMatch" in change)
    return ["tagMatch", change.tagMatch === "all" ? "all" : null];
  return ["hasAudio", change.audio ? "true" : null];
}

function EmptyLine({
  children,
  action,
}: Readonly<{ children: string; action?: { label: string; run: () => void } }>) {
  return (
    <span>
      {children}
      {action && (
        <>
          {" "}
          <button type="button" className={EMPTY_ACTION} onClick={action.run}>
            {action.label}
          </button>
        </>
      )}
    </span>
  );
}

function BankSkeleton() {
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
        <div key={index} className="flex items-center gap-3 border-t px-4 py-3.5">
          <Skeleton className="size-4 flex-none rounded-[0.25rem]" />
          <Skeleton className="h-4 flex-1" />
          <Skeleton className="h-4 w-20 flex-none" />
        </div>
      ))}
    </div>
  );
}

/**
 * QuestionBankPage is the teacher's question bank as the deck draws it: the
 * filters in a 220px aside from 1024px and in a sheet below it, a search by
 * prompt, the questions in a table whose columns drop as the table narrows,
 * and a bulk bar that adds the selection to a test, tags it or deletes it.
 * The filters, the search, the page and its size live in the URL, so a
 * filtered list survives a reload and a trip to the editor and back. A row
 * opens the question's editor.
 */
export default function QuestionBankPage() {
  useDisplayTimeZone();
  const { t } = useTranslation();
  const locale = useLocale();
  const client = useQueryClient();
  const { params, setParams, setFilter } = useListFilters();
  const filters = readBankFilters(params);
  const search = useDebounced(filters.query.trim(), 300);
  const wide = useMediaQuery("(min-width: 1024px)");
  const [size] = usePageSize();
  const [sheetOpen, setSheetOpen] = useState(false);
  const [tagging, setTagging] = useState(false);
  const [adding, setAdding] = useState(false);
  const bulk = useBulkSelection<AdminQuestion>();
  const { types, levels, skills, tags, tagMatch, audio } = filters;
  const [page] = usePage(
    JSON.stringify({ types, levels, skills, tags, tagMatch, audio, search, size }),
  );
  const bank = useQuery({
    queryKey: [
      ...QUERY_KEY,
      { types, levels, skills, tags, tagMatch, audio, search, page, size },
    ],
    queryFn: ({ signal }) =>
      listQuestions(
        {
          limit: size,
          page,
          ...(types.length > 0 ? { type: [...types] } : {}),
          ...(levels.length > 0 ? { level: [...levels] } : {}),
          ...(skills.length > 0 ? { skill: [...skills] } : {}),
          ...(tags.length > 0 ? { tag: [...tags] } : {}),
          ...(tags.length > 1 && tagMatch === "all" ? { tagMatch } : {}),
          ...(audio ? { hasAudio: true } : {}),
          ...(search === "" ? {} : { q: search }),
        },
        signal,
      ),
    placeholderData: keepPreviousData,
  });
  const columns = useMemo(() => bankColumns(t, wide ? BANK_ASIDE_WIDTH : 0), [t, wide]);
  const figure = useMemo(() => new Intl.NumberFormat(locale), [locale]);

  const invalidate = () => client.invalidateQueries({ queryKey: QUERY_KEY });
  const change = (next: BankFilterChange) => {
    const [name, value] = filterParam(next);
    setFilter(name, value);
  };
  const clear = (names: readonly string[]) =>
    setParams(
      (current) => {
        const next = new URLSearchParams(current);
        for (const name of [...names, "page"]) next.delete(name);
        return next;
      },
      { replace: true },
    );
  const clearTags = () => clear(["tag", "tagMatch"]);
  const clearFilters = () =>
    clear(["type", "level", "skill", "tag", "tagMatch", "hasAudio", "q"]);

  const data = bank.data;
  const active = activeFilterCount(filters);
  const selected = [...bulk.selected.values()];
  const newQuestion = (
    <Button asChild>
      <Link to="/teacher/question-bank/new">
        <Plus aria-hidden="true" />
        {t("bank.newQuestion")}
      </Link>
    </Button>
  );

  const empty = (): ReactNode => {
    if (data === undefined || data.items.length > 0) return null;
    if (data.bankTotal === 0) return <EmptyLine>{t("bank.empty")}</EmptyLine>;
    if (tags.length > 0)
      return (
        <EmptyLine action={{ label: t("bank.clearTags"), run: clearTags }}>
          {t("bank.noTagMatches")}
        </EmptyLine>
      );
    return (
      <EmptyLine action={{ label: t("bank.clearFilters"), run: clearFilters }}>
        {t("bank.noMatches")}
      </EmptyLine>
    );
  };

  const panel = (
    <BankFilterPanel
      filters={filters}
      facets={data?.facets}
      tags={data?.tags ?? []}
      onChange={change}
    />
  );

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <PageHead
        title={t("teacherShell.nav.questionBank")}
        description={
          data === undefined
            ? " "
            : t("bank.description", {
                count: data.bankTotal,
                total: figure.format(data.bankTotal),
              })
        }
        actions={newQuestion}
      />
      <BankNavigation />

      <div className="flex items-start gap-3.5">
        {wide && (
          <aside aria-label={t("bank.filters")} className="sticky top-0 w-55 flex-none">
            {panel}
          </aside>
        )}
        <div className="flex min-w-0 flex-1 flex-col gap-2.5">
          <div className="flex flex-wrap gap-2">
            <SearchInput
              className={SEARCH}
              value={filters.query}
              onChange={(value) => setFilter("q", value)}
              placeholder={t("bank.searchPlaceholder")}
            />
            {!wide && (
              <button
                type="button"
                className={FILTERS_BUTTON}
                aria-haspopup="dialog"
                aria-label={
                  active > 0
                    ? t("bank.filtersActive", { count: active })
                    : t("bank.filters")
                }
                onClick={() => setSheetOpen(true)}
              >
                <SlidersHorizontal aria-hidden="true" className="size-3.5" />
                {t("bank.filters")}
                {active > 0 && (
                  <span aria-hidden="true" className={COUNT}>
                    {active}
                  </span>
                )}
              </button>
            )}
          </div>

          <BulkActions
            selected={selected}
            name={promptLine}
            actions={[
              {
                label: t("bank.delete"),
                description: t("bank.deleteSelectedBody"),
                icon: Trash2,
                run: (question) => deleteQuestion(question.id),
              },
            ]}
            onRemoved={bulk.remove}
            onClear={bulk.clear}
            onSettled={invalidate}
          >
            <BulkBarButton icon={FilePlus} onClick={() => setAdding(true)}>
              {t("bank.addToTest")}
            </BulkBarButton>
            <BulkBarButton icon={Tag} onClick={() => setTagging(true)}>
              {t("bank.bulkTag")}
            </BulkBarButton>
          </BulkActions>

          {bank.isError && (
            <LoadError error={bank.error} onRetry={() => void bank.refetch()}>
              {t("bank.loadFailed")}
            </LoadError>
          )}
          {data === undefined ? (
            bank.isPending && <BankSkeleton />
          ) : (
            <DataTable
              label={t("teacherShell.nav.questionBank")}
              columns={columns}
              rows={data.items}
              rowSize={{ padY: 10 }}
              rowHref={(question) => `/teacher/question-bank/${question.id}`}
              selection={bulk}
              rowName={promptLine}
              empty={empty()}
              footer={
                <Pager
                  page={data.page}
                  pageSize={data.pageSize}
                  total={data.total}
                  noun={(count) => t("bank.questionsNoun", { count })}
                />
              }
            />
          )}
        </div>
      </div>

      <Sheet
        open={!wide && sheetOpen}
        onOpenChange={setSheetOpen}
        title={t("bank.filters")}
        width={320}
      >
        {panel}
      </Sheet>
      <BulkTagDialog
        questionIds={[...bulk.selected.keys()]}
        suggestions={data?.tags ?? []}
        open={tagging}
        onOpenChange={setTagging}
        onApplied={bulk.clear}
      />
      <AddToTestDialog
        questionIds={[...bulk.selected.keys()]}
        open={adding}
        onOpenChange={setAdding}
        onAdded={bulk.clear}
      />
    </div>
  );
}
