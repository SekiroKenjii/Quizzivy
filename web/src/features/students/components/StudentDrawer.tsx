import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Ban, KeyRound, Pencil, UserCheck, X } from "lucide-react";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { PageAside } from "@/components/shared/PageAside";
import { toast } from "@/components/ui/sonner";
import { EditStudentForm } from "@/features/students/components/EditStudentForm";
import { TemporaryPasswordCard } from "@/features/students/components/TemporaryPasswordCard";
import {
  resetStudentPassword,
  scorePercent,
  updateStudent,
  type Student,
} from "@/features/students/api";
import { removeMember } from "@/features/classes/api";
import { invalidateClassMembership } from "@/features/classes/invalidate";
import { useLocale } from "@/lib/i18n/useLocale";
import { formatDate } from "@/lib/i18n/datetime";
import { failureMessage } from "@/lib/api/errors";
import type { TFunction } from "i18next";

type Confirming =
  { kind: "disable" } | { kind: "remove"; classId: string; className: string } | null;

/** G-07's detail panel. */
export function StudentDrawer({
  student,
  onClose,
}: Readonly<{
  student: Student;
  onClose: () => void;
}>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [temporary, setTemporary] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const locale = useLocale();

  // Escape closes it, unless a layer above already answered for it.
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape" && !event.defaultPrevented) onClose();
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  const [editing, setEditing] = useState(false);
  const [confirming, setConfirming] = useState<Confirming>(null);
  const setDisabled = useMutation({
    mutationFn: (disabled: boolean) => updateStudent(student.id, { disabled }),
    onSuccess: async (_, disabled) => {
      setError(null);
      setConfirming(null);
      toast(t(disabled ? "students.disabled" : "students.enabled"));
      await queryClient.invalidateQueries({ queryKey: ["admin-students"] });
      await queryClient.invalidateQueries({ queryKey: ["admin-student", student.id] });
      await queryClient.invalidateQueries({ queryKey: ["admin-classes"] });
      await queryClient.invalidateQueries({ queryKey: ["admin-assignments"] });
    },
    onError: (cause) => setError(failureMessage(cause, t("students.disableFailed"))),
  });

  const reset = useMutation({
    mutationFn: () => resetStudentPassword(student.id),
    onSuccess: async (result) => {
      setError(null);
      setTemporary(result.temporaryPassword);
      await queryClient.invalidateQueries({ queryKey: ["admin-students"] });
    },
    onError: (cause) => setError(failureMessage(cause, t("students.resetFailed"))),
  });

  const remove = useMutation({
    mutationFn: (classId: string) => removeMember(classId, student.id),
    onSuccess: async (_data, classId) => {
      setError(null);
      setConfirming(null);
      toast(t("students.removed"));
      await invalidateClassMembership(queryClient, classId);
      await queryClient.invalidateQueries({ queryKey: ["admin-students"] });
    },
    onError: (cause) => setError(failureMessage(cause, t("students.removeFailed"))),
  });

  const percent = scorePercent(student.stats);
  const googleOnly = !student.hasPassword && student.linkedProviders.includes("google");

  return (
    <PageAside label={t("students.detailFor", { name: student.fullName })}>
      <div className="flex items-start gap-3">
        <Avatar size="lg" name={student.fullName} />
        {editing ? (
          <EditStudentForm student={student} onDone={() => setEditing(false)} />
        ) : (
          <div className="min-w-0 flex-1">
            <p className="flex items-center gap-1.5 truncate text-base font-semibold">
              {student.fullName}
              <Button
                variant="ghost"
                size="icon-xs"
                className="text-muted-foreground"
                aria-label={t("students.editInfo")}
                onClick={() => setEditing(true)}
              >
                <Pencil aria-hidden="true" />
              </Button>
            </p>
            <p className="text-muted-foreground truncate text-xs">{student.email}</p>
            <IdentityBadges student={student} />
          </div>
        )}
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={t("students.closeDetail")}
          onClick={onClose}
        >
          <X aria-hidden="true" />
        </Button>
      </div>

      <div className="grid grid-cols-3 gap-3">
        <Tile
          label={t("students.submitted")}
          value={String(student.stats.submittedCount)}
        />
        <Tile
          label={t("students.average")}
          value={percent === null ? "—" : t("students.percent", { value: percent })}
        />
        <Tile
          label={t("students.flagged")}
          value={String(student.stats.flaggedCount)}
        />
      </div>

      {error === null ? null : (
        <p role="alert" className="text-destructive text-sm">
          {error}
        </p>
      )}

      <div>
        <p className="text-muted-foreground mb-2 text-xs font-medium tracking-wide uppercase">
          {t("students.classes")}
        </p>
        {student.classes.length === 0 ? (
          <p className="text-muted-foreground text-sm">{t("students.noClasses")}</p>
        ) : (
          student.classes.map((klass) => (
            <div
              key={klass.id}
              className="flex items-center justify-between gap-2 py-1.5 text-sm"
            >
              <div className="min-w-0">
                <p className="truncate">{klass.name}</p>
                <p className="text-muted-foreground text-xs">
                  {t(`students.joinedVia.${klass.joinedVia}`)}
                  {" · "}
                  {formatDate(klass.joinedAt, locale)}
                </p>
              </div>
              <Button
                variant="ghost"
                size="xs"
                className="text-muted-foreground shrink-0"
                disabled={remove.isPending}
                onClick={() =>
                  setConfirming({
                    kind: "remove",
                    classId: klass.id,
                    className: klass.name,
                  })
                }
              >
                {t("students.removeFromClass")}
              </Button>
            </div>
          ))
        )}
      </div>

      <Separator />

      <div>
        <p className="text-muted-foreground mb-2 text-xs font-medium tracking-wide uppercase">
          {t("students.signIn")}
        </p>
        <p className="text-muted-foreground text-sm leading-relaxed">
          {googleOnly ? t("students.googleOnlyHint") : t("students.passwordHint")}
        </p>
        <Button
          variant="outline"
          size="sm"
          className="mt-2.5"
          disabled={reset.isPending}
          onClick={() => reset.mutate()}
        >
          <KeyRound aria-hidden="true" />
          {reset.isPending ? t("common.loading") : t("students.resetPassword")}
        </Button>
      </div>

      {temporary === null ? null : <TemporaryPasswordCard password={temporary} />}

      <Separator />

      <div>
        <p className="text-muted-foreground mb-2 text-xs font-medium tracking-wide uppercase">
          {t("students.access")}
        </p>
        <p className="text-muted-foreground text-sm leading-relaxed">
          {student.disabledAt === null
            ? t("students.enabledHint")
            : t("students.disabledHint")}
        </p>
        <Button
          variant="outline"
          size="sm"
          className="mt-2.5"
          disabled={setDisabled.isPending}
          onClick={() =>
            student.disabledAt
              ? setDisabled.mutate(false)
              : setConfirming({ kind: "disable" })
          }
        >
          {student.disabledAt ? (
            <UserCheck aria-hidden="true" />
          ) : (
            <Ban aria-hidden="true" />
          )}
          {student.disabledAt ? t("students.enable") : t("students.disable")}
        </Button>
      </div>

      <ConfirmDialog
        open={confirming !== null}
        onOpenChange={(open) => !open && setConfirming(null)}
        {...confirmCopy(confirming, student.fullName, t)}
        destructive
        pending={remove.isPending || setDisabled.isPending}
        onConfirm={() => {
          if (confirming?.kind === "remove") remove.mutate(confirming.classId);
          else setDisabled.mutate(true);
        }}
      />
    </PageAside>
  );
}

function Tile({ label, value }: Readonly<{ label: string; value: string }>) {
  return (
    <div className="rounded-lg border p-3">
      <p className="text-muted-foreground text-xs">{label}</p>
      <p className="text-lg font-semibold tabular-nums">{value}</p>
    </div>
  );
}

function IdentityBadges({ student }: Readonly<{ student: Student }>) {
  const { t } = useTranslation();
  return (
    <div className="mt-2 flex flex-wrap items-center gap-1.5">
      {student.linkedProviders.includes("google") && (
        <Badge variant="outline">{t("students.google")}</Badge>
      )}
      {student.hasPassword && <Badge variant="outline">{t("students.password")}</Badge>}
      {student.disabledAt && (
        <Badge variant="outline" className="text-destructive-ink">
          {t("students.disabledBadge")}
        </Badge>
      )}
    </div>
  );
}

function confirmCopy(
  confirming: Confirming,
  name: string,
  t: TFunction,
): { title: string; description: string; confirmLabel: string } {
  if (confirming?.kind === "remove") {
    return {
      title: t("students.removeConfirmTitle", { name, klass: confirming.className }),
      description: t("students.removeConfirmBody"),
      confirmLabel: t("students.removeFromClass"),
    };
  }
  return {
    title: t("students.disableConfirmTitle", { name }),
    description: t("students.disableConfirmBody"),
    confirmLabel: t("students.disable"),
  };
}
