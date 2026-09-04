import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Controller, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
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
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { toast } from "@/components/ui/sonner";
import { createClass } from "@/features/classes/api";
import { newClassSchema, type NewClassValues } from "@/features/classes/newClassSchema";
import { ApiError, fieldMessages } from "@/lib/api/errors";

/** G-08's "Lớp mới": two fields and a switch. The join code is issued on the class page. */
export function NewClassDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [error, setError] = useState<string | null>(null);
  const form = useForm<NewClassValues>({
    resolver: zodResolver(newClassSchema),
    defaultValues: { name: "", description: "", selfJoinEnabled: false },
    mode: "onTouched",
  });

  const create = useMutation({
    mutationFn: (values: NewClassValues) =>
      createClass({
        name: values.name,
        description:
          values.description.trim() === "" ? null : values.description.trim(),
        selfJoinEnabled: values.selfJoinEnabled,
      }),
    onSuccess: async (created) => {
      await queryClient.invalidateQueries({ queryKey: ["admin-classes"] });
      close();
      toast(t("classes.created"));
      void navigate(`/admin/classes/${created.id}`);
    },
    onError: (cause) => {
      setError(
        fieldMessages(cause)[0] ??
          (cause instanceof ApiError ? cause.message : t("classes.createFailed")),
      );
    },
  });

  function close() {
    form.reset();
    setError(null);
    onOpenChange(false);
  }

  const nameError = form.formState.errors.name?.message;

  return (
    <Dialog open={open} onOpenChange={(next) => (next ? onOpenChange(true) : close())}>
      <DialogContent>
        <form
          onSubmit={form.handleSubmit((values) => create.mutate(values))}
          noValidate
        >
          <DialogHeader>
            <DialogTitle>{t("classes.new")}</DialogTitle>
            <DialogDescription>{t("classes.newHint")}</DialogDescription>
          </DialogHeader>

          <div className="mt-4 space-y-3">
            <div>
              <Label htmlFor="class-name">{t("classes.name")}</Label>
              <Input
                id="class-name"
                className="mt-1.5"
                placeholder={t("classes.namePlaceholder")}
                aria-invalid={nameError !== undefined}
                aria-describedby={
                  nameError === undefined ? "class-name-hint" : "class-name-error"
                }
                {...form.register("name")}
              />
              {nameError === undefined ? (
                <p
                  id="class-name-hint"
                  className="text-muted-foreground mt-1.5 text-xs"
                >
                  {t("classes.nameHint")}
                </p>
              ) : (
                <p id="class-name-error" className="text-destructive mt-1.5 text-xs">
                  {t(nameError)}
                </p>
              )}
            </div>
            <div>
              <Label htmlFor="class-description">
                {t("classes.description")}{" "}
                <span className="text-muted-foreground font-normal">
                  — {t("classes.optional")}
                </span>
              </Label>
              <Textarea
                id="class-description"
                className="mt-1.5 min-h-14"
                placeholder={t("classes.descriptionPlaceholder")}
                {...form.register("description")}
              />
            </div>
            <div className="flex items-center justify-between gap-3">
              <div>
                <Label htmlFor="class-self-join" className="font-normal">
                  {t("classes.selfJoin")}
                </Label>
                <p className="text-muted-foreground mt-0.5 text-xs">
                  {t("classes.selfJoinHint")}
                </p>
              </div>
              <Controller
                control={form.control}
                name="selfJoinEnabled"
                render={({ field }) => (
                  <Switch
                    id="class-self-join"
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                )}
              />
            </div>
            {error === null ? null : (
              <p role="alert" className="text-destructive text-sm">
                {error}
              </p>
            )}
          </div>

          <DialogFooter className="mt-5">
            <Button type="button" variant="outline" className="flex-1" onClick={close}>
              {t("common.cancel")}
            </Button>
            <Button type="submit" className="flex-1" disabled={create.isPending}>
              {t("classes.create")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
