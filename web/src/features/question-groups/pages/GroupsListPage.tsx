import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Archive, Copy, Plus, Trash2, Undo2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { PageHeader } from "@/components/shared/PageHeader";
import { SearchInput } from "@/components/shared/SearchInput";
import { Pager } from "@/components/shared/Pager";
import { RowMenu } from "@/components/shared/RowMenu";
import { BulkActions } from "@/components/shared/BulkActions";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { EmptyState, ListSkeleton, QueryStates } from "@/components/shared/ListState";
import { useBulkSelection } from "@/hooks/useBulkSelection";
import { useListFilters } from "@/hooks/useListFilters";
import { usePage } from "@/hooks/usePage";
import { useDebounced } from "@/lib/useDebounced";
import { useLocale } from "@/lib/i18n/useLocale";
import { formatRelative } from "@/lib/i18n/datetime";
import { ApiError } from "@/lib/api/errors";
import {
  archiveGroup,
  copyGroup,
  createGroup,
  deleteGroup,
  listGroups,
  type GroupSummary,
} from "../api";
import { emptyGroup } from "../model";
import { BankNavigation } from "../components/BankNavigation";

type Action = { kind: "archive" | "restore" | "delete"; group: GroupSummary };
export default function GroupsListPage() {
  const { t } = useTranslation();
  const locale = useLocale();
  const navigate = useNavigate();
  const client = useQueryClient();
  const { params, setFilter } = useListFilters();
  const query = params.get("q") ?? "";
  const search = useDebounced(query.trim(), 300);
  const status = params.get("status") === "archived" ? "archived" : "active";
  const [page, setPage] = usePage(JSON.stringify({ search, status }));
  const bulk = useBulkSelection<GroupSummary>();
  const [recent, setRecent] = useState<ReadonlySet<string>>(new Set());
  const [action, setAction] = useState<Action | null>(null);
  const [error, setError] = useState<string | null>(null);
  const list = useQuery({
    queryKey: ["admin-groups", search, status, page],
    queryFn: ({ signal }) => listGroups({ q: search, status, page, limit: 20 }, signal),
  });
  const refresh = () => client.invalidateQueries({ queryKey: ["admin-groups"] });
  const fail = (cause: unknown) =>
    setError(cause instanceof ApiError ? cause.message : t("common.actionFailed"));
  const create = useMutation({
    mutationFn: () => createGroup({ bundle: emptyGroup(t("groups.newGroup")) }),
    onSuccess: (saved) => {
      client.setQueryData(["admin-group", saved.bundle.group.id], saved);
      void refresh();
      void navigate(`/admin/question-bank/groups/${saved.bundle.group.id}`);
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
  const items = list.data?.items ?? [];
  const selection = [...bulk.selected.values()];
  const selectedArchived = selection.every((item) => item.archivedAt !== null);
  const selectedActive = selection.every((item) => item.archivedAt === null);
  const ask = (next: Action) => {
    setError(null);
    setAction(next);
  };
  return (
    <div className="flex min-w-0 flex-col gap-4">
      <PageHeader
        variant="title"
        title={t("groups.bankTitle")}
        subtitle={t("groups.bankHint")}
        actions={
          <Button
            size="sm"
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
      <div className="flex flex-wrap gap-3">
        <SearchInput
          value={query}
          onChange={(value) => setFilter("q", value)}
          placeholder={t("groups.search")}
          className="min-w-48 flex-1"
        />
        <Select
          value={status}
          onValueChange={(value) =>
            setFilter("status", value === "active" ? null : value)
          }
        >
          <SelectTrigger className="w-auto min-w-40" aria-label={t("common.status")}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              <SelectItem value="active">{t("groups.active")}</SelectItem>
              <SelectItem value="archived">{t("common.archived")}</SelectItem>
            </SelectGroup>
          </SelectContent>
        </Select>
      </div>
      {error && !action ? (
        <p role="alert" className="text-sm">
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
                  run: (group: GroupSummary) => archiveGroup(group, true),
                },
              ]
            : []),
          ...(selectedArchived
            ? [
                {
                  label: t("common.restore"),
                  description: t("groups.restoreBody"),
                  run: (group: GroupSummary) => archiveGroup(group, false),
                },
                {
                  label: t("common.deletePermanently"),
                  description: t("groups.deleteBody"),
                  run: (group: GroupSummary) => deleteGroup(group.id, group.revision),
                },
              ]
            : []),
        ]}
      />
      <QueryStates
        query={list}
        skeleton={<ListSkeleton />}
        failed={t("groups.loadFailed")}
      >
        {(data) =>
          data.items.length === 0 ? (
            <EmptyState hint={t("groups.bankHint")}>{t("groups.empty")}</EmptyState>
          ) : (
            <>
              <div className="rounded-lg border shadow-sm">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-10">
                        <Checkbox
                          aria-label={t("common.selectPage")}
                          checked={
                            items.length > 0 &&
                            items.every((item) => bulk.selected.has(item.id))
                          }
                          onChange={(event) =>
                            bulk.selectPage(items, event.target.checked)
                          }
                        />
                      </TableHead>
                      <TableHead>{t("groups.titleLabel")}</TableHead>
                      <TableHead>{t("groups.questionCount")}</TableHead>
                      <TableHead>{t("bank.points")}</TableHead>
                      <TableHead>{t("common.updatedAt")}</TableHead>
                      <TableHead>
                        <span className="sr-only">{t("common.actions")}</span>
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {data.items.map((group) => (
                      <TableRow key={group.id}>
                        <TableCell>
                          <Checkbox
                            aria-label={t("groups.select", { title: group.title })}
                            checked={bulk.selected.has(group.id)}
                            onChange={() => bulk.toggle(group)}
                          />
                        </TableCell>
                        <TableCell>
                          <div className="flex flex-wrap items-center gap-2">
                            <Link
                              className="font-medium hover:underline"
                              to={`/admin/question-bank/groups/${group.id}`}
                            >
                              {group.title}
                            </Link>
                            {recent.has(group.id) ? (
                              <Badge variant="outline">
                                {t("common.justDuplicated")}
                              </Badge>
                            ) : null}
                          </div>
                        </TableCell>
                        <TableCell className="tabular-nums">
                          {group.questionCount}
                        </TableCell>
                        <TableCell className="tabular-nums">
                          {group.totalPoints}
                        </TableCell>
                        <TableCell className="text-muted-foreground">
                          {formatRelative(group.updatedAt, locale)}
                        </TableCell>
                        <TableCell>
                          <div className="flex justify-end gap-1">
                            <Button
                              variant="ghost"
                              size="icon-xs"
                              aria-label={t("common.duplicateNamed", {
                                name: group.title,
                              })}
                              disabled={
                                duplicate.isPending || group.archivedAt !== null
                              }
                              onClick={() => duplicate.mutate(group)}
                            >
                              <Copy aria-hidden="true" />
                            </Button>
                            <RowMenu>
                              <DropdownMenuItem
                                disabled={
                                  duplicate.isPending || group.archivedAt !== null
                                }
                                onSelect={() => duplicate.mutate(group)}
                              >
                                <Copy aria-hidden="true" />
                                {t("bank.duplicate")}
                              </DropdownMenuItem>
                              {group.archivedAt ? (
                                <>
                                  <DropdownMenuItem
                                    onSelect={() => ask({ kind: "restore", group })}
                                  >
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
                                <DropdownMenuItem
                                  onSelect={() => ask({ kind: "archive", group })}
                                >
                                  <Archive aria-hidden="true" />
                                  {t("common.archive")}
                                </DropdownMenuItem>
                              )}
                            </RowMenu>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
              <Pager page={data.page} pageSize={data.pageSize} total={data.total} />
            </>
          )
        }
      </QueryStates>
      <ConfirmDialog
        open={action !== null}
        onOpenChange={(open) => {
          if (!open && !mutate.isPending) setAction(null);
        }}
        title={t(actionLabels[action?.kind ?? "archive"].title)}
        description={t(actionLabels[action?.kind ?? "archive"].description)}
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

const actionLabels = {
  delete: { title: "common.deletePermanently", description: "groups.deleteBody" },
  restore: { title: "common.restore", description: "groups.restoreBody" },
  archive: { title: "common.archive", description: "groups.archiveBody" },
};
