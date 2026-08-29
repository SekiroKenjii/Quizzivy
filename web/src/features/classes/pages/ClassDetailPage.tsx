import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router";
import { ClipboardList, Search, UserPlus } from "lucide-react";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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
import { ClassSettingsCard } from "@/features/classes/components/ClassSettingsCard";
import { invalidateClassMembership } from "@/features/classes/invalidate";
import { JoinCodePanel } from "@/features/classes/components/JoinCodePanel";
import { PageHeader } from "@/components/shared/PageHeader";
import { SUPPORTED_LOCALES, type Locale } from "@/lib/i18n";
import { formatDate } from "@/lib/i18n/datetime";
import { ApiError } from "@/lib/api/errors";

function currentLocale(language: string): Locale {
  return (SUPPORTED_LOCALES as readonly string[]).includes(language)
    ? (language as Locale)
    : "vi";
}

/**
 * §6.4's class screen: the join code, and who is in the class.
 *
 * The member list is not decoration. §17.2 declines to build an approval queue,
 * and `joinedVia` plus the code hint are what it chose INSTEAD -- they are how a
 * teacher notices someone they did not expect, and after a rotation they say
 * whether that person came in through the code that leaked or the current one
 * (D-10).
 */
export default function ClassDetailPage() {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const { id = "" } = useParams();
  const queryClient = useQueryClient();
  const locale = currentLocale(i18n.language);

  const klass = useQuery({
    queryKey: ["admin-class", id],
    queryFn: ({ signal }) => fetchClass(id, signal),
  });
  const members = useQuery({
    queryKey: ["admin-class-members", id],
    queryFn: ({ signal }) => fetchMembers(id, signal),
  });

  const [query, setQuery] = useState("");
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
    },
    onError: (cause) => {
      setConfirmRemove(null);
      setRemoveError(
        cause instanceof ApiError ? cause.message : t("classDetail.removeFailed"),
      );
    },
  });

  if (klass.isPending) {
    return (
      <p className="text-muted-foreground text-sm" role="status" aria-live="polite">
        {t("common.loading")}
      </p>
    );
  }
  if (klass.isError || !klass.data) {
    return (
      <p role="alert" className="text-destructive text-sm">
        {t("classDetail.loadFailed")}
      </p>
    );
  }

  const items = matching(members.data?.items ?? [], query);

  const memberIds = new Set((members.data?.items ?? []).map((m) => m.userId));

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
                <div className="relative w-56">
                  <Search
                    className="text-muted-foreground pointer-events-none absolute top-2.5 left-2.5 size-3.5"
                    aria-hidden="true"
                  />
                  <Input
                    className="h-8 pl-8 text-xs"
                    placeholder={t("classDetail.searchMembers")}
                    aria-label={t("classDetail.searchMembers")}
                    value={query}
                    onChange={(event) => setQuery(event.target.value)}
                  />
                </div>
              </div>

              {members.isError ? (
                <p role="alert" className="text-destructive px-5 pb-8 text-sm">
                  {t("classDetail.membersFailed")}
                </p>
              ) : members.isPending ? (
                <p
                  className="text-muted-foreground px-5 pb-8 text-sm"
                  role="status"
                  aria-live="polite"
                >
                  {t("common.loading")}
                </p>
              ) : items.length === 0 ? (
                // §12: one short sentence, no illustration. Which sentence
                // matters: `items` is search-filtered, so an empty result after
                // typing means "nobody matches", not "the class is empty".
                <p className="text-muted-foreground px-5 pb-8 text-sm">
                  {members.data.items.length === 0
                    ? t("classDetail.noMembers")
                    : t("classDetail.noMemberMatches", { query: query.trim() })}
                </p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("classDetail.name")}</TableHead>
                      <TableHead>{t("classDetail.joinedVia")}</TableHead>
                      <TableHead>{t("classDetail.joinedAt")}</TableHead>
                      <TableHead className="sr-only">
                        {t("classDetail.actions")}
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
                              {t("classDetail.viaCode", { hint: m.joinCodeHint ?? "" })}
                            </Badge>
                          )}
                        </TableCell>
                        <TableCell className="text-muted-foreground">
                          {formatDate(m.joinedAt, locale)}
                        </TableCell>
                        <TableCell className="text-right">
                          <Button
                            variant="ghost"
                            size="sm"
                            disabled={remove.isPending}
                            onClick={() => {
                              setRemoveError(null);
                              setConfirmRemove({ userId: m.userId, name: m.fullName });
                            }}
                          >
                            <span aria-hidden="true">{t("classDetail.remove")}</span>
                            <span className="sr-only">
                              {t("classDetail.removeNamed", { name: m.fullName })}
                            </span>
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
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
        </div>

        <div className="space-y-5">
          <JoinCodePanel klass={klass.data} />
          <ClassSettingsCard klass={klass.data} />
        </div>
      </div>

      <Dialog
        open={confirmRemove !== null}
        onOpenChange={(open) => !open && setConfirmRemove(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("classDetail.removeConfirmTitle")}</DialogTitle>
            <DialogDescription>
              {confirmRemove ? `${confirmRemove.name} — ` : ""}
              {t("classDetail.removeConfirmBody")}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmRemove(null)}>
              {t("common.cancel")}
            </Button>
            <Button
              disabled={remove.isPending}
              onClick={() => confirmRemove && remove.mutate(confirmRemove.userId)}
            >
              {remove.isPending ? t("common.loading") : t("classDetail.removeConfirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AddMemberDialog
        classId={id}
        memberIds={memberIds}
        open={adding}
        onOpenChange={setAdding}
      />
    </>
  );
}

// The members endpoint has no query parameter and a class is ~50 people, so the
// deck's search box filters what is already loaded rather than adding a round
// trip per keystroke.
function matching<T extends { fullName: string; email: string }>(
  members: T[],
  query: string,
): T[] {
  const needle = query.trim().toLocaleLowerCase("vi");
  if (needle === "") return members;
  return members.filter(
    (member) =>
      member.fullName.toLocaleLowerCase("vi").includes(needle) ||
      member.email.toLowerCase().includes(needle),
  );
}
