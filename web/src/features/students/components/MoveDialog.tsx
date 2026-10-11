import { useState, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";
import { ArrowRightLeft } from "lucide-react";
import { FormDialog, type FormField } from "@/components/shared/form/FormDialog";
import { toast } from "@/components/ui/sonner";
import type { Class } from "@/features/classes/api";
import { invalidateClassMembership } from "@/features/classes/invalidate";
import type { Student } from "@/features/students/api";
import { classesLeftBy, moveStudent } from "@/features/students/membership";

type MoveValues = { to: string; keep: boolean };

const INITIAL: MoveValues = { to: "", keep: false };

/**
 * MoveDialog moves one student or several to one of the caller's classes:
 * Class, and "Keep them in their current class too", which is off. Each
 * student is added to the class, then taken out of their other classes unless
 * kept; unless kept, the dialog names the classes they leave, which for an
 * Admin can be any class in the system. A student who could not be moved is
 * named in the dialog, which stays open; when every one moved it closes with
 * a toast.
 */
export function MoveDialog({
  students,
  classes,
  returnFocus,
  onOpenChange,
  onMoved,
}: Readonly<{
  students: readonly Student[];
  classes: readonly Class[];
  returnFocus?: RefObject<HTMLElement | null> | undefined;
  onOpenChange: (open: boolean) => void;
  onMoved: () => void;
}>) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const one = students.length === 1 ? students[0] : undefined;
  const fields: readonly FormField<MoveValues>[] = [
    {
      kind: "select",
      name: "to",
      label: t("students.move.class"),
      required: true,
      placeholder: t("students.move.pick"),
      options: classes.map((klass) => ({ value: klass.id, label: klass.name })),
    },
    {
      kind: "toggle",
      name: "keep",
      text: t(one === undefined ? "students.move.keepMany" : "students.move.keepOne"),
    },
  ];

  const describe = ({ to, keep }: MoveValues) => {
    if (keep) return undefined;
    const left = classesLeftBy(students, to);
    if (left.count === 0) return undefined;
    return one === undefined
      ? t("students.move.leavesMany", { count: left.count, classes: left.names })
      : t("students.move.leavesOne", { name: one.fullName, classes: left.names });
  };

  async function move({ to, keep }: MoveValues) {
    setPending(true);
    setError(null);
    const failed: string[] = [];
    for (const student of students) {
      try {
        await moveStudent(student, to, keep);
      } catch {
        failed.push(student.fullName);
      }
    }
    const touched = new Set([
      to,
      ...students.flatMap((s) => s.classes.map((c) => c.id)),
    ]);
    await Promise.all([
      client.invalidateQueries({ queryKey: ["admin-students"] }),
      ...[...touched].map((id) => invalidateClassMembership(client, id)),
    ]);
    setPending(false);
    const target = classes.find((klass) => klass.id === to)?.name ?? "";
    if (failed.length > 0) {
      setError(t("students.move.failed", { names: failed.join(", ") }));
      return;
    }
    onMoved();
    onOpenChange(false);
    toast(
      one === undefined
        ? t("students.move.movedMany", { count: students.length, klass: target })
        : t("students.move.movedOne", { name: one.fullName, klass: target }),
    );
  }

  return (
    <FormDialog<MoveValues>
      open={students.length > 0}
      onOpenChange={(open) => {
        if (pending) return;
        if (!open) setError(null);
        onOpenChange(open);
      }}
      title={
        one === undefined
          ? t("students.move.titleMany", { count: students.length })
          : t("students.move.titleOne", { name: one.fullName })
      }
      icon={ArrowRightLeft}
      description={describe}
      initial={INITIAL}
      fields={fields}
      submitLabel={t("students.move.submit")}
      pending={pending}
      error={error}
      returnFocus={returnFocus}
      onSubmit={(values) => void move(values)}
    />
  );
}
