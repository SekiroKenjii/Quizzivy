import { z } from "zod";

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
  newPassword: z.string().min(8, "changePassword.errors.tooShort"),
});

export type ChangePasswordValues = z.infer<typeof changePasswordSchema>;
