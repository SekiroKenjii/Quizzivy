import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import {
  EmptyState,
  ListSkeleton,
  LoadError,
  QueryStates,
} from "@/components/shared/ListState";
import { PageHeader } from "@/components/shared/PageHeader";
import { RowMenu } from "@/components/shared/RowMenu";
import { SearchInput } from "@/components/shared/SearchInput";
import { toast } from "@/components/ui/sonner";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { fetchClass, fetchMembers, removeMember } from "@/features/classes/api";
import { AddMemberDialog } from "@/features/classes/components/AddMemberDialog";
import { ClassAssignmentsCard } from "@/features/classes/components/ClassAssignmentsCard";
import { ClassSettingsCard } from "@/features/classes/components/ClassSettingsCard";
import { JoinCodePanel } from "@/features/classes/components/JoinCodePanel";
import { invalidateClassMembership } from "@/features/classes/invalidate";
import { scorePercent } from "@/features/students/api";
import { ApiError } from "@/lib/api/errors";
import { useLocale } from "@/lib/i18n/useLocale";
import { formatDate } from "@/lib/i18n/datetime";
import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { ClipboardList, UserMinus, UserPlus } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router";
import { Pager } from "@/components/shared/Pager";
import { usePage } from "@/hooks/usePage";
import { useDebounced } from "@/lib/useDebounced";

/** §6.4's class screen: the join code, and who is in the class. */
const MEMBERS_PAGE_SIZE = 20;

export default function ClassDetailPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { id = "" } = useParams();
  const queryClient = useQueryClient();
  const locale = useLocale();

  const klass = useQuery({
    queryKey: ["admin-class", id],
    queryFn: ({ signal }) => fetchClass(id, signal),
  });
  const [query, setQuery] = useState("");
  const search = useDebounced(query, 300).trim();
  const [page] = usePage(search);
  const members = useQuery({
    queryKey: ["admin-class-members", id, { q: search, page }],
    queryFn: ({ signal }) =>
      fetchMembers(
        id,
        { limit: MEMBERS_PAGE_SIZE, page, ...(search === "" ? {} : { q: search }) },
        signal,
      ),
    placeholderData: keepPreviousData,
  });

  const [adding, setAdding] = useState(false);
  const [removeError, setRemoveError] = useState<string | null>(null);
  const [confirmRemove, setConfirmRemove] = useState<{
    userId: string;
    name: string;
  } | null>(null);

  const remove = useMutation({
    mutationFn: (userId: string) => removeMember(id, userId),
    onSuccess: async () => {
      setRemoveError(null);
      setConfirmRemove(null);
      await invalidateClassMembership(queryClient, id);
      toast(t("classDetail.removed"));
    },
    onError: (cause) => {
      setConfirmRemove(null);
      setRemoveError(
        cause instanceof ApiError ? cause.message : t("classDetail.removeFailed"),
      );
    },
  });

  if (klass.isPending) {
    return <ListSkeleton rows={6} />;
  }
  if (klass.isError) {
    return (
      <LoadError error={klass.error} onRetry={() => void klass.refetch()}>
        {t("classDetail.loadFailed")}
      </LoadError>
    );
  }

  const items = members.data?.items ?? [];

  return (
    <>
      <PageHeader
        title={klass.data.name}
        backTo="/admin/classes"
        meta={
          <Badge>
            {t("classDetail.studentCount", { count: klass.data.studentCount })}
          </Badge>
        }
        actions={
          <>
            <Button variant="outline" size="sm" onClick={() => setAdding(true)}>
              <UserPlus aria-hidden="true" />
              {t("classDetail.addStudent")}
            </Button>
            <Button
              size="sm"
              onClick={() =>
                void navigate(`/admin/assignments/new?classId=${klass.data.id}`)
              }
            >
              <ClipboardList aria-hidden="true" />
              {t("classDetail.assignToClass")}
            </Button>
          </>
        }
      />

      <div className="grid gap-5 lg:grid-cols-3">
        <div className="space-y-5 lg:col-span-2">
          <Card asChild className="gap-0 py-0">
            <section aria-labelledby="members-heading">
              <div className="flex items-center justify-between gap-3 px-5 pt-4 pb-3">
                <h2
                  id="members-heading"
                  className="text-[0.9375rem] font-semibold tracking-[-0.01em]"
                >
                  {t("classDetail.members", { count: klass.data.studentCount })}
                </h2>
                <SearchInput
                  className="w-56"
                  value={query}
                  onChange={setQuery}
                  placeholder={t("classDetail.searchMembers")}
                />
              </div>

              <QueryStates
                query={members}
                skeleton={<ListSkeleton rows={4} />}
                failed={t("classDetail.membersFailed")}
                className="px-5 pb-5"
              >
                {() =>
                  items.length === 0 ? (
                    <div className="px-5 pb-5">
                      <EmptyState
                        action={
                          search === "" ? (
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => setAdding(true)}
                            >
                              <UserPlus aria-hidden="true" />
                              {t("classDetail.addStudent")}
                            </Button>
                          ) : undefined
                        }
                      >
                        {search === ""
                          ? t("classDetail.noMembers")
                          : t("classDetail.noMemberMatches", { query: search })}
                      </EmptyState>
                    </div>
                  ) : (
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>{t("classDetail.name")}</TableHead>
                          <TableHead>{t("classDetail.joinedVia")}</TableHead>
                          <TableHead>{t("classDetail.joinedAt")}</TableHead>
                          <TableHead className="text-right">
                            {t("students.submitted")}
                          </TableHead>
                          <TableHead className="text-right">
                            {t("students.average")}
                          </TableHead>
                          <TableHead className="w-10">
                            <span className="sr-only">{t("classDetail.actions")}</span>
                          </TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {items.map((m) => (
                          <TableRow key={m.userId}>
                            <TableCell>
                              <div className="flex items-center gap-2">
                                <Avatar name={m.fullName} size="sm" />
                                <div className="min-w-0">
                                  <p className="truncate font-medium">{m.fullName}</p>
                                  <p className="text-muted-foreground truncate text-xs">
                                    {m.email}
                                  </p>
                                </div>
                              </div>
                            </TableCell>
                            <TableCell>
                              {m.joinedVia === "admin" ? (
                                <Badge>{t("classDetail.viaAdmin")}</Badge>
                              ) : (
                                <Badge variant="secondary">
                                  {t("classDetail.viaCode", {
                                    hint: m.joinCodeHint ?? "",
                                  })}
                                </Badge>
                              )}
                            </TableCell>
                            <TableCell className="text-muted-foreground">
                              {formatDate(m.joinedAt, locale)}
                            </TableCell>
                            <TableCell className="text-right tabular-nums">
                              {m.stats.submittedCount}
                            </TableCell>
                            <TableCell className="text-right tabular-nums">
                              {scorePercent(m.stats) === null ? (
                                <span className="text-muted-foreground">—</span>
                              ) : (
                                t("students.percent", { value: scorePercent(m.stats) })
                              )}
                            </TableCell>
                            <TableCell className="text-right">
                              <RowMenu>
                                <DropdownMenuItem
                                  variant="destructive"
                                  disabled={remove.isPending}
                                  onSelect={() => {
                                    setRemoveError(null);
                                    setConfirmRemove({
                                      userId: m.userId,
                                      name: m.fullName,
                                    });
                                  }}
                                >
                                  <UserMinus aria-hidden="true" />
                                  {t("classDetail.remove")}
                                </DropdownMenuItem>
                              </RowMenu>
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  )
                }
              </QueryStates>
              {members.data && members.data.total > MEMBERS_PAGE_SIZE && (
                <div className="px-5 pb-4">
                  <Pager
                    page={members.data.page}
                    pageSize={members.data.pageSize}
                    total={members.data.total}
                  />
                </div>
              )}
              {removeError ? (
                <p role="alert" className="text-destructive px-5 pb-2 text-sm">
                  {removeError}
                </p>
              ) : null}
              <p className="text-muted-foreground border-t px-5 pt-2 pb-2 text-xs">
                {t("classDetail.removeKeepsWork")}
              </p>
            </section>
          </Card>
          <ClassAssignmentsCard classId={id} />
        </div>

        <div className="space-y-5">
          <JoinCodePanel klass={klass.data} />
          <ClassSettingsCard klass={klass.data} />
        </div>
      </div>

      <ConfirmDialog
        open={confirmRemove !== null}
        onOpenChange={(open) => !open && setConfirmRemove(null)}
        title={t("classDetail.removeConfirmTitle")}
        description={`${confirmRemove?.name ?? ""} — ${t("classDetail.removeConfirmBody")}`}
        confirmLabel={t("classDetail.removeConfirm")}
        destructive
        pending={remove.isPending}
        onConfirm={() => confirmRemove && remove.mutate(confirmRemove.userId)}
      />

      <AddMemberDialog classId={id} open={adding} onOpenChange={setAdding} />
    </>
  );
}
