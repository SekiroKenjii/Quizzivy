import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { TemporaryPasswordCard } from "@/features/students/components/TemporaryPasswordCard";
import { createStudent } from "@/features/students/api";
import { fetchClasses } from "@/features/classes/api";
import { useLazyList } from "@/hooks/useLazyList";
import { useDebounced } from "@/lib/useDebounced";
import { LoadMoreSentinel } from "@/components/shared/LoadMoreSentinel";
import { ApiError, fieldMessages } from "@/lib/api/errors";

/**
 * G-07's "Thêm học viên".
 *
 * §6.3 has no password self-signup, so an admin-created account necessarily
 * arrives with a temporary password — which is why this dialog does not close
 * on success: the password is returned once and closing would discard it.
 */
export function NewStudentDialog({
  open,
  onOpenChange,
}: Readonly<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
}>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [fullName, setFullName] = useState("");
  const [email, setEmail] = useState("");
  const [classIds, setClassIds] = useState<string[]>([]);
  const toggleClass = (id: string, checked: boolean) =>
    setClassIds((current) =>
      checked ? [...current, id] : current.filter((existing) => existing !== id),
    );
  const [temporary, setTemporary] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const [classQuery, setClassQuery] = useState("");
  const classSearch = useDebounced(classQuery, 250).trim();
  const classes = useLazyList({
    queryKey: ["admin-classes", "picker", { q: classSearch }],
    fetchPage: (page, signal) =>
      fetchClasses(
        classSearch === "" ? { page, limit: 20 } : { q: classSearch, page, limit: 20 },
        signal,
      ),
    enabled: open,
  });

  // Every close goes through here.
  function close() {
    reset();
    onOpenChange(false);
  }

  function reset() {
    setFullName("");
    setEmail("");
    setClassIds([]);
    setClassQuery("");
    setTemporary(null);
    setError(null);
  }

  const create = useMutation({
    mutationFn: () =>
      createStudent({
        email: email.trim(),
        fullName: fullName.trim(),
        ...(classIds.length > 0 ? { classIds } : {}),
      }),
    onSuccess: async (result) => {
      setError(null);
      setTemporary(result.temporaryPassword);
      await queryClient.invalidateQueries({ queryKey: ["admin-students"] });
      await queryClient.invalidateQueries({ queryKey: ["admin-classes"] });
    },
    onError: (cause) => {
      const fields = fieldMessages(cause);
      setError(
        fields[0] ??
          (cause instanceof ApiError ? cause.message : t("students.createFailed")),
      );
    },
  });

  const ready = fullName.trim() !== "" && email.trim() !== "";

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) reset();
        onOpenChange(next);
      }}
    >
      <DialogContent
        showCloseButton={temporary === null}
        onEscapeKeyDown={(event) => temporary !== null && event.preventDefault()}
        onPointerDownOutside={(event) => temporary !== null && event.preventDefault()}
        onInteractOutside={(event) => temporary !== null && event.preventDefault()}
      >
        <DialogHeader>
          <DialogTitle>{t("students.new")}</DialogTitle>
          <DialogDescription>{t("students.newHint")}</DialogDescription>
        </DialogHeader>

        {temporary === null ? (
          <div className="space-y-3">
            <div>
              <Label htmlFor="student-name">{t("students.fullName")}</Label>
              <Input
                id="student-name"
                className="mt-1.5"
                value={fullName}
                onChange={(event) => setFullName(event.target.value)}
              />
            </div>
            <div>
              <Label htmlFor="student-email">{t("students.email")}</Label>
              <Input
                id="student-email"
                type="email"
                className="mt-1.5"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
              />
            </div>
            {classes.total === 0 && classSearch === "" ? null : (
              <fieldset>
                <legend className="text-sm font-medium">
                  {t("students.addToClasses")}
                </legend>
                <Input
                  className="mt-1.5"
                  value={classQuery}
                  placeholder={t("students.searchClasses")}
                  aria-label={t("students.searchClasses")}
                  onChange={(event) => setClassQuery(event.target.value)}
                />
                <ul className="mt-1.5 max-h-48 space-y-1 overflow-y-auto">
                  {classes.items.length === 0 && !classes.isPending ? (
                    <li className="text-muted-foreground text-sm">
                      {t("students.noClassMatches")}
                    </li>
                  ) : (
                    classes.items.map((klass) => (
                      <li key={klass.id}>
                        <label className="flex items-center gap-2 text-sm">
                          <input
                            type="checkbox"
                            checked={classIds.includes(klass.id)}
                            onChange={(event) =>
                              toggleClass(klass.id, event.target.checked)
                            }
                          />
                          {klass.name}
                        </label>
                      </li>
                    ))
                  )}
                  <LoadMoreSentinel
                    as="li"
                    active={classes.hasMore}
                    loading={classes.loadingMore}
                    onVisible={classes.loadMore}
                  />
                </ul>
                {classIds.length > 0 && (
                  <p className="text-muted-foreground mt-1.5 text-xs">
                    {t("students.selectedClasses", { count: classIds.length })}
                  </p>
                )}
              </fieldset>
            )}
          </div>
        ) : (
          <div className="space-y-3">
            <p className="text-sm">{t("students.created", { name: fullName })}</p>
            <TemporaryPasswordCard password={temporary} />
          </div>
        )}

        {error === null ? null : (
          <p role="alert" className="text-destructive text-sm">
            {error}
          </p>
        )}

        <DialogFooter>
          {temporary === null ? (
            <>
              <Button variant="outline" onClick={close}>
                {t("common.cancel")}
              </Button>
              <Button
                disabled={!ready || create.isPending}
                onClick={() => create.mutate()}
              >
                {create.isPending ? t("common.loading") : t("students.create")}
              </Button>
            </>
          ) : (
            <Button onClick={close}>{t("students.done")}</Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
