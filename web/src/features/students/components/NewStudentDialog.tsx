import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";
import { UserPlus } from "lucide-react";
import { FormDialog, type FormField } from "@/components/shared/form/FormDialog";
import type { Class } from "@/features/classes/api";
import { createStudent } from "@/features/students/api";
import type { OneTimeCredential } from "@/features/students/credentials";
import { ApiError, fieldMessages } from "@/lib/api/errors";

type NewStudentValues = { fullName: string; email: string; classId: string };

const INITIAL: NewStudentValues = { fullName: "", email: "", classId: "" };

/**
 * NewStudentDialog is the deck's "Add student": Full name, Email and Class,
 * one of the caller's classes or "No class yet". The account is created with
 * a temporary password, which goes to `onCreated` and nowhere else: never a
 * toast, a cache or the URL. The dialog closes when the account exists, and
 * the caller shows the password.
 */
export function NewStudentDialog({
  open,
  classes,
  onOpenChange,
  onCreated,
}: Readonly<{
  open: boolean;
  classes: readonly Class[];
  onOpenChange: (open: boolean) => void;
  onCreated: (credential: OneTimeCredential) => void;
}>) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const fields: readonly FormField<NewStudentValues>[] = [
    {
      kind: "text",
      name: "fullName",
      label: t("students.fullName"),
      required: true,
      placeholder: t("students.newNamePlaceholder"),
      autoComplete: "off",
      maxLength: 200,
    },
    {
      kind: "email",
      name: "email",
      label: t("students.email"),
      required: true,
      autoComplete: "off",
      maxLength: 254,
    },
    {
      kind: "select",
      name: "classId",
      label: t("students.class"),
      options: [
        { value: "", label: t("students.noClassYet") },
        ...classes.map((klass) => ({ value: klass.id, label: klass.name })),
      ],
    },
  ];

  async function create({ fullName, email, classId }: NewStudentValues) {
    setPending(true);
    setError(null);
    try {
      const created = await createStudent({
        fullName: fullName.trim(),
        email: email.trim(),
        ...(classId === "" ? {} : { classIds: [classId] }),
      });
      onOpenChange(false);
      onCreated({
        studentId: created.user.id,
        fullName: created.user.fullName,
        email: created.user.email,
        temporaryPassword: created.temporaryPassword,
      });
      await Promise.all([
        client.invalidateQueries({ queryKey: ["teacher-students"] }),
        client.invalidateQueries({ queryKey: ["teacher-classes"] }),
      ]);
    } catch (cause) {
      setError(
        fieldMessages(cause)[0] ??
          (cause instanceof ApiError ? cause.message : t("students.createFailed")),
      );
    } finally {
      setPending(false);
    }
  }

  return (
    <FormDialog<NewStudentValues>
      open={open}
      onOpenChange={(next) => {
        if (pending) return;
        if (!next) setError(null);
        onOpenChange(next);
      }}
      title={t("students.new")}
      description={t("students.newHint")}
      icon={UserPlus}
      initial={INITIAL}
      fields={fields}
      submitLabel={t("students.create")}
      pending={pending}
      error={error}
      onSubmit={(values) => void create(values)}
    />
  );
}
