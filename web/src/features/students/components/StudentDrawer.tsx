import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Ban, KeyRound, Pencil, UserCheck } from "lucide-react";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { DeleteItemButton } from "@/components/shared/DeleteItemButton";
import { Sheet } from "@/components/shared/Sheet";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { toast } from "@/components/ui/sonner";
import { useCan } from "@/features/auth/permissions";
import { removeMember } from "@/features/classes/api";
import { invalidateClassMembership } from "@/features/classes/invalidate";
import { EditStudentForm } from "@/features/students/components/EditStudentForm";
import { ResetPasswordFlow } from "@/features/students/components/ResetPasswordFlow";
import {
  deleteStudent,
  scorePercent,
  updateStudent,
  type Student,
} from "@/features/students/api";
import { failureMessage } from "@/lib/api/errors";
import { formatDate, useDisplayTimeZone } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import type { TFunction } from "i18next";

type Confirming =
  { kind: "disable" } | { kind: "remove"; classId: string; className: string } | null;

const SECTION = "text-muted-fg text-meta mb-2 font-medium";

/**
 * StudentDrawer is a student's record in the deck's 420px sheet, the "View
 * results" of the Students list until R9 brings the gradebook: what they have
 * submitted, their average and flags, their classes, how they sign in and
 * whether the account is open. Each control shows only with its permission:
 * editing the name and email with `people.students.create`, a temporary
 * password with `people.students.reset_password`, Remove from class with
 * `teaching.classes.write`, and suspending, restoring or deleting the account
 * with `people.users.manage`.
 */
export function StudentDrawer({
  student,
  onClose,
}: Readonly<{
  student: Student;
  onClose: () => void;
}>) {
  useDisplayTimeZone();
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const canEdit = useCan("people.students.create");
  const canReset = useCan("people.students.reset_password");
  const canMove = useCan("teaching.classes.write");
  const canManageUsers = useCan("people.users.manage");
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState(false);
  const [confirming, setConfirming] = useState<Confirming>(null);
  const [resetting, setResetting] = useState(false);
  const resetButton = useRef<HTMLButtonElement>(null);

  const invalidate = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: ["admin-students"] }),
      queryClient.invalidateQueries({ queryKey: ["admin-student", student.id] }),
    ]);

  const setDisabled = useMutation({
    mutationFn: (disabled: boolean) => updateStudent(student.id, { disabled }),
    onSuccess: async (_, disabled) => {
      setError(null);
      setConfirming(null);
      toast(t(disabled ? "students.disabled" : "students.enabled"));
      await invalidate();
      await queryClient.invalidateQueries({ queryKey: ["admin-classes"] });
      await queryClient.invalidateQueries({ queryKey: ["admin-assignments"] });
    },
    onError: (cause) => setError(failureMessage(cause, t("students.disableFailed"))),
  });

  const remove = useMutation({
    mutationFn: (classId: string) => removeMember(classId, student.id),
    onSuccess: async (_data, classId) => {
      setError(null);
      setConfirming(null);
      toast(t("students.removed"));
      await invalidateClassMembership(queryClient, classId);
      await invalidate();
    },
    onError: (cause) => setError(failureMessage(cause, t("students.removeFailed"))),
  });

  const percent = scorePercent(student.stats);
  const googleOnly = !student.hasPassword && student.linkedProviders.includes("google");
  const busy = remove.isPending || setDisabled.isPending;

  return (
    <Sheet
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={student.fullName}
      subtitle={student.email}
      leading={<Avatar size="lg" name={student.fullName} />}
    >
      {editing ? (
        <EditStudentForm student={student} onDone={() => setEditing(false)} />
      ) : (
        <div className="flex flex-wrap items-center gap-1.5">
          <IdentityBadges student={student} />
          {canEdit ? (
            <Button
              variant="ghost"
              size="xs"
              className="text-muted-fg ml-auto"
              onClick={() => setEditing(true)}
            >
              <Pencil aria-hidden="true" />
              {t("students.editInfo")}
            </Button>
          ) : null}
        </div>
      )}

      <div className="grid grid-cols-3 gap-3">
        <Tile
          label={t("students.submittedShort")}
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
        <p role="alert" className="text-danger-ink text-sm">
          {error}
        </p>
      )}

      <ClassesSection
        student={student}
        canMove={canMove}
        busy={busy}
        onRemove={(classId, className) =>
          setConfirming({ kind: "remove", classId, className })
        }
      />

      <Separator />

      <div>
        <p className={SECTION}>{t("students.signIn")}</p>
        <p className="text-muted-fg text-sm leading-relaxed">
          {t(googleOnly ? "students.googleOnlyHint" : "students.passwordHint")}
        </p>
        {canReset ? (
          <Button
            ref={resetButton}
            variant="outline"
            size="sm"
            className="mt-2.5"
            onClick={() => setResetting(true)}
          >
            <KeyRound aria-hidden="true" />
            {t("students.resetPassword")}
          </Button>
        ) : null}
      </div>

      {canManageUsers ? (
        <AccessSection
          student={student}
          busy={busy}
          onRestore={() => setDisabled.mutate(false)}
          onSuspend={() => setConfirming({ kind: "disable" })}
          onDeleted={async () => {
            onClose();
            await invalidate();
          }}
        />
      ) : null}

      <ConfirmDialog
        open={confirming !== null}
        onOpenChange={(open) => {
          if (!open && !busy) setConfirming(null);
        }}
        {...confirmCopy(confirming, student.fullName, t)}
        destructive
        pending={busy}
        onConfirm={() => {
          if (confirming?.kind === "remove") remove.mutate(confirming.classId);
          else setDisabled.mutate(true);
        }}
      />
      <ResetPasswordFlow
        students={resetting ? [student] : []}
        returnFocus={resetButton}
        onClose={() => setResetting(false)}
      />
    </Sheet>
  );
}

function ClassesSection({
  student,
  canMove,
  busy,
  onRemove,
}: Readonly<{
  student: Student;
  canMove: boolean;
  busy: boolean;
  onRemove: (classId: string, className: string) => void;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  return (
    <div>
      <p className={SECTION}>{t("students.classes")}</p>
      {student.classes.length === 0 ? (
        <p className="text-muted-fg text-sm">{t("students.noClasses")}</p>
      ) : (
        student.classes.map((klass) => (
          <div
            key={klass.id}
            className="flex items-center justify-between gap-2 py-1.5 text-sm"
          >
            <div className="min-w-0">
              <p className="break-words">{klass.name}</p>
              <p className="text-muted-fg text-xs">
                {t(`students.joinedVia.${klass.joinedVia}`)}
                {" · "}
                {formatDate(klass.joinedAt, locale)}
              </p>
            </div>
            {canMove ? (
              <Button
                variant="ghost"
                size="xs"
                className="text-muted-fg shrink-0"
                aria-disabled={busy || undefined}
                onClick={() => {
                  if (!busy) onRemove(klass.id, klass.name);
                }}
              >
                {t("students.removeFromClass")}
              </Button>
            ) : null}
          </div>
        ))
      )}
    </div>
  );
}

function AccessSection({
  student,
  busy,
  onRestore,
  onSuspend,
  onDeleted,
}: Readonly<{
  student: Student;
  busy: boolean;
  onRestore: () => void;
  onSuspend: () => void;
  onDeleted: () => Promise<unknown>;
}>) {
  const { t } = useTranslation();
  const suspended = student.disabledAt !== null;
  return (
    <>
      <Separator />
      <div>
        <p className={SECTION}>{t("students.access")}</p>
        <p className="text-muted-fg text-sm leading-relaxed">
          {t(suspended ? "students.disabledHint" : "students.enabledHint")}
        </p>
        <div className="mt-2.5 flex flex-wrap gap-2">
          <Button
            variant="outline"
            size="sm"
            aria-disabled={busy || undefined}
            onClick={() => {
              if (busy) return;
              if (suspended) onRestore();
              else onSuspend();
            }}
          >
            {suspended ? <UserCheck aria-hidden="true" /> : <Ban aria-hidden="true" />}
            {t(suspended ? "students.enable" : "students.disable")}
          </Button>
          {suspended ? (
            <DeleteItemButton
              name={student.fullName}
              onDelete={() => deleteStudent(student.id)}
              onDeleted={onDeleted}
            />
          ) : null}
        </div>
      </div>
    </>
  );
}

function Tile({ label, value }: Readonly<{ label: string; value: string }>) {
  return (
    <div className="rounded-lg border p-3">
      <p className="text-muted-fg text-xs">{label}</p>
      <p className="text-lg font-semibold tabular-nums">{value}</p>
    </div>
  );
}

function IdentityBadges({ student }: Readonly<{ student: Student }>) {
  const { t } = useTranslation();
  return (
    <>
      {student.classes.some((klass) => klass.joinedVia === "join_code") && (
        <Badge variant="outline">{t("students.joinedVia.join_code")}</Badge>
      )}
      {student.linkedProviders.includes("google") && (
        <Badge variant="outline">{t("students.google")}</Badge>
      )}
      {student.hasPassword && <Badge variant="outline">{t("students.password")}</Badge>}
      {student.disabledAt && (
        <Badge variant="outline" className="text-danger-ink">
          {t("students.disabledBadge")}
        </Badge>
      )}
    </>
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
