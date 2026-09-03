import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { updateClass, type Class } from "@/features/classes/api";
import { ApiError } from "@/lib/api/errors";

/** G-06's "Lớp" card: the class's own name and description. */
export function ClassSettingsCard({ klass }: { klass: Class }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [name, setName] = useState(klass.name);
  const [description, setDescription] = useState(klass.description ?? "");
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  const save = useMutation({
    mutationFn: () => updateClass(klass.id, { name: name.trim(), description }),
    onSuccess: async () => {
      setError(null);
      setSaved(true);
      await queryClient.invalidateQueries({ queryKey: ["admin-class", klass.id] });
    },
    onError: (cause) => {
      setSaved(false);
      setError(cause instanceof ApiError ? cause.message : t("classDetail.saveFailed"));
    },
  });

  const dirty = name.trim() !== klass.name || description !== (klass.description ?? "");

  return (
    <Card asChild className="gap-0 py-0">
      <section aria-labelledby="class-settings-heading">
        <div className="px-5 pt-4 pb-3">
          <h2
            id="class-settings-heading"
            className="text-[0.9375rem] font-semibold tracking-[-0.01em]"
          >
            {t("classDetail.classSettings")}
          </h2>
        </div>

        <div className="space-y-3 px-5 pb-4">
          <div>
            <Label htmlFor="class-name">{t("classDetail.className")}</Label>
            <Input
              id="class-name"
              value={name}
              className="mt-1.5"
              onChange={(event) => {
                setName(event.target.value);
                setSaved(false);
              }}
            />
          </div>

          <div>
            <Label htmlFor="class-description">
              {t("classDetail.classDescription")}
            </Label>
            <Textarea
              id="class-description"
              value={description}
              className="mt-1.5 min-h-14"
              onChange={(event) => {
                setDescription(event.target.value);
                setSaved(false);
              }}
            />
          </div>

          {error === null ? null : (
            <p role="alert" className="text-destructive text-sm">
              {error}
            </p>
          )}

          <div className="flex items-center gap-3">
            <Button
              size="sm"
              disabled={!dirty || name.trim() === "" || save.isPending}
              onClick={() => save.mutate()}
            >
              {save.isPending ? t("common.saving") : t("common.save")}
            </Button>
            {saved && !dirty ? (
              <span role="status" className="text-muted-foreground text-xs">
                {t("classDetail.savedNote")}
              </span>
            ) : null}
          </div>
        </div>
      </section>
    </Card>
  );
}
