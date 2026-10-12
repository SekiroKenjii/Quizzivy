import { useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { updateClass, type Class } from "@/features/classes/api";
import { blankAsNull, saveError } from "@/features/classes/classFields";
import { invalidateClass } from "@/features/classes/invalidate";

type Words = { name: string; description: string; schedule: string; room: string };

function wordsOf(klass: Class): Words {
  return {
    name: klass.name,
    description: klass.description ?? "",
    schedule: klass.scheduleLabel ?? "",
    room: klass.room ?? "",
  };
}

function same(a: Words, b: Words): boolean {
  return (
    a.name.trim() === b.name.trim() &&
    a.description === b.description &&
    a.schedule.trim() === b.schedule.trim() &&
    a.room.trim() === b.room.trim()
  );
}

/**
 * ClassSettingsCard is the class's own words, as the deck's "Class" card
 * draws them, with the schedule and room T-R4.18 added: Name, Description,
 * Schedule and Room, and Save, active while something changed and the name
 * is not blank. A save says "Saved. Students see the new name." Without
 * `canWrite` the fields only read.
 */
export function ClassSettingsCard({
  klass,
  canWrite,
}: Readonly<{ klass: Class; canWrite: boolean }>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const prefix = useId();
  const [words, setWords] = useState<Words>(() => wordsOf(klass));
  const [saved, setSaved] = useState(false);
  const stored = wordsOf(klass);
  const dirty = !same(words, stored);
  const ready = canWrite && dirty && words.name.trim() !== "";

  const save = useMutation({
    mutationFn: (next: Words) =>
      updateClass(klass.id, {
        name: next.name.trim(),
        description: next.description,
        scheduleLabel: blankAsNull(next.schedule),
        room: blankAsNull(next.room),
      }),
    onSuccess: async () => {
      setSaved(true);
      await invalidateClass(queryClient, klass.id);
    },
  });

  const edit = (key: keyof Words) => (value: string) => {
    setSaved(false);
    setWords((current) => ({ ...current, [key]: value }));
  };
  const field = (key: keyof Words) => `${prefix}-${key}`;

  return (
    <section
      aria-labelledby={`${prefix}-heading`}
      className="bg-card shadow-card rounded-xl border"
    >
      <div className="px-4.5 pt-4 pb-3">
        <h2
          id={`${prefix}-heading`}
          className="text-[0.9375rem] font-semibold tracking-[-0.01em]"
        >
          {t("classDetail.classSettings")}
        </h2>
      </div>
      <form
        className="flex flex-col gap-3 px-4.5 pb-4"
        onSubmit={(event) => {
          event.preventDefault();
          if (ready && !save.isPending) save.mutate(words);
        }}
      >
        <Label htmlFor={field("name")}>{t("classDetail.className")}</Label>
        <Input
          id={field("name")}
          value={words.name}
          maxLength={120}
          readOnly={!canWrite}
          className="-mt-1.5"
          onChange={(event) => edit("name")(event.target.value)}
        />
        <Label htmlFor={field("description")}>
          {t("classDetail.classDescription")}
        </Label>
        <Textarea
          id={field("description")}
          value={words.description}
          rows={3}
          readOnly={!canWrite}
          className="-mt-1.5 min-h-18"
          onChange={(event) => edit("description")(event.target.value)}
        />
        <Label htmlFor={field("schedule")}>{t("classes.schedule")}</Label>
        <Input
          id={field("schedule")}
          value={words.schedule}
          maxLength={120}
          readOnly={!canWrite}
          placeholder={t("classes.schedulePlaceholder")}
          className="-mt-1.5"
          onChange={(event) => edit("schedule")(event.target.value)}
        />
        <Label htmlFor={field("room")}>{t("classes.room")}</Label>
        <Input
          id={field("room")}
          value={words.room}
          maxLength={60}
          readOnly={!canWrite}
          placeholder={t("classes.roomPlaceholder")}
          className="-mt-1.5"
          onChange={(event) => edit("room")(event.target.value)}
        />
        {save.isError ? (
          <p role="alert" className="text-danger-ink text-xs">
            {saveError(save.error, t("classDetail.saveFailed"))}
          </p>
        ) : null}
        {canWrite ? (
          <div className="flex items-center gap-2.5">
            <Button
              type="submit"
              className="h-8.5 px-3.5 in-data-[scale=deck]:h-8.5 aria-disabled:opacity-50"
              aria-disabled={!ready || save.isPending || undefined}
            >
              {t("common.save")}
            </Button>
            {saved && !dirty ? (
              <span role="status" className="text-muted-fg text-xs">
                {t("classDetail.savedNote")}
              </span>
            ) : null}
          </div>
        ) : null}
      </form>
    </section>
  );
}

function Label({ htmlFor, children }: Readonly<{ htmlFor: string; children: string }>) {
  return (
    <label htmlFor={htmlFor} className="text-meta font-medium">
      {children}
    </label>
  );
}
