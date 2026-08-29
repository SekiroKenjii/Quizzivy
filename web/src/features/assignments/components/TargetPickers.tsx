import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { TokenField, type Token } from "@/features/assignments/components/TokenField";
import { listStudents } from "@/features/assignments/api";
import { fetchClasses } from "@/features/classes/api";
import { useDebounced } from "@/lib/useDebounced";

interface PickerProps {
  selected: Token[];
  onAdd: (token: Token) => void;
  onRemove: (id: string) => void;
}

/** Filters the one cached class list; a practice has single-digit classes (§1.3). */
export function ClassTargetPicker({ selected, onAdd, onRemove }: PickerProps) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  const classes = useQuery({
    queryKey: ["admin-classes"],
    queryFn: ({ signal }) => fetchClasses(signal),
  });

  const all = classes.data?.items ?? [];
  const needle = fold(query.trim());
  const options = (
    needle === "" ? all : all.filter((c) => fold(c.name).includes(needle))
  ).map((c) => ({ id: c.id, label: c.name, hint: String(c.studentCount) }));

  return (
    <TokenField
      label={t("assignments.classes")}
      placeholder={t("assignments.addClass")}
      selected={selected}
      options={options}
      loading={classes.isPending}
      query={query}
      onQueryChange={setQuery}
      onAdd={onAdd}
      onRemove={onRemove}
    />
  );
}

/** Searches the server, because the student table is the one that grows. */
export function StudentTargetPicker({ selected, onAdd, onRemove }: PickerProps) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  const search = useDebounced(query, 250).trim();

  const students = useQuery({
    queryKey: ["admin-students", { q: search }],
    queryFn: ({ signal }) =>
      listStudents(search === "" ? { limit: 20 } : { q: search, limit: 20 }, signal),
  });

  return (
    <TokenField
      label={t("assignments.individualStudents")}
      optionalNote={t("assignments.optional")}
      placeholder={t("assignments.findStudent")}
      selected={selected}
      options={(students.data?.items ?? []).map((s) => ({
        id: s.id,
        label: s.fullName,
        hint: s.email,
      }))}
      loading={students.isPending}
      query={query}
      onQueryChange={setQuery}
      onAdd={onAdd}
      onRemove={onRemove}
    />
  );
}

function fold(value: string): string {
  return value
    .normalize("NFD")
    .replace(/\p{Diacritic}/gu, "")
    .toLowerCase();
}
