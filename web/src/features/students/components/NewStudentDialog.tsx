import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
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
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [fullName, setFullName] = useState("");
  const [email, setEmail] = useState("");
  const [classIds, setClassIds] = useState<string[]>([]);
  const [temporary, setTemporary] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const classes = useQuery({
    queryKey: ["admin-classes"],
    queryFn: ({ signal }) => fetchClasses({ limit: 100 }, signal),
    enabled: open,
  });

  // Every close goes through here. Radix does not fire onOpenChange when a
  // controlled `open` is flipped from outside, so a button that called
  // onOpenChange(false) directly left `temporary` alive -- and the next "Thêm
  // học viên" reopened straight onto the previous student's password.
  function close() {
    reset();
    onOpenChange(false);
  }

  function reset() {
    setFullName("");
    setEmail("");
    setClassIds([]);
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
      <DialogContent>
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
            {(classes.data?.items.length ?? 0) === 0 ? null : (
              <fieldset>
                <legend className="text-sm font-medium">
                  {t("students.addToClasses")}
                </legend>
                <div className="mt-1.5 space-y-1">
                  {classes.data?.items.map((klass) => (
                    <label key={klass.id} className="flex items-center gap-2 text-sm">
                      <input
                        type="checkbox"
                        checked={classIds.includes(klass.id)}
                        onChange={(event) =>
                          setClassIds((current) =>
                            event.target.checked
                              ? [...current, klass.id]
                              : current.filter((id) => id !== klass.id),
                          )
                        }
                      />
                      {klass.name}
                    </label>
                  ))}
                </div>
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
