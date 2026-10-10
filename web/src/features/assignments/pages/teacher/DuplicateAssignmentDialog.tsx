import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery } from "@tanstack/react-query";
import { Copy } from "lucide-react";
import { FormDialog } from "@/components/shared/form/FormDialog";
import { duplicateAssignment, type Assignment } from "@/features/assignments/api";
import { fetchClasses } from "@/features/classes/api";
import { ApiError } from "@/lib/api/errors";

/**
 * DuplicateAssignmentDialog is the row menu's "Duplicate": a draft with the
 * same test and rules, assigned to the classes chosen, the original's classes
 * to start with. Nothing is sent to students. `onDuplicated` hears the draft.
 */
export function DuplicateAssignmentDialog({
  assignment,
  open,
  onOpenChange,
  onDuplicated,
}: Readonly<{
  assignment: Assignment | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDuplicated: (draft: Assignment) => void;
}>) {
  const { t } = useTranslation();
  const [error, setError] = useState<string | null>(null);
  const classes = useQuery({
    queryKey: ["admin-classes", "picker", { limit: 100 }],
    queryFn: ({ signal }) => fetchClasses({ limit: 100 }, signal),
    enabled: open,
    staleTime: 60_000,
  });
  const duplicate = useMutation({
    mutationFn: ({ id, classIds }: { id: string; classIds: readonly string[] }) =>
      duplicateAssignment(id, classIds),
    onSuccess: (draft) => {
      setError(null);
      onOpenChange(false);
      onDuplicated(draft);
    },
    onError: (cause) =>
      setError(
        cause instanceof ApiError
          ? cause.message
          : t("assignments.list.duplicateFailed"),
      ),
  });
  const own = assignment?.targets.classes ?? [];
  const known = new Map(own.map((klass) => [klass.id, klass.name]));
  for (const klass of classes.data?.items ?? []) known.set(klass.id, klass.name);
  return (
    <FormDialog
      key={assignment?.id ?? "none"}
      open={open && assignment !== null}
      onOpenChange={(next) => {
        if (!next) setError(null);
        onOpenChange(next);
      }}
      icon={Copy}
      title={t("assignments.list.duplicateTitle")}
      description={t("assignments.list.duplicateBody")}
      initial={{ classIds: own.map((klass) => klass.id) as readonly string[] }}
      fields={[
        {
          kind: "chips",
          name: "classIds",
          label: t("assignments.list.duplicateClasses"),
          options: [...known].map(([value, label]) => ({ value, label })),
        },
      ]}
      submitLabel={t("assignments.list.duplicateSubmit")}
      pending={duplicate.isPending}
      error={error}
      onSubmit={({ classIds }) => {
        if (assignment !== null) duplicate.mutate({ id: assignment.id, classIds });
      }}
    />
  );
}
