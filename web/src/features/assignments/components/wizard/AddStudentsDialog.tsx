import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { UserPlus } from "lucide-react";
import { FormDialog, type FormField } from "@/components/shared/form/FormDialog";
import type { Token } from "@/features/assignments/components/TokenField";
import { listStudents, type Student } from "@/features/students/api";
import { notify } from "@/lib/toast";

type Values = { students: readonly string[] };

const PAGE = 100;

async function allActiveStudents(signal: AbortSignal): Promise<Student[]> {
  const first = await listStudents({ status: "active", limit: PAGE }, signal);
  const rest = await Promise.all(
    Array.from(
      { length: Math.max(0, Math.ceil(first.total / first.pageSize) - 1) },
      (_, i) => listStudents({ status: "active", limit: PAGE, page: i + 2 }, signal),
    ),
  );
  return [...first.items, ...rest.flatMap((page) => page.items)];
}

/**
 * AddStudentsDialog picks the assignment's individual students from every
 * active student the teacher reaches, searched by name or class. A student in
 * a class already picked says so in its meta line. Done replaces the
 * individual targets with the picked ones; a token keeps its classes as its
 * hint.
 */
export function AddStudentsDialog({
  open,
  onOpenChange,
  selected,
  classIds,
  onDone,
}: Readonly<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  selected: readonly Token[];
  classIds: readonly string[];
  onDone: (students: Token[]) => void;
}>) {
  const { t } = useTranslation();
  const students = useQuery({
    queryKey: ["admin-students", "wizard", "all-active"],
    queryFn: ({ signal }) => allActiveStudents(signal),
    enabled: open,
  });
  const picked = new Set(classIds);
  const rows = students.data ?? [];
  const byId = new Map(rows.map((row) => [row.id, row]));
  const options = rows.map((row) => {
    const classes = row.classes.map((klass) => klass.name).join(", ");
    const included = row.classes.some((klass) => picked.has(klass.id));
    return {
      value: row.id,
      label: row.fullName,
      meta: included
        ? [classes, t("assignments.wizard.addDialog.alreadyIncluded")].join(" · ")
        : classes,
    };
  });
  const fields: FormField<Values>[] = [
    {
      kind: "list",
      name: "students",
      label: t("assignments.wizard.addDialog.label"),
      required: true,
      requiredText: t("assignments.wizard.addDialog.required"),
      options,
      searchPlaceholder: t("assignments.wizard.addDialog.search"),
    },
  ];
  const previous = new Map(selected.map((token) => [token.id, token]));

  return (
    <FormDialog<Values>
      open={open}
      onOpenChange={onOpenChange}
      title={t("assignments.wizard.addIndividuals")}
      description={t("assignments.wizard.addDialog.description")}
      icon={UserPlus}
      width={520}
      initial={{ students: selected.map((token) => token.id) }}
      fields={students.isSuccess ? fields : []}
      disabled={!students.isSuccess}
      error={students.isError ? t("assignments.wizard.addDialog.failed") : null}
      submitLabel={(values) =>
        values.students.length > 0
          ? t("assignments.wizard.addDialog.addCount", {
              count: values.students.length,
            })
          : t("assignments.wizard.addDialog.add")
      }
      onSubmit={(values) => {
        const tokens = values.students.flatMap((id): Token[] => {
          const row = byId.get(id);
          if (row) {
            const hint = row.classes.map((klass) => klass.name).join(", ");
            return [{ id, label: row.fullName, ...(hint === "" ? {} : { hint }) }];
          }
          const kept = previous.get(id);
          return kept ? [kept] : [];
        });
        onDone(tokens);
        onOpenChange(false);
        notify.success(
          t("assignments.wizard.addDialog.picked", { count: tokens.length }),
        );
      }}
    >
      {students.isPending && open ? (
        <p role="status" className="text-muted-fg text-sm">
          {t("common.loading")}
        </p>
      ) : null}
    </FormDialog>
  );
}
