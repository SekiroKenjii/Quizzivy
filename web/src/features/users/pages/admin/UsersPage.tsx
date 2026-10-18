import { useRef, useState } from "react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { Upload } from "lucide-react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { LoadError } from "@/components/shared/ListState";
import { SearchInput, TOOLBAR_SEARCH } from "@/components/shared/SearchInput";
import { Button } from "@/components/ui/button";
import { Segmented, type SegmentedOption } from "@/components/ui/segmented";
import { ImportUsersDialog } from "@/features/users/components/ImportUsersDialog";
import { StateChip } from "@/features/users/components/StateChip";
import { UsersTable } from "@/features/users/components/UsersTable";
import { CARDS_BELOW } from "@/features/users/components/userColumns";
import { UsersSkeleton } from "@/features/users/components/UsersSkeleton";
import {
  SEARCH_MIN,
  USER_PAGE_SIZES,
  USER_TABS,
  usersQuery,
  type AdminUserFacets,
} from "@/features/users/api";
import {
  isTooShort,
  limitOf,
  searchOf,
  stateOf,
  tabOf,
} from "@/features/users/userFilters";
import { useListFilters } from "@/hooks/useListFilters";
import { usePage, usePageSize } from "@/hooks/usePage";
import { PageHead } from "@/layouts/shell/PageHead";
import { useContentWidthAtLeast } from "@/layouts/shell/contentWidth";
import { useDisplayTimeZone } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { useDebounced } from "@/lib/useDebounced";

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

/**
 * UsersPage is the Admin console's Users screen (T-R5.22a): everyone who can
 * sign in, in a table the Admin narrows by tab (All, Teachers, Students,
 * Admins, Disabled, each with its count), by a search of the name and the
 * email, and by a state the Overview links to, shown as one removable chip.
 * The tab (`tab`), the search (`q`), the state (`status`), the page and its
 * size live in the URL. A search under three characters is not sent, and the
 * page says so (QC-N-15). The table's columns join as the content widens, and
 * below 560px each account is a card. "Import CSV" opens the Import users
 * dialog (T-R5.23), which stays mounted so a job it started is followed while
 * it is closed.
 */
export default function UsersPage() {
  useDisplayTimeZone();
  const { t } = useTranslation();
  const locale = useLocale();
  const { params, setParams, setFilter } = useListFilters();
  const tab = tabOf(params.get("tab"));
  const state = stateOf(params.get("status"));
  const typed = params.get("q") ?? "";
  const settled = useDebounced(typed, 300);
  const q = searchOf(settled);
  const [size] = usePageSize(USER_PAGE_SIZES);
  const [page] = usePage(JSON.stringify({ tab, state, q, size }));
  const cards = !useContentWidthAtLeast(CARDS_BELOW);
  const [importing, setImporting] = useState(false);
  const importButton = useRef<HTMLButtonElement>(null);

  const users = useQuery({
    ...usersQuery({ tab, status: state, q, page, limit: limitOf(size) }),
    placeholderData: keepPreviousData,
  });
  const data = users.data;

  const clearFilters = () =>
    setParams(
      (previous) => {
        const next = new URLSearchParams(previous);
        for (const name of FILTER_PARAMS) next.delete(name);
        return next;
      },
      { replace: true },
    );
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
      <PageHead
        title={t("users.title")}
        description={t("users.lead")}
        actions={
          <Button
            ref={importButton}
            variant="outline"
            onClick={() => setImporting(true)}
          >
            <Upload aria-hidden="true" />
            {t("users.import.open")}
          </Button>
        }
      />
      <ImportUsersDialog
        open={importing}
        onOpenChange={setImporting}
        returnFocus={importButton}
      />

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
          onChange={(value) => setFilter("q", value)}
          placeholder={t("users.search.placeholder")}
        />
      </div>

      {isTooShort(typed) ? (
        <p role="status" className="text-muted-fg text-meta">
          {t("users.search.short", { min: SEARCH_MIN })}
        </p>
      ) : null}

      {state === null ? null : (
        <StateChip state={state} onRemove={() => setFilter("status", null)} />
      )}

      {users.isError ? (
        <LoadError error={users.error} onRetry={() => void users.refetch()}>
          {t("users.loadFailed")}
        </LoadError>
      ) : null}
      {data === undefined ? (
        users.isPending && <UsersSkeleton />
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
