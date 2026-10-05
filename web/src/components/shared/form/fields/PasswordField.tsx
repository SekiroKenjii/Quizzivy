import { TextField } from "./TextField";
import type { TextControlProps } from "./types";
/** PasswordField edits a string with the password input type. */
export function PasswordField(props: TextControlProps) {
  return <TextField {...props} type="password" />;
}
