import type { ReactNode, HTMLInputTypeAttribute } from "react";
import type { LucideIcon } from "lucide-react";

/** FormValue is a controlled form value that its field can edit. */
export type FormValue = string | boolean | readonly string[] | File | null;
/** FormValues is a form's named collection of editable values. */
export type FormValues = Record<string, FormValue>;
/** FormOption is a named choice with optional searchable secondary text. */
export type FormOption = Readonly<{ value: string; label: string; meta?: string }>;
type Keys<V, T> = { [K in keyof V & string]: V[K] extends T ? K : never }[keyof V &
  string];
type Common<V> = {
  label?: string;
  hint?: string;
  required?: boolean;
  requiredText?: string;
  noOptional?: boolean;
  when?: (values: V) => boolean;
};
type Named<V, T> = Common<V> & { name: Keys<V, T> };
type Text<V> = Named<V, string> & {
  kind: "text" | "email" | "number" | "password";
  placeholder?: string;
  autoComplete?: string;
  inputMode?:
    "text" | "email" | "numeric" | "decimal" | "tel" | "search" | "url" | "none";
  maxLength?: number;
};
type Choice<V> = Named<V, string> & {
  kind: "select" | "seg";
  options: readonly FormOption[];
  placeholder?: string;
};
type Multiple<V> = Named<V, readonly string[]> & {
  kind: "chips" | "list";
  options: readonly FormOption[];
  search?: boolean;
  searchPlaceholder?: string;
};
/** FormInfoRow is an informational line with an optional independent action. */
export type FormInfoRow = Readonly<{
  icon: LucideIcon;
  tone?: "neutral" | "success" | "warning" | "danger" | "info";
  text: ReactNode;
  action?: { label: string; onAction: () => void };
}>;
/** FormField ties each field kind to a compatible key of the form's values. */
export type FormField<V extends FormValues> =
  | Text<V>
  | Choice<V>
  | Multiple<V>
  | (Named<V, string> & { kind: "area"; placeholder?: string; maxLength?: number })
  | (Named<V, string> & { kind: "date" | "time"; minuteStep?: number })
  | (Named<V, boolean> & { kind: "toggle"; text: string; sub?: string })
  | (Named<V, File | null> & {
      kind: "file";
      accept?: string;
      placeholder?: string;
      limits?: string;
    })
  | (Common<V> & {
      kind: "info";
      name: keyof V & string;
      rows: readonly FormInfoRow[] | ((values: V) => readonly FormInfoRow[]);
    })
  | (Common<V> & {
      kind: "qr";
      name: keyof V & string;
      payload: string;
      value: string;
      text: string;
    });
/** FieldControlProps carries a field's control value and accessible validation state. */
export type FieldControlProps<T> = Readonly<{
  id: string;
  label: string;
  value: T;
  onChange: (next: T) => void;
  invalid?: boolean | undefined;
  describedBy?: string | undefined;
  disabled?: boolean | undefined;
}>;
/** TextControlProps configures one textual form control. */
export type TextControlProps = FieldControlProps<string> &
  Readonly<{
    type?: HTMLInputTypeAttribute;
    placeholder?: string | undefined;
    autoComplete?: string | undefined;
    inputMode?: Text<Virtual>["inputMode"] | undefined;
    maxLength?: number | undefined;
  }>;
type Virtual = { value: string };
