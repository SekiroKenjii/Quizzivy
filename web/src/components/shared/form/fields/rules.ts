import { fold } from "@/lib/fold";
import type { FormField, FormOption, FormValue, FormValues } from "./types";

/** visibleFields retains fields whose condition permits them for the current values. */
export function visibleFields<V extends FormValues>(
  fields: readonly FormField<V>[],
  values: V,
): FormField<V>[] {
  return fields.filter((field) => field.when?.(values) !== false);
}
/** missingValue detects an empty required control value after trimming text. */
export function missingValue(value: FormValue): boolean {
  if (typeof value === "string") return value.trim() === "";
  if (Array.isArray(value)) return value.length === 0;
  return value === null || value === false;
}
/** fieldErrors combines required and caller errors only for visible editable fields. */
export function fieldErrors<V extends FormValues>(
  fields: readonly FormField<V>[],
  values: V,
  required: string,
  extra: Partial<Record<keyof V & string, string>> | null = null,
): Partial<Record<keyof V & string, string>> {
  const errors: Partial<Record<keyof V & string, string>> = {};
  for (const field of visibleFields(fields, values)) {
    if (field.kind === "info" || field.kind === "qr") continue;
    const message = extra?.[field.name];
    if (message) errors[field.name] = message;
    if (field.required && missingValue(values[field.name]!))
      errors[field.name] = field.requiredText ?? required;
  }
  return errors;
}
/** matchesOption finds folded label or secondary text matches for a list query. */
export function matchesOption(option: FormOption, query: string): boolean {
  return fold(`${option.label} ${option.meta ?? ""}`).includes(fold(query.trim()));
}
/** optionalField identifies the deck's optional label kinds. */
export function optionalField<V extends FormValues>(field: FormField<V>): boolean {
  return (
    !field.required &&
    !field.noOptional &&
    ["text", "email", "date", "time", "area", "file"].includes(field.kind)
  );
}
