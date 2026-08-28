import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useParams } from "react-router";
import { Button } from "@/components/ui/button";
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
import { JoinCodePanel } from "@/features/classes/components/JoinCodePanel";
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

  const [removeError, setRemoveError] = useState<string | null>(null);
  // Removing a student revokes their access immediately and is not undoable --
  // the same two properties that earned rotation its dialog (§6.4). One of the
  // three destructive actions on this screen having a confirmation and the
  // others not was an inconsistency, not a design.
  const [confirmRemove, setConfirmRemove] = useState<{
    userId: string;
    name: string;
  } | null>(null);

  const remove = useMutation({
    mutationFn: (userId: string) => removeMember(id, userId),
    onSuccess: async () => {
      setRemoveError(null);
      setConfirmRemove(null);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["admin-class-members", id] }),
        queryClient.invalidateQueries({ queryKey: ["admin-class", id] }),
      ]);
    },
    // Without this a failed removal is pixel-identical to the screen before the
    // click: no message, and the row stays because nothing was invalidated. The
    // teacher concludes it worked and the student keeps their access.
    onError: (cause) =>
      setRemoveError(
        cause instanceof ApiError ? cause.message : t("classDetail.removeFailed"),
      ),
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

  const items = members.data?.items ?? [];

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">{klass.data.name}</h1>
        {klass.data.description ? (
          <p className="text-muted-foreground mt-1 text-sm">{klass.data.description}</p>
        ) : null}
      </div>

      <JoinCodePanel klass={klass.data} />

      <section className="rounded-lg border" aria-labelledby="members-heading">
        <div className="border-b px-6 py-4">
          <h2 id="members-heading" className="text-base font-semibold">
            {t("classDetail.members", { count: klass.data.studentCount })}
          </h2>
        </div>

        {members.isError ? (
          // NOT the empty state. Reusing it here would put "no students yet"
          // directly under a heading that says "Students (12)", and a teacher
          // reads that as an enrolment problem rather than a failed request.
          <p role="alert" className="text-destructive px-6 py-8 text-sm">
            {t("classDetail.membersFailed")}
          </p>
        ) : members.isPending ? (
          <p
            className="text-muted-foreground px-6 py-8 text-sm"
            role="status"
            aria-live="polite"
          >
            {t("common.loading")}
          </p>
        ) : items.length === 0 ? (
          // §12: one short sentence and one action, no illustration.
          <p className="text-muted-foreground px-6 py-8 text-sm">
            {t("classDetail.noMembers")}
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("classDetail.name")}</TableHead>
                <TableHead>{t("classDetail.joinedVia")}</TableHead>
                <TableHead>{t("classDetail.joinedAt")}</TableHead>
                <TableHead className="sr-only">{t("classDetail.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((m) => (
                <TableRow key={m.userId}>
                  <TableCell>
                    <span className="font-medium">{m.fullName}</span>
                    <span className="text-muted-foreground block text-xs">
                      {m.email}
                    </span>
                  </TableCell>
                  <TableCell className="text-sm">
                    {m.joinedVia === "admin"
                      ? t("classDetail.viaAdmin")
                      : t("classDetail.viaCode", { hint: m.joinCodeHint ?? "" })}
                  </TableCell>
                  <TableCell className="text-sm">
                    {formatDate(m.joinedAt, locale)}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={remove.isPending}
                      onClick={() =>
                        setConfirmRemove({ userId: m.userId, name: m.fullName })
                      }
                    >
                      {t("classDetail.remove")}
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
        {removeError ? (
          <p role="alert" className="text-destructive border-t px-6 py-3 text-sm">
            {removeError}
          </p>
        ) : null}
        <p className="text-muted-foreground border-t px-6 py-3 text-xs">
          {t("classDetail.removeKeepsWork")}
        </p>
      </section>

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
    </div>
  );
}
