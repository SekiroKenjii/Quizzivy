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
  hint?: string | ((values: V) => string | undefined);
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
  options: readonly FormOption[] | ((values: V) => readonly FormOption[]);
  search?: boolean;
  searchPlaceholder?: string;
};
/** FormIconOption is an icon a form offers, drawn as a square button named by its label. */
export type FormIconOption = Readonly<{
  value: string;
  label: string;
  icon: LucideIcon;
}>;
/**
 * FormColorOption is a swatch a form offers, named by its label. `swatch` is
 * the classes that paint the circle and the tick on it, token classes only.
 */
export type FormColorOption = Readonly<{
  value: string;
  label: string;
  swatch: string;
}>;
/**
 * FormInfoRow is an informational line with an optional independent action.
 * An action that opens or closes something sets `expanded`, which the button
 * announces; `controls` names what it opens.
 */
export type FormInfoRow = Readonly<{
  icon: LucideIcon;
  tone?: "neutral" | "success" | "warning" | "danger" | "info";
  text: ReactNode;
  action?: {
    label: string;
    onAction: () => void;
    expanded?: boolean;
    controls?: string;
  };
}>;
/**
 * FormField ties each field kind to a compatible key of the form's values. A
 * field's hint, and a list's or chips' options, may be a function of the
 * current values.
 */
export type FormField<V extends FormValues> =
  | Text<V>
  | Choice<V>
  | Multiple<V>
  | (Named<V, string> & { kind: "area"; placeholder?: string; maxLength?: number })
  | (Named<V, string> & { kind: "date" | "time"; minuteStep?: number })
  | (Named<V, string> & { kind: "icons"; options: readonly FormIconOption[] })
  | (Named<V, string> & { kind: "colors"; options: readonly FormColorOption[] })
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
    readOnly?: boolean | undefined;
  }>;
type Virtual = { value: string };
