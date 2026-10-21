import { z } from "zod";

import { newPasswordSchema } from "@/features/auth/changePasswordSchema";
import type { paths } from "@/lib/api/schema";

/** MAX_LINK_TOKEN_LENGTH is the longest token the contract accepts from a link. */
export const MAX_LINK_TOKEN_LENGTH = 128;

const MAX_PASSWORD_LENGTH = 512;

/**
 * setPasswordSchema validates what the set-password page sends: the token the
 * link carried and a new password under change-password's rules, at most 512
 * characters as the contract counts them. Its messages are i18n keys.
 */
export const setPasswordSchema = z.object({
  token: z.string().min(1).max(MAX_LINK_TOKEN_LENGTH),
  newPassword: newPasswordSchema.refine(
    (value) => [...value].length <= MAX_PASSWORD_LENGTH,
    "setPassword.errors.tooLong",
  ),
});

/** SetPasswordValues is the body `setPasswordWithLink` takes. */
export type SetPasswordValues = z.infer<typeof setPasswordSchema>;

type SetPasswordRequest =
  paths["/auth/set-password"]["post"]["requestBody"]["content"]["application/json"];

type Expect<T extends true> = T;
type Equal<A, B> =
  (<T>() => T extends A ? 1 : 2) extends <T>() => T extends B ? 1 : 2 ? true : false;

/**
 * SetPasswordValuesMatchTheContract fails `tsc -b` when the form and the
 * contract disagree about the request body. Nothing reads it; it is exported
 * so `noUnusedLocals` keeps it.
 */
export type SetPasswordValuesMatchTheContract = Expect<
  Equal<SetPasswordValues, SetPasswordRequest>
>;
