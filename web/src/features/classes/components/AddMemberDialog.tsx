import { useState, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Check, Search, UserPlus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/sonner";
import {
  DialogShell,
  DialogShellBody,
  DialogShellFooter,
  DialogShellHeader,
} from "@/components/shared/form/DialogShell";
import { LoadMoreSentinel } from "@/components/shared/LoadMoreSentinel";
import { addMember, type Class } from "@/features/classes/api";
import { invalidateClassMembership } from "@/features/classes/invalidate";
import { listStudents } from "@/features/students/api";
import { useLazyList } from "@/hooks/useLazyList";
import { ApiError } from "@/lib/api/errors";
import { useDebounced } from "@/lib/useDebounced";
import { cn } from "@/lib/utils";

const FOOTER_BUTTON =
  "text-ui h-9 px-4 leading-normal shadow-none in-data-[scale=deck]:h-9 in-data-[scale=deck]:px-4";

/**
 * AddMemberDialog is the deck's "Add students to {class}": a search by name
 * over the students the caller reaches, one row per student with the
 * classes they are in, ticked to add, those already in the class shown and
 * not choosable, and "Add {n}", which asks for at least one. It adds them
 * one by one and names any it could not add. Focus goes back to
 * `returnFocus` when it closes.
 */
export function AddMemberDialog({
  klass,
  open,
  onOpenChange,
  returnFocus,
}: Readonly<{
  klass: Pick<Class, "id" | "name">;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  returnFocus?: RefObject<HTMLElement | null> | undefined;
}>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [query, setQuery] = useState("");
  const [chosen, setChosen] = useState<ReadonlyMap<string, string>>(new Map());
  const [touched, setTouched] = useState(false);
  const search = useDebounced(query, 250).trim();
  const students = useLazyList({
    queryKey: ["admin-students", "picker", { q: search }],
    fetchPage: (page, signal) =>
      listStudents(
        search === "" ? { page, limit: 20 } : { q: search, page, limit: 20 },
        signal,
      ),
    enabled: open,
  });

  const add = useMutation({
    mutationFn: async (ids: readonly string[]) => {
      const failed: string[] = [];
      for (const id of ids) {
        try {
          await addMember(klass.id, id);
        } catch (cause) {
          failed.push(
            `${chosen.get(id) ?? id}: ${cause instanceof ApiError ? cause.message : t("classDetail.addFailed")}`,
          );
        }
      }
      return { added: ids.length - failed.length, failed };
    },
    onSuccess: async ({ added, failed }) => {
      await invalidateClassMembership(queryClient, klass.id);
      if (added > 0) toast(t("classDetail.addedCount", { count: added }));
      if (failed.length === 0) close();
    },
  });

  function close() {
    setQuery("");
    setChosen(new Map());
    setTouched(false);
    add.reset();
    onOpenChange(false);
  }

  function toggle(id: string, name: string) {
    if (add.isPending) return;
    setChosen((current) => {
      const next = new Map(current);
      if (!next.delete(id)) next.set(id, name);
      return next;
    });
  }

  const missing = touched && chosen.size === 0;
  const failed = add.data?.failed ?? [];

  return (
    <DialogShell
      open={open}
      onOpenChange={(next) => (next ? onOpenChange(true) : close())}
      width={520}
      returnFocus={returnFocus}
    >
      <DialogShellHeader
        title={t("classDetail.addStudentsTitle", { name: klass.name })}
        description={t("classDetail.addStudentHint")}
        icon={UserPlus}
      />
      <DialogShellBody>
        <label className="border-border bg-bg flex h-9 items-center gap-2 rounded-md border px-2.5">
          <Search aria-hidden="true" className="text-muted-fg size-3.75" />
          <input
            aria-label={t("classDetail.searchStudents")}
            placeholder={t("classDetail.searchStudents")}
            size={1}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            className="text-ui min-w-0 flex-1 bg-transparent outline-none"
          />
          {chosen.size > 0 ? (
            <span className="text-muted-fg text-xs whitespace-nowrap">
              {t("formDialog.selected", { count: chosen.size })}
            </span>
          ) : null}
        </label>
        <div
          role="group"
          aria-label={t("classDetail.studentsToAdd")}
          aria-describedby={missing ? "add-members-missing" : undefined}
          className="border-border flex max-h-75 flex-col overflow-y-auto rounded-lg border"
        >
          {students.isPending ? (
            <p className="text-muted-fg px-3 py-4.5 text-center text-sm">
              {t("common.loading")}
            </p>
          ) : null}
          {students.isError ? (
            <p role="alert" className="text-danger-ink px-3 py-4.5 text-center text-sm">
              {t("classDetail.studentsFailed")}
            </p>
          ) : null}
          {!students.isPending && !students.isError && students.items.length === 0 ? (
            <p className="text-muted-fg px-3 py-4.5 text-center text-sm">
              {t("classDetail.noStudentMatches")}
            </p>
          ) : null}
          {students.items.map((student) => {
            const member = student.classes.some((c) => c.id === klass.id);
            const on = member || chosen.has(student.id);
            const classes = student.classes.map((c) => c.name).join(", ");
            let meta = classes === "" ? t("classDetail.noClass") : classes;
            if (member) meta = t("classDetail.alreadyMember");
            return (
              <button
                key={student.id}
                type="button"
                role="checkbox"
                aria-checked={on}
                aria-disabled={member || undefined}
                className={cn(
                  "flex items-center gap-2.5 border-t px-3 py-2.5 text-left first:border-t-0",
                  member ? "cursor-default opacity-60" : "hover:bg-muted",
                  on && !member && "bg-muted",
                )}
                onClick={() => {
                  if (!member) toggle(student.id, student.fullName);
                }}
              >
                <span
                  aria-hidden="true"
                  className={cn(
                    "border-ring bg-card grid size-4 flex-none place-items-center rounded-sm border",
                    on && "border-primary bg-primary text-primary-fg",
                  )}
                >
                  {on ? <Check className="size-2.75" /> : null}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="text-ui block truncate">{student.fullName}</span>
                  <span className="text-muted-fg block truncate text-xs">{meta}</span>
                </span>
              </button>
            );
          })}
          <LoadMoreSentinel
            active={students.hasMore}
            loading={students.loadingMore}
            onVisible={students.loadMore}
          />
        </div>
        {missing ? (
          <p id="add-members-missing" className="text-danger-ink text-xs">
            {t("classDetail.pickOne")}
          </p>
        ) : null}
        {failed.length > 0 ? (
          <div role="alert" className="text-danger-ink text-xs">
            <p>{t("classDetail.someNotAdded")}</p>
            <ul className="mt-1 list-disc pl-4">
              {failed.map((line) => (
                <li key={line}>{line}</li>
              ))}
            </ul>
          </div>
        ) : null}
      </DialogShellBody>
      <DialogShellFooter>
        <Button
          type="button"
          variant="outline"
          className={`bg-card ${FOOTER_BUTTON} px-3.5 in-data-[scale=deck]:px-3.5`}
          onClick={close}
        >
          {t("common.cancel")}
        </Button>
        <Button
          type="button"
          className={cn(FOOTER_BUTTON, add.isPending && "opacity-50")}
          aria-disabled={add.isPending || undefined}
          onClick={() => {
            if (add.isPending) return;
            setTouched(true);
            if (chosen.size > 0) add.mutate([...chosen.keys()]);
          }}
        >
          {chosen.size > 0
            ? t("classDetail.addCount", { count: chosen.size })
            : t("classDetail.add")}
        </Button>
      </DialogShellFooter>
    </DialogShell>
  );
}
