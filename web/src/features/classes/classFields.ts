import type { TFunction } from "i18next";
import type { FormField } from "@/components/shared/form/FormDialog";
import { ApiError, fieldMessages } from "@/lib/api/errors";

/** ClassFieldValues are the words a teacher gives a class in the deck's dialogs. */
export type ClassFieldValues = {
  name: string;
  schedule: string;
  room: string;
};

/** classFields are the New class and Edit class fields: name, schedule and room. */
export function classFields(t: TFunction): readonly FormField<ClassFieldValues>[] {
  return [
    {
      kind: "text",
      name: "name",
      label: t("classes.name"),
      placeholder: t("classes.namePlaceholder"),
      required: true,
      maxLength: 120,
    },
    {
      kind: "text",
      name: "schedule",
      label: t("classes.schedule"),
      placeholder: t("classes.schedulePlaceholder"),
      maxLength: 120,
    },
    {
      kind: "text",
      name: "room",
      label: t("classes.room"),
      placeholder: t("classes.roomPlaceholder"),
      maxLength: 60,
    },
  ];
}

/**
 * saveError is what a class dialog says when its save fails: the first field
 * the server named, else the server's message, else `fallback`.
 */
export function saveError(cause: unknown, fallback: string): string {
  const field = fieldMessages(cause)[0];
  if (field !== undefined) return field;
  return cause instanceof ApiError ? cause.message : fallback;
}

/** blankAsNull trims a field and sends a blank one as none. */
export function blankAsNull(value: string): string | null {
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}
