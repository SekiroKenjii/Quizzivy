import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { TFunction } from "i18next";
import { Archive, Copy, Plus, Trash2, Undo2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { Segmented } from "@/components/ui/segmented";
import { BulkActions } from "@/components/shared/BulkActions";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { ListSkeleton, LoadError } from "@/components/shared/ListState";
import { SearchInput } from "@/components/shared/SearchInput";
import { DataTable, type DataColumn } from "@/components/shared/data/DataTable";
import { Pager } from "@/components/shared/data/Pager";
import { useBulkSelection } from "@/hooks/useBulkSelection";
import { useListFilters } from "@/hooks/useListFilters";
import { usePage, usePageSize } from "@/hooks/usePage";
import { PageHead } from "@/layouts/shell/PageHead";
import { ApiError } from "@/lib/api/errors";
import { formatRelative, useDisplayTimeZone } from "@/lib/i18n/datetime";
import type { Locale } from "@/lib/i18n";
import { useLocale } from "@/lib/i18n/useLocale";
import { useDebounced } from "@/lib/useDebounced";
import {
  archiveGroup,
  copyGroup,
  createGroup,
  deleteGroup,
  listGroups,
  type GroupSummary,
} from "../../api";
import { emptyGroup } from "../../model";
import { BankNavigation } from "../../components/BankNavigation";

const QUERY_KEY = ["admin-groups"] as const;

const SEARCH =
  "w-auto min-w-0 flex-[1_1_240px] [&_input]:bg-card [&_input]:border-border [&_input]:h-8.5 [&_input]:pl-8.5 [&_input]:text-sm [&_svg]:top-[9.5px] [&_svg]:size-3.75";

type Action = { kind: "archive" | "restore" | "delete"; group: GroupSummary };

const ACTION_LABELS = {
  delete: { title: "common.deletePermanently", description: "groups.deleteBody" },
  restore: { title: "common.restore", description: "groups.restoreBody" },
  archive: { title: "common.archive", description: "groups.archiveBody" },
};

function groupColumns(
  t: TFunction,
  locale: Locale,
  recent: ReadonlySet<string>,
): DataColumn<GroupSummary>[] {
  return [
    {
      id: "title",
      header: t("groups.titleLabel"),
      track: "minmax(160px,1fr)",
      cell: (group, shown) => (
        <span className="block min-w-0">
          <span className="block font-medium [overflow-wrap:anywhere]">
            {group.title}
          </span>
          {!shown.has("questions") && (
            <span className="text-muted-fg text-meta block">
              {t("groups.questions", { count: group.questionCount })}
            </span>
          )}
        </span>
      ),
      aside: (group) =>
        recent.has(group.id) ? (
          <Badge variant="outline" className="mt-1">
            {t("common.justDuplicated")}
          </Badge>
        ) : null,
    },
    {
      id: "questions",
      header: t("groups.questionCount"),
      track: "110px",
      align: "end",
      showFrom: 440,
      cell: (group) => <span className="tabular-nums">{group.questionCount}</span>,
    },
    {
      id: "points",
      header: t("bank.points"),
      track: "90px",
      align: "end",
      showFrom: 560,
      cell: (group) => <span className="tabular-nums">{group.totalPoints}</span>,
    },
    {
      id: "updated",
      header: t("common.updatedAt"),
      track: "150px",
      showFrom: 680,
      cell: (group) => (
        <span className="text-muted-fg">{formatRelative(group.updatedAt, locale)}</span>
      ),
    },
  ];
}

/**
 * GroupsListPage is the bank's question groups (DG-68): a search, Active or
 * Archived, a table whose row opens the group's editor, duplicate, archive,
 * restore and permanent delete from each row's menu, and the same actions
 * in bulk. The search, the status and the page live in the URL.
 */
export default function GroupsListPage() {
  useDisplayTimeZone();
  const { t } = useTranslation();
  const locale = useLocale();
  const navigate = useNavigate();
  const client = useQueryClient();
  const { params, setFilter } = useListFilters();
  const query = params.get("q") ?? "";
  const search = useDebounced(query.trim(), 300);
  const status = params.get("status") === "archived" ? "archived" : "active";
  const [size] = usePageSize();
  const [page, setPage] = usePage(JSON.stringify({ search, status, size }));
  const bulk = useBulkSelection<GroupSummary>();
  const [selectionStatus, setSelectionStatus] = useState(status);
  if (selectionStatus !== status) {
    setSelectionStatus(status);
    bulk.clear();
  }
  const [recent, setRecent] = useState<ReadonlySet<string>>(new Set());
  const [action, setAction] = useState<Action | null>(null);
  const [error, setError] = useState<string | null>(null);
  const list = useQuery({
    queryKey: [...QUERY_KEY, search, status, page, size],
    queryFn: ({ signal }) =>
      listGroups({ q: search, status, page, limit: size }, signal),
    placeholderData: (previous, previousQuery) =>
      previousQuery?.queryKey[2] === status ? previous : undefined,
  });
  const columns = useMemo(() => groupColumns(t, locale, recent), [t, locale, recent]);
  const refresh = () => client.invalidateQueries({ queryKey: QUERY_KEY });
  const fail = (cause: unknown) =>
    setError(cause instanceof ApiError ? cause.message : t("common.actionFailed"));
  const create = useMutation({
    mutationFn: () => createGroup({ bundle: emptyGroup(t("groups.newGroup")) }),
    onSuccess: (saved) => {
      client.setQueryData(["admin-group", saved.bundle.group.id], saved);
      void refresh();
      void navigate(`/teacher/question-bank/groups/${saved.bundle.group.id}`);
    },
    onError: fail,
  });
  const duplicate = useMutation({
    mutationFn: (group: GroupSummary) =>
      copyGroup(group.id, { expectedRevision: group.revision }),
    onSuccess: (saved) => {
      setPage(1);
      setRecent((previous) => new Set([...previous, saved.bundle.group.id]));
      void refresh();
    },
    onError: fail,
  });
  const mutate = useMutation({
    mutationFn: async ({ kind, group }: Action): Promise<unknown> =>
      kind === "delete"
        ? deleteGroup(group.id, group.revision)
        : archiveGroup(group, kind === "archive"),
    onSuccess: (_saved, input) => {
      bulk.remove([input.group.id]);
      setAction(null);
      void refresh();
    },
    onError: fail,
  });
  const selection = [...bulk.selected.values()];
  const selectedArchived = selection.every((item) => item.archivedAt !== null);
  const selectedActive = selection.every((item) => item.archivedAt === null);
  const ask = (next: Action) => {
    setError(null);
    setAction(next);
  };
  const data = list.data;

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <PageHead
        title={t("groups.bankTitle")}
        description={t("groups.bankHint")}
        actions={
          <Button
            disabled={create.isPending}
            onClick={() => {
              setError(null);
              create.mutate();
            }}
          >
            <Plus aria-hidden="true" />
            {t("groups.create")}
          </Button>
        }
      />
      <BankNavigation />
      <div className="flex flex-wrap items-center gap-2">
        <SearchInput
          className={SEARCH}
          value={query}
          onChange={(value) => setFilter("q", value)}
          placeholder={t("groups.search")}
        />
        <Segmented
          label={t("common.status")}
          value={status}
          options={[
            { value: "active", label: t("groups.active") },
            { value: "archived", label: t("common.archived") },
          ]}
          onChange={(value) => setFilter("status", value === "active" ? null : value)}
        />
      </div>
      {error && !action ? (
        <p role="alert" className="text-danger text-sm">
          {error}
        </p>
      ) : null}
      <BulkActions
        selected={selection}
        name={(group) => group.title}
        onRemoved={bulk.remove}
        onClear={bulk.clear}
        onSettled={refresh}
        actions={[
          ...(selectedActive
            ? [
                {
                  label: t("common.archive"),
                  description: t("groups.archiveBody"),
                  icon: Archive,
                  run: (group: GroupSummary) => archiveGroup(group, true),
                },
              ]
            : []),
          ...(selectedArchived
            ? [
                {
                  label: t("common.restore"),
                  description: t("groups.restoreBody"),
                  icon: Undo2,
                  run: (group: GroupSummary) => archiveGroup(group, false),
                },
                {
                  label: t("common.deletePermanently"),
                  description: t("groups.deleteBody"),
                  icon: Trash2,
                  run: (group: GroupSummary) => deleteGroup(group.id, group.revision),
                },
              ]
            : []),
        ]}
      />
      {list.isError && (
        <LoadError error={list.error} onRetry={() => void list.refetch()}>
          {t("groups.loadFailed")}
        </LoadError>
      )}
      {data === undefined ? (
        list.isPending && <ListSkeleton rows={6} />
      ) : (
        <DataTable
          label={t("groups.bankTitle")}
          columns={columns}
          rows={data.items}
          rowSize={{ padY: 12 }}
          rowHref={(group) => `/teacher/question-bank/groups/${group.id}`}
          selection={bulk}
          rowName={(group) => group.title}
          empty={data.items.length === 0 ? t("groups.empty") : null}
          menu={(group) => (
            <>
              <DropdownMenuItem
                disabled={duplicate.isPending || group.archivedAt !== null}
                onSelect={() => duplicate.mutate(group)}
              >
                <Copy aria-hidden="true" />
                {t("bank.duplicate")}
              </DropdownMenuItem>
              {group.archivedAt ? (
                <>
                  <DropdownMenuItem onSelect={() => ask({ kind: "restore", group })}>
                    <Undo2 aria-hidden="true" />
                    {t("common.restore")}
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    variant="destructive"
                    onSelect={() => ask({ kind: "delete", group })}
                  >
                    <Trash2 aria-hidden="true" />
                    {t("common.deletePermanently")}
                  </DropdownMenuItem>
                </>
              ) : (
                <DropdownMenuItem onSelect={() => ask({ kind: "archive", group })}>
                  <Archive aria-hidden="true" />
                  {t("common.archive")}
                </DropdownMenuItem>
              )}
            </>
          )}
          footer={
            <Pager
              page={data.page}
              pageSize={data.pageSize}
              total={data.total}
              noun={(count) => t("groups.noun", { count })}
            />
          }
        />
      )}
      <ConfirmDialog
        open={action !== null}
        onOpenChange={(open) => {
          if (!open && !mutate.isPending) setAction(null);
        }}
        title={t(ACTION_LABELS[action?.kind ?? "archive"].title)}
        description={t(ACTION_LABELS[action?.kind ?? "archive"].description)}
        confirmLabel={t("common.confirm")}
        destructive={action?.kind === "delete"}
        pending={mutate.isPending}
        error={error}
        onConfirm={() => action && mutate.mutate(action)}
      >
        <p className="text-sm font-medium">{action?.group.title}</p>
      </ConfirmDialog>
    </div>
  );
}
