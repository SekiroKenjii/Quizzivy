import { FileDrop } from "../FileDrop";
import type { FieldControlProps } from "./types";
/** FileField chooses a file through the shared drop box. */
export function FileField({
  describedBy,
  label: _label,
  ...props
}: FieldControlProps<File | null> &
  Readonly<{
    accept?: string | undefined;
    placeholder?: string | undefined;
    limits?: string | undefined;
  }>) {
  return <FileDrop {...props} aria-describedby={describedBy} />;
}
