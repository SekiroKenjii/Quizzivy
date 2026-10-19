import { useMemo, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { UserRoundCog } from "lucide-react";
import { FormDialog, type FormField } from "@/components/shared/form/FormDialog";
import { toast } from "@/components/ui/sonner";
import {
  changeClassTeacher,
  classTeachersQuery,
  invalidateAdminClasses,
  type AdminClass,
} from "@/features/classes/adminApi";
import { teacherRefusal } from "@/features/classes/adminText";
import { invalidateClass } from "@/features/classes/invalidate";

type Values = { teacher: string; notify: boolean };

/**
 * ChangeTeacherDialog is the deck's "Change teacher": the class's name, "New
 * teacher" from the active accounts of the Teachers and Admins tabs, starting
 * on the current one, and "Tell both teachers", on. Naming the current
 * teacher changes nothing. It is open while `klass` is set; a refusal of the
 * chosen teacher (`INVALID_RECIPIENT`) stays in the dialog.
 */
export function ChangeTeacherDialog({
  klass,
  onClose,
  returnFocus,
}: Readonly<{
  klass: AdminClass | null;
  onClose: () => void;
  returnFocus?: RefObject<HTMLElement | null> | undefined;
}>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const teachers = useQuery({ ...classTeachersQuery(), enabled: klass !== null });
  const initial = useMemo<Values>(
    () => ({ teacher: klass?.teacher.id ?? "", notify: true }),
    [klass],
  );

  let hint: string | undefined;
  if (teachers.isPending) hint = t("classes.admin.teachersLoading");
  else if (teachers.isError) hint = t("classes.admin.teachersFailed");
  const options = (teachers.data ?? []).map((one) => ({
    value: one.id,
    label: one.name,
    meta: one.email,
  }));
  if (klass !== null && !options.some((one) => one.value === klass.teacher.id))
    options.unshift({ value: klass.teacher.id, label: klass.teacher.name, meta: "" });
  const fields: readonly FormField<Values>[] = [
    {
      kind: "select",
      name: "teacher",
      label: t("classes.admin.newTeacher"),
      options,
      required: true,
      ...(hint === undefined ? {} : { hint }),
    },
    { kind: "toggle", name: "notify", text: t("classes.admin.notifyBoth") },
  ];

  const change = useMutation({
    mutationFn: ({ id, values }: { id: string; values: Values }) =>
      changeClassTeacher(id, values.teacher, values.notify),
    onSuccess: async (_, { id, values }) => {
      onClose();
      const name =
        teachers.data?.find((one) => one.id === values.teacher)?.name ??
        klass?.teacher.name ??
        "";
      toast(
        t("classes.admin.teacherChanged", { name: klass?.name ?? "", teacher: name }),
      );
      await invalidateClass(queryClient, id);
      await invalidateAdminClasses(queryClient);
    },
  });

  return (
    <FormDialog
      open={klass !== null}
      onOpenChange={(next) => {
        if (next) return;
        change.reset();
        onClose();
      }}
      title={t("classes.admin.changeTeacher")}
      description={klass?.name}
      icon={UserRoundCog}
      initial={initial}
      fields={fields}
      submitLabel={t("classes.admin.change")}
      pending={change.isPending}
      error={
        change.isError
          ? teacherRefusal(change.error, t, t("classes.admin.changeFailed"))
          : null
      }
      returnFocus={returnFocus}
      onSubmit={(values) => {
        if (klass !== null && !change.isPending)
          change.mutate({ id: klass.id, values });
      }}
    />
  );
}
