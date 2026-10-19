import { useMemo, useRef, type RefObject } from "react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseQueryResult,
} from "@tanstack/react-query";
import { GraduationCap, Pencil, UserRoundCog } from "lucide-react";
import { FormDialog, type FormField } from "@/components/shared/form/FormDialog";
import { toast } from "@/components/ui/sonner";
import {
  classTeachersQuery,
  createClassForTeacher,
  invalidateAdminClasses,
  teacherOptions,
  type AdminClass,
  type ClassTeacherOption,
} from "@/features/classes/adminApi";
import { updateClass } from "@/features/classes/api";
import { teacherRefusal } from "@/features/classes/adminText";
import { blankAsNull } from "@/features/classes/classFields";
import { invalidateClass } from "@/features/classes/invalidate";
import { useJoinCodesOff } from "@/features/organization/api";

type Values = { name: string; teacher: string; schedule: string };

const BLANK: Values = { name: "", teacher: "", schedule: "" };

function fieldsOf(
  t: TFunction,
  klass: AdminClass | null,
  teachers: UseQueryResult<readonly ClassTeacherOption[]>,
  onChangeTeacher: () => void,
): readonly FormField<Values>[] {
  const name: FormField<Values> = {
    kind: "text",
    name: "name",
    label: t("classes.name"),
    placeholder: t("classes.admin.namePlaceholder"),
    required: true,
    maxLength: 120,
  };
  const schedule: FormField<Values> = {
    kind: "text",
    name: "schedule",
    label: t("classes.schedule"),
    placeholder: t("classes.schedulePlaceholder"),
    maxLength: 120,
  };
  if (klass !== null)
    return [
      name,
      {
        kind: "info",
        name: "teacher",
        label: t("classes.admin.column.teacher"),
        rows: [
          {
            icon: UserRoundCog,
            text: klass.teacher.disabled
              ? t("classes.admin.teacherDisabledInline", { name: klass.teacher.name })
              : klass.teacher.name,
            action: {
              label: t("classes.admin.changeTeacherShort"),
              onAction: onChangeTeacher,
            },
          },
        ],
      },
      schedule,
    ];
  let hint: string | undefined;
  if (teachers.isPending) hint = t("classes.admin.teachersLoading");
  else if (teachers.isError) hint = t("classes.admin.teachersFailed");
  else if (teachers.data.length === 0) hint = t("classes.admin.teachersNone");
  return [
    name,
    {
      kind: "select",
      name: "teacher",
      label: t("classes.admin.column.teacher"),
      placeholder: t("classes.admin.teacherPlaceholder"),
      options: teacherOptions(teachers.data ?? []),
      required: true,
      ...(hint === undefined ? {} : { hint }),
    },
    schedule,
  ];
}

/**
 * AdminClassDialog is the deck's "New class" and "Edit class" for the Admin
 * console. New class (`klass` null while `open`) takes a name, the teacher,
 * from the active accounts of the Teachers and Admins tabs, and a schedule;
 * it creates the class for that teacher, whose first join code the server
 * issues unless the centre has joining by code off. Edit class (`klass` set)
 * saves the name and the schedule through the teacher workspace's operation,
 * and shows the teacher read-only with "Change teacher" as its own action
 * (`onChangeTeacher`), so a reassignment always offers to tell both teachers.
 */
export function AdminClassDialog({
  open,
  klass,
  onClose,
  onChangeTeacher,
  returnFocus,
}: Readonly<{
  open: boolean;
  klass: AdminClass | null;
  onClose: () => void;
  onChangeTeacher: (klass: AdminClass) => void;
  returnFocus?: RefObject<HTMLElement | null> | undefined;
}>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const joinOff = useJoinCodesOff();
  const creating = open && klass === null;
  const teachers = useQuery({ ...classTeachersQuery(), enabled: creating });
  const fields = fieldsOf(t, klass, teachers, () => {
    if (klass !== null) onChangeTeacher(klass);
  });
  const initial = useMemo<Values>(
    () =>
      klass === null
        ? BLANK
        : {
            name: klass.name,
            teacher: klass.teacher.id,
            schedule: klass.scheduleLabel ?? "",
          },
    [klass],
  );

  const sending = useRef(false);
  const save = useMutation({
    mutationFn: async (values: Values) => {
      if (klass !== null) {
        await updateClass(klass.id, {
          name: values.name.trim(),
          scheduleLabel: blankAsNull(values.schedule),
        });
        return null;
      }
      return createClassForTeacher({
        name: values.name.trim(),
        teacherId: values.teacher,
        scheduleLabel: blankAsNull(values.schedule),
      });
    },
    onSuccess: async (created) => {
      onClose();
      if (created === null) toast(t("classes.updated"));
      else if (created.joinCode == null && joinOff) toast(t("classes.createdJoinOff"));
      else toast(t("classes.admin.created"));
      if (klass !== null) await invalidateClass(queryClient, klass.id);
      await invalidateAdminClasses(queryClient);
    },
    onError: () => invalidateAdminClasses(queryClient),
    onSettled: () => {
      sending.current = false;
    },
  });

  const fallback = t(klass === null ? "classes.createFailed" : "classes.updateFailed");
  const error = save.isError ? teacherRefusal(save.error, t, fallback) : null;

  const newHint = joinOff ? "classes.admin.newHintOff" : "classes.admin.newHint";
  return (
    <FormDialog
      open={open}
      onOpenChange={(next) => {
        if (next) return;
        save.reset();
        onClose();
      }}
      title={t(klass === null ? "classes.new" : "classes.edit")}
      description={klass === null ? t(newHint) : undefined}
      icon={klass === null ? GraduationCap : Pencil}
      initial={initial}
      fields={fields}
      submitLabel={t(klass === null ? "classes.create" : "common.saveChanges")}
      pending={save.isPending}
      error={error}
      returnFocus={returnFocus}
      onSubmit={(values) => {
        if (sending.current) return;
        sending.current = true;
        save.mutate(values);
      }}
    />
  );
}
