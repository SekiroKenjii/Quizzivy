import { useMemo, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { GraduationCap } from "lucide-react";
import { FormDialog } from "@/components/shared/form/FormDialog";
import { toast } from "@/components/ui/sonner";
import { createClass, rotateJoinCode } from "@/features/classes/api";
import { useJoinCodesOff } from "@/features/organization/api";
import {
  blankAsNull,
  classFields,
  saveError,
  type ClassFieldValues,
} from "@/features/classes/classFields";

const EMPTY: ClassFieldValues = { name: "", schedule: "", room: "" };
const FIRST_CODE = { expiresInDays: 30, maxUses: 40 } as const;

/**
 * NewClassDialog is the deck's "New class": a name, a schedule and a room.
 * It creates the class open to joining, then issues its first code for 30
 * days and 40 uses (O-06) and toasts "Class created · join code {code}". A
 * class whose code could not be issued is still created, and the toast says
 * to make one from its card. While the centre has joining by code off it
 * issues no code and the toast says why (D-028).
 */
export function NewClassDialog({
  open,
  onOpenChange,
  returnFocus,
}: Readonly<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  returnFocus?: RefObject<HTMLElement | null> | undefined;
}>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const fields = useMemo(() => classFields(t), [t]);
  const joinOff = useJoinCodesOff();

  const create = useMutation({
    mutationFn: async (values: ClassFieldValues) => {
      const created = await createClass({
        name: values.name.trim(),
        scheduleLabel: blankAsNull(values.schedule),
        room: blankAsNull(values.room),
        selfJoinEnabled: true,
      });
      if (joinOff) return undefined;
      try {
        const issued = await rotateJoinCode(created.id, FIRST_CODE);
        return issued.code;
      } catch {
        return null;
      }
    },
    onSuccess: async (code) => {
      onOpenChange(false);
      if (code === undefined) toast(t("classes.createdJoinOff"));
      else if (code === null) toast(t("classes.createdNoCode"));
      else toast(t("classes.createdWithCode", { code }));
      await queryClient.invalidateQueries({ queryKey: ["teacher-classes"] });
    },
  });

  const error = create.isError
    ? saveError(create.error, t("classes.createFailed"))
    : null;

  return (
    <FormDialog
      open={open}
      onOpenChange={(next) => {
        if (!next) create.reset();
        onOpenChange(next);
      }}
      title={t("classes.new")}
      description={t(joinOff ? "classes.newHintOff" : "classes.newHint")}
      icon={GraduationCap}
      initial={EMPTY}
      fields={fields}
      submitLabel={t("classes.create")}
      pending={create.isPending}
      error={error}
      returnFocus={returnFocus}
      onSubmit={(values) => {
        if (!create.isPending) create.mutate(values);
      }}
    />
  );
}
