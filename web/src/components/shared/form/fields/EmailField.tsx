import { TextField } from "./TextField";
import type { TextControlProps } from "./types";
/** EmailField edits a string with the email input type. */
export function EmailField(props: TextControlProps) {
  return <TextField {...props} type="email" />;
}
