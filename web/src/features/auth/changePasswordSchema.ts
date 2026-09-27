import { z } from "zod";

import { passwordRules } from "@/lib/password";

/**
 * newPasswordSchema is the contract's rules for a new password: at least
 * eight characters, and a number or a symbol. That it differs from the old
 * one only the server can tell.
 */
export const newPasswordSchema = z
  .string()
  .refine((value) => passwordRules(value).length, "changePassword.errors.tooShort")
  .refine(
    (value) => passwordRules(value).numberOrSymbol,
    "changePassword.errors.numberOrSymbol",
  );

/**
 * Form validation for §5.4's password change.
 *
 * `currentPassword` may be empty: while `mustChangePassword` is set the server
 * does not ask for it, and `changePassword` in api.ts drops an empty one from
 * the request. That mapping is why this schema is not asserted equal to the
 * contract's body the way loginSchema is.
 */
export const changePasswordSchema = z.object({
  currentPassword: z.string(),
  newPassword: newPasswordSchema,
});

export type ChangePasswordValues = z.infer<typeof changePasswordSchema>;
