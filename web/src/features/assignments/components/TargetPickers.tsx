import { useState } from "react";
import { useTranslation } from "react-i18next";
import { TokenField, type Token } from "@/features/assignments/components/TokenField";
import { listStudents } from "@/features/students/api";
import { fetchClasses } from "@/features/classes/api";
import { useLazyList } from "@/hooks/useLazyList";
import { useDebounced } from "@/lib/useDebounced";

interface PickerProps {
  selected: Token[];
  onAdd: (token: Token) => void;
  onRemove: (id: string) => void;
}

const PAGE = 20;

/**
 * Searches the server and pages as the list is scrolled. §1.3 promised
 * single-digit classes; a development database already holds over a hundred,
 * and a picker that read them all at once was the first thing to break.
 */
export function ClassTargetPicker({ selected, onAdd, onRemove }: PickerProps) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  const search = useDebounced(query, 250).trim();

  const classes = useLazyList({
    queryKey: ["admin-classes", "picker", { q: search }],
    fetchPage: (page, signal) =>
      fetchClasses(
        search === "" ? { page, limit: PAGE } : { q: search, page, limit: PAGE },
        signal,
      ),
  });

  return (
    <TokenField
      label={t("assignments.classes")}
      placeholder={t("assignments.addClass")}
      selected={selected}
      options={classes.items.map((c) => ({
        id: c.id,
        label: c.name,
        hint: String(c.studentCount),
      }))}
      loading={classes.isPending}
      hasMore={classes.hasMore}
      loadingMore={classes.loadingMore}
      onEndReached={classes.loadMore}
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

  const students = useLazyList({
    queryKey: ["admin-students", "picker", { q: search }],
    fetchPage: (page, signal) =>
      listStudents(
        search === "" ? { page, limit: PAGE } : { q: search, page, limit: PAGE },
        signal,
      ),
  });

  return (
    <TokenField
      label={t("assignments.individualStudents")}
      optionalNote={t("assignments.optional")}
      placeholder={t("assignments.findStudent")}
      selected={selected}
      options={students.items.map((s) => ({
        id: s.id,
        label: s.fullName,
        hint: s.email,
      }))}
      loading={students.isPending}
      hasMore={students.hasMore}
      loadingMore={students.loadingMore}
      onEndReached={students.loadMore}
      query={query}
      onQueryChange={setQuery}
      onAdd={onAdd}
      onRemove={onRemove}
    />
  );
}
