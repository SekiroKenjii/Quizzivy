import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Search, UserPlus } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { listStudents } from "@/features/students/api";
import { addMember } from "@/features/classes/api";
import { invalidateClassMembership } from "@/features/classes/invalidate";
import { useDebounced } from "@/lib/useDebounced";
import { useLazyList } from "@/hooks/useLazyList";
import { LoadMoreSentinel } from "@/components/shared/LoadMoreSentinel";
import { ApiError } from "@/lib/api/errors";
import type { TFunction } from "i18next";
import type { ReactNode } from "react";

/**
 * G-06's "Thêm học viên": enrols someone who already has an account, which is
 * the path for a student who joined before the class existed or who lost the
 * code.
 */
export function AddMemberDialog({
  classId,
  open,
  onOpenChange,
}: Readonly<{
  classId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [query, setQuery] = useState("");
  const [error, setError] = useState<string | null>(null);
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
    mutationFn: (userId: string) => addMember(classId, userId),
    onSuccess: async () => {
      setError(null);
      await invalidateClassMembership(queryClient, classId);
    },
    onError: (cause) =>
      setError(cause instanceof ApiError ? cause.message : t("classDetail.addFailed")),
  });

  const items = students.items;

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          setQuery("");
          setError(null);
        }
        onOpenChange(next);
      }}
    >
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("classDetail.addStudent")}</DialogTitle>
          <DialogDescription>{t("classDetail.addStudentHint")}</DialogDescription>
        </DialogHeader>

        <div className="relative">
          <Search
            className="text-muted-foreground pointer-events-none absolute top-2.5 left-2.5 size-4"
            aria-hidden="true"
          />
          <Input
            // eslint-disable-next-line jsx-a11y/no-autofocus
            autoFocus
            className="pl-9"
            value={query}
            placeholder={t("classDetail.searchStudents")}
            aria-label={t("classDetail.searchStudents")}
            onChange={(event) => setQuery(event.target.value)}
          />
        </div>

        {error === null ? null : (
          <p role="alert" className="text-destructive text-sm">
            {error}
          </p>
        )}

        <ul className="max-h-72 space-y-1 overflow-y-auto">
          {memberNote(students, items.length, t) ??
            items.map((student) => {
              // The row carries every membership, so this cannot depend on which roster page is loaded.
              const already = student.classes.some((c) => c.id === classId);
              return (
                <li
                  key={student.id}
                  className="flex items-center gap-3 rounded-md px-2 py-1.5"
                >
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium">{student.fullName}</p>
                    <p className="text-muted-foreground truncate text-xs">
                      {student.email}
                    </p>
                  </div>
                  {already ? (
                    <span className="text-muted-foreground shrink-0 text-xs">
                      {t("classDetail.alreadyMember")}
                    </span>
                  ) : (
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={add.isPending}
                      onClick={() => add.mutate(student.id)}
                    >
                      <UserPlus aria-hidden="true" />
                      {t("classDetail.add")}
                    </Button>
                  )}
                </li>
              );
            })}
          <LoadMoreSentinel
            as="li"
            active={students.hasMore}
            loading={students.loadingMore}
            onVisible={students.loadMore}
          />
        </ul>
      </DialogContent>
    </Dialog>
  );
}

function memberNote(
  students: { readonly isPending: boolean; readonly isError: boolean },
  shown: number,
  t: TFunction,
): ReactNode | null {
  if (students.isPending) {
    return <li className="text-muted-foreground p-2 text-sm">{t("common.loading")}</li>;
  }
  if (students.isError) {
    return (
      <li role="alert" className="text-destructive p-2 text-sm">
        {t("classDetail.studentsFailed")}
      </li>
    );
  }
  if (shown === 0) {
    return (
      <li className="text-muted-foreground p-2 text-sm">
        {t("classDetail.noStudentMatches")}
      </li>
    );
  }
  return null;
}
