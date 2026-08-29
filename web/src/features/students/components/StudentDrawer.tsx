import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { KeyRound, X } from "lucide-react";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { TemporaryPasswordCard } from "@/features/students/components/TemporaryPasswordCard";
import {
  resetStudentPassword,
  scorePercent,
  type Student,
} from "@/features/students/api";
import { removeMember } from "@/features/classes/api";
import { invalidateClassMembership } from "@/features/classes/invalidate";
import { SUPPORTED_LOCALES, type Locale } from "@/lib/i18n";
import { formatDate } from "@/lib/i18n/datetime";
import { ApiError } from "@/lib/api/errors";

/**
 * G-07's detail panel.
 *
 * An inline aside, not a modal sheet: the deck's markup has no overlay and
 * keeps the selected row visible and highlighted, and its callout says why —
 * "the teacher's task is nearly always 'look something up about this student
 * while doing something else'". A modal takes the table away, which is the one
 * thing the panel exists not to do.
 */
export function StudentDrawer({
  student,
  onClose,
}: {
  student: Student;
  onClose: () => void;
}) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const [temporary, setTemporary] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const locale = currentLocale(i18n.language);

  // Escape closes it. This is the one thing a modal would have given for free,
  // and it is cheaper than the overlay and scroll lock that come with one.
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  const reset = useMutation({
    mutationFn: () => resetStudentPassword(student.id),
    onSuccess: async (result) => {
      setError(null);
      setTemporary(result.temporaryPassword);
      await queryClient.invalidateQueries({ queryKey: ["admin-students"] });
    },
    onError: (cause) =>
      setError(cause instanceof ApiError ? cause.message : t("students.resetFailed")),
  });

  const remove = useMutation({
    mutationFn: (classId: string) => removeMember(classId, student.id),
    onSuccess: async (_data, classId) => {
      setError(null);
      await invalidateClassMembership(queryClient, classId);
      await queryClient.invalidateQueries({ queryKey: ["admin-students"] });
    },
    onError: (cause) =>
      setError(cause instanceof ApiError ? cause.message : t("students.removeFailed")),
  });

  const percent = scorePercent(student.stats);
  const googleOnly = !student.hasPassword && student.linkedProviders.includes("google");

  return (
    <aside
      aria-label={t("students.detailFor", { name: student.fullName })}
      className="w-96 shrink-0 space-y-5 border-l p-5"
    >
      <div className="flex items-start gap-3">
        <Avatar size="lg" name={student.fullName} />
        <div className="min-w-0 flex-1">
          <p className="truncate text-base font-semibold">{student.fullName}</p>
          <p className="text-muted-foreground truncate text-xs">{student.email}</p>
          <div className="mt-2 flex flex-wrap items-center gap-1.5">
            {student.linkedProviders.includes("google") ? (
              <Badge variant="outline">{t("students.google")}</Badge>
            ) : null}
            {student.hasPassword ? (
              <Badge variant="outline">{t("students.password")}</Badge>
            ) : null}
          </div>
        </div>
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
                onClick={() => remove.mutate(klass.id)}
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
    </aside>
  );
}

function Tile({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border p-3">
      <p className="text-muted-foreground text-xs">{label}</p>
      <p className="text-lg font-semibold tabular-nums">{value}</p>
    </div>
  );
}

function currentLocale(language: string): Locale {
  return (SUPPORTED_LOCALES as readonly string[]).includes(language)
    ? (language as Locale)
    : "vi";
}
