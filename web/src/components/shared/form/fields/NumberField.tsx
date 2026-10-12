import { TextField } from "./TextField";
import type { TextControlProps } from "./types";
/** NumberField edits a string with the number input type. */
export function NumberField(props: TextControlProps) {
  return <TextField {...props} type="number" />;
}
