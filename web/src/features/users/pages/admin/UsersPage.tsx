import { useCallback, useEffect, useRef } from "react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { LoadError } from "@/components/shared/ListState";
import { SearchInput, TOOLBAR_SEARCH } from "@/components/shared/SearchInput";
import { Button } from "@/components/ui/button";
import { Segmented, type SegmentedOption } from "@/components/ui/segmented";
import { StateChip } from "@/features/users/components/StateChip";
import { UsersTable } from "@/features/users/components/UsersTable";
import { CARDS_BELOW } from "@/features/users/components/userColumns";
import { UsersSkeleton } from "@/features/users/components/UsersSkeleton";
import {
  SEARCH_MAX,
  SEARCH_MIN,
  USER_PAGE_SIZES,
  USER_TABS,
  usersQuery,
  type AdminUserFacets,
} from "@/features/users/api";
import {
  isTooShort,
  limitOf,
  pageOf,
  searchOf,
  stateOf,
  tabOf,
} from "@/features/users/userFilters";
import { useDebouncedInput } from "@/hooks/useDebouncedInput";
import { useListFilters } from "@/hooks/useListFilters";
import { usePage, usePageSize } from "@/hooks/usePage";
import { PageHead } from "@/layouts/shell/PageHead";
import { useContentWidthAtLeast } from "@/layouts/shell/contentWidth";
import { useDisplayTimeZone } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { pageRange } from "@/lib/pagination";

const FILTER_PARAMS = ["tab", "q", "status", "page"] as const;

function tabOptions(
  t: TFunction,
  facets: AdminUserFacets | undefined,
): SegmentedOption[] {
  return USER_TABS.map((tab) => ({
    value: tab,
    label: t(`users.tabs.${tab}`),
    count: facets?.[tab],
  }));
}

function announcement(t: TFunction, total: number | undefined): string {
  if (total === undefined) return "";
  return total === 0 ? t("users.resultNone") : t("users.result", { count: total });
}

/**
 * UsersPage is the Admin console's Users screen (T-R5.22a): everyone who can
 * sign in, in a table the Admin narrows by tab (All, Teachers, Students,
 * Admins, Disabled, each with its count), by a search of the name and the
 * email, and by a state the Overview links to, shown as one removable chip.
 * The tab (`tab`), the search (`q`), the state (`status`), the page and its
 * size live in the URL; the search reaches it, and the request, once the
 * Admin has paused for 300 ms. A search under three characters is not sent, and
 * the page says so (QC-N-15). A page past the end goes to the last one. The
 * table's columns join as the content widens, and below 560px each account is
 * a card.
 */
export default function UsersPage() {
  useDisplayTimeZone();
  const { t } = useTranslation();
  const locale = useLocale();
  const { params, setParams, setFilter } = useListFilters();
  const tab = tabOf(params.get("tab"));
  const state = stateOf(params.get("status"));
  const applied = params.get("q") ?? "";
  const applyQuery = useCallback((value: string) => setFilter("q", value), [setFilter]);
  const [typed, setTyped] = useDebouncedInput(applied, applyQuery, searchOf);
  const q = searchOf(applied);
  const [size] = usePageSize(USER_PAGE_SIZES);
  const [asked, setPage] = usePage(JSON.stringify({ tab, state, q, size }));
  const page = pageOf(asked);
  const cards = !useContentWidthAtLeast(CARDS_BELOW);
  const searchRef = useRef<HTMLInputElement>(null);

  const users = useQuery({
    ...usersQuery({ tab, status: state, q, page, limit: limitOf(size) }),
    placeholderData: keepPreviousData,
  });
  const data = users.data;
  const pastEnd = data !== undefined && data.items.length === 0 && data.total > 0;
  const lastPage =
    pastEnd && !users.isPlaceholderData
      ? pageRange(page, data.pageSize, data.total).page
      : null;
  const toLastPage = pastEnd && (users.isPlaceholderData || lastPage !== page);
  useEffect(() => {
    if (lastPage !== null && lastPage !== page) setPage(lastPage);
  }, [lastPage, page, setPage]);

  const clearFilters = () => {
    setParams(
      (previous) => {
        const next = new URLSearchParams(previous);
        for (const name of FILTER_PARAMS) next.delete(name);
        return next;
      },
      { replace: true },
    );
    searchRef.current?.focus();
  };
  const removeState = () => {
    setFilter("status", null);
    searchRef.current?.focus();
  };
  const filtered = tab !== "all" || state !== null || q !== "";
  const empty = filtered ? (
    <span className="flex flex-col items-center gap-3">
      {t("users.empty")}
      <Button variant="outline" size="sm" onClick={clearFilters}>
        {t("users.clearFilters")}
      </Button>
    </span>
  ) : (
    t("users.empty")
  );

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <PageHead title={t("users.title")} description={t("users.lead")} />
      <p role="status" className="sr-only">
        {announcement(t, data?.total)}
      </p>

      <div className="flex flex-wrap items-center justify-between gap-2.5">
        <div className="max-w-full min-w-0">
          <Segmented
            label={t("users.tabs.label")}
            value={tab}
            options={tabOptions(t, data?.facets)}
            onChange={(value) => setFilter("tab", value === "all" ? null : value)}
            scroll
          />
        </div>
        <SearchInput
          className={TOOLBAR_SEARCH}
          value={typed}
          onChange={setTyped}
          placeholder={t("users.search.placeholder")}
          inputRef={searchRef}
          maxLength={SEARCH_MAX}
        />
      </div>

      {isTooShort(typed) ? (
        <p role="status" className="text-muted-fg text-meta">
          {t("users.search.short", { min: SEARCH_MIN })}
        </p>
      ) : null}

      {state === null ? null : <StateChip state={state} onRemove={removeState} />}

      {users.isError ? (
        <LoadError error={users.error} onRetry={() => void users.refetch()}>
          {t("users.loadFailed")}
        </LoadError>
      ) : null}
      {data === undefined || toLastPage ? (
        (users.isPending || toLastPage) && <UsersSkeleton />
      ) : (
        <UsersTable
          users={data.items}
          page={data.page}
          pageSize={data.pageSize}
          total={data.total}
          cards={cards}
          empty={empty}
          locale={locale}
        />
      )}
    </div>
  );
}
