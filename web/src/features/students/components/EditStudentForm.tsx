import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "@/components/ui/sonner";
import { updateStudent, type Student } from "@/features/students/api";
import {
  editStudentSchema,
  type EditStudentValues,
} from "@/features/students/editStudentSchema";
import { ApiError, fieldMessages } from "@/lib/api/errors";

/** G-07a: the drawer header as a two-field form; Lưu is the only thing that writes. */
export function EditStudentForm({
  student,
  onDone,
}: Readonly<{
  student: Student;
  onDone: () => void;
}>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const form = useForm<EditStudentValues>({
    resolver: zodResolver(editStudentSchema),
    defaultValues: { fullName: student.fullName, email: student.email },
    mode: "onTouched",
  });
  const save = useMutation({
    mutationFn: (values: EditStudentValues) => updateStudent(student.id, values),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["admin-students"] });
      await queryClient.invalidateQueries({ queryKey: ["admin-student", student.id] });
      toast(t("students.saved"));
      onDone();
    },
    onError: (cause) => {
      form.setError("root", {
        message:
          fieldMessages(cause)[0] ??
          (cause instanceof ApiError ? cause.message : t("students.saveFailed")),
      });
    },
  });
  const { errors } = form.formState;

  return (
    <form
      className="min-w-0 flex-1 space-y-3"
      noValidate
      onSubmit={form.handleSubmit((values) => save.mutate(values))}
    >
      <div>
        <Label htmlFor="student-edit-name">{t("students.fullName")}</Label>
        <Input
          id="student-edit-name"
          className="mt-1.5"
          aria-invalid={errors.fullName !== undefined}
          {...form.register("fullName")}
        />
        {errors.fullName?.message === undefined ? null : (
          <p className="text-destructive mt-1 text-xs">{t(errors.fullName.message)}</p>
        )}
      </div>
      <div>
        <Label htmlFor="student-edit-email">{t("students.email")}</Label>
        <Input
          id="student-edit-email"
          type="email"
          className="mt-1.5"
          aria-invalid={errors.email !== undefined}
          {...form.register("email")}
        />
        <p className="text-muted-foreground mt-1 text-xs leading-relaxed">
          {errors.email?.message === undefined
            ? t("students.emailHint")
            : t(errors.email.message)}
        </p>
      </div>
      {errors.root?.message === undefined ? null : (
        <p role="alert" className="text-destructive text-xs">
          {errors.root.message}
        </p>
      )}
      <div className="flex gap-2">
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="flex-1"
          onClick={onDone}
        >
          {t("common.cancel")}
        </Button>
        <Button type="submit" size="sm" className="flex-1" disabled={save.isPending}>
          {t("common.save")}
        </Button>
      </div>
    </form>
  );
}
