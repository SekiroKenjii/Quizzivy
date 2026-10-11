import { TextField } from "./TextField";
import { EmailField } from "./EmailField";
import { NumberField } from "./NumberField";
import { PasswordField } from "./PasswordField";
import { AreaField } from "./AreaField";
import { DateField } from "./DateField";
import { TimeField } from "./TimeField";
import { SelectField } from "./SelectField";
import { SegField } from "./SegField";
import { ChipsField } from "./ChipsField";
import { ListField } from "./ListField";
import { ToggleField } from "./ToggleField";
import { FileField } from "./FileField";
import { InfoField } from "./InfoField";
import { QrField } from "./QrField";
import type { FormField, FormValues, FormValue } from "./types";
const TEXT = {
  text: TextField,
  email: EmailField,
  number: NumberField,
  password: PasswordField,
};
const SINGLE = { select: SelectField, seg: SegField };
const MANY = { chips: ChipsField, list: ListField };
const DATE = { date: DateField, time: TimeField };
/**
 * FieldControl renders the matching typed control for one form field. While
 * `locked`, a text or multiline field, and a list's search box, turn
 * read-only and stay focusable, since Enter in one of them may have
 * submitted the form, and every other control is disabled.
 */
export function FieldControl<V extends FormValues>({
  field,
  values,
  onChange,
  id,
  invalid,
  describedBy,
  disabled,
  locked = false,
}: Readonly<{
  field: FormField<V>;
  values: V;
  onChange: (next: FormValue) => void;
  id: string;
  invalid: boolean;
  describedBy: string | undefined;
  disabled: boolean;
  locked?: boolean;
}>) {
  const raw = values[field.name];
  const label = field.label ?? "";
  const props = {
    id,
    label,
    invalid,
    describedBy,
    disabled: disabled || locked,
    onChange,
  };
  const textProps = { ...props, disabled, readOnly: locked };
  const text = typeof raw === "string" ? raw : "";
  const multiple = Array.isArray(raw) ? raw : [];
  switch (field.kind) {
    case "text":
    case "email":
    case "number":
    case "password": {
      const Control = TEXT[field.kind];
      return (
        <Control
          {...textProps}
          placeholder={field.placeholder}
          autoComplete={field.autoComplete}
          inputMode={field.inputMode}
          maxLength={field.maxLength}
          value={text}
        />
      );
    }
    case "select":
    case "seg": {
      const Control = SINGLE[field.kind];
      return (
        <Control
          {...props}
          options={field.options}
          {...(field.kind === "select" ? { placeholder: field.placeholder } : {})}
          value={text}
        />
      );
    }
    case "list":
    case "chips": {
      const Control = MANY[field.kind];
      return (
        <Control
          {...props}
          options={field.options}
          {...(field.kind === "list"
            ? {
                search: field.search,
                searchPlaceholder: field.searchPlaceholder,
                readOnly: locked,
              }
            : {})}
          value={multiple}
        />
      );
    }
    case "date":
    case "time": {
      const Control = DATE[field.kind];
      return <Control {...props} minuteStep={field.minuteStep} value={text} />;
    }
    case "area":
      return (
        <AreaField
          {...textProps}
          placeholder={field.placeholder}
          maxLength={field.maxLength}
          value={text}
        />
      );
    case "toggle":
      return (
        <ToggleField
          {...props}
          text={field.text}
          sub={field.sub}
          value={raw === true}
        />
      );
    case "file":
      return (
        <FileField
          {...props}
          accept={field.accept}
          placeholder={field.placeholder}
          limits={field.limits}
          value={raw instanceof File ? raw : null}
        />
      );
    case "info":
      return (
        <InfoField
          rows={typeof field.rows === "function" ? field.rows(values) : field.rows}
        />
      );
    case "qr":
      return <QrField payload={field.payload} value={field.value} text={field.text} />;
  }
  return null;
}
