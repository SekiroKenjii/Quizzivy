import { useState, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";
import { KeyRound } from "lucide-react";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import {
  resetStudentPassword,
  resetStudentsPasswords,
  type Student,
} from "@/features/students/api";
import type { OneTimeCredential, ResetFailure } from "@/features/students/credentials";
import { ResetSheet } from "@/features/students/components/ResetSheet";
import { ApiError, failureMessage } from "@/lib/api/errors";

/** BULK_RESET_LIMIT is how many students one bulk reset may name. */
export const BULK_RESET_LIMIT = 40;

type Outcome = Readonly<{
  credentials: readonly OneTimeCredential[];
  failures: readonly ResetFailure[];
}>;

async function resetOne(student: Student): Promise<Outcome> {
  const { temporaryPassword } = await resetStudentPassword(student.id);
  return {
    credentials: [
      {
        studentId: student.id,
        fullName: student.fullName,
        email: student.email,
        temporaryPassword,
      },
    ],
    failures: [],
  };
}

async function resetMany(students: readonly Student[]): Promise<Outcome> {
  const names = new Map(students.map((student) => [student.id, student.fullName]));
  const { items, failed } = await resetStudentsPasswords(students.map((s) => s.id));
  return {
    credentials: items,
    failures: failed.map((failure) => ({
      studentId: failure.studentId,
      fullName: names.get(failure.studentId) ?? "",
      code: failure.code,
    })),
  };
}

/**
 * ResetPasswordFlow resets the passwords of `students`, one or up to 40: a
 * confirmation that says what happens, then the ResetSheet with each new
 * temporary password and each student that was not reset. One student goes
 * through `resetStudentPassword`, several through the bulk operation. The
 * passwords are held only in this component's state, and dropped when the
 * sheet closes; a refusal, such as a student another teacher also reaches,
 * is shown in the confirmation.
 */
export function ResetPasswordFlow({
  students,
  returnFocus,
  onClose,
}: Readonly<{
  students: readonly Student[];
  returnFocus?: RefObject<HTMLElement | null> | undefined;
  onClose: () => void;
}>) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const one = students.length === 1 ? students[0] : undefined;

  async function reset() {
    setPending(true);
    setError(null);
    try {
      setOutcome(one === undefined ? await resetMany(students) : await resetOne(one));
      void client.invalidateQueries({ queryKey: ["admin-students"] });
    } catch (cause) {
      setError(
        cause instanceof ApiError && cause.code === "STUDENT_SHARED"
          ? t("students.reset.reason.shared")
          : failureMessage(cause, t("students.resetFailed")),
      );
    } finally {
      setPending(false);
    }
  }

  function close() {
    setOutcome(null);
    setError(null);
    onClose();
  }

  return (
    <>
      <ConfirmDialog
        open={students.length > 0 && outcome === null}
        onOpenChange={(open) => {
          if (!open && !pending) close();
        }}
        icon={KeyRound}
        title={
          one === undefined
            ? t("students.reset.confirmMany", { count: students.length })
            : t("students.reset.confirmOne", { name: one.fullName })
        }
        description={t(
          one === undefined ? "students.reset.bodyMany" : "students.reset.bodyOne",
        )}
        confirmLabel={
          one === undefined
            ? t("students.reset.submitMany", { count: students.length })
            : t("students.reset.submitOne")
        }
        pending={pending}
        error={error}
        {...(returnFocus === undefined ? {} : { returnFocus })}
        onConfirm={() => void reset()}
      />
      <ResetSheet
        open={outcome !== null}
        title={
          one === undefined
            ? t("students.reset.doneMany", { count: outcome?.credentials.length ?? 0 })
            : t("students.reset.doneOne", { name: one.fullName })
        }
        credentials={outcome?.credentials ?? []}
        failures={outcome?.failures ?? []}
        returnFocus={returnFocus}
        onDone={close}
      />
    </>
  );
}
