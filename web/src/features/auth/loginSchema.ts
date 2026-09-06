import { z } from "zod";
import type { paths } from "@/lib/api/schema";

/** Form input validation for §5.1's sign-in. */
export const loginSchema = z.object({
  email: z
    .string()
    .min(1, "login.errors.emailRequired")
    .pipe(z.email("login.errors.emailInvalid")),
  password: z.string().min(1, "login.errors.passwordRequired"),
});

export type LoginValues = z.infer<typeof loginSchema>;

/**
 * Fails `tsc` if the form and the contract disagree about the request body.
 *
 * Note the direction: the schema must produce something the endpoint ACCEPTS.
 * A field the contract does not have, or a type mismatch, is a compile error
 * here rather than a 400 in front of a student.
 */
type LoginRequest =
  paths["/auth/login"]["post"]["requestBody"]["content"]["application/json"];

type Expect<T extends true> = T;
type Equal<A, B> =
  (<T>() => T extends A ? 1 : 2) extends <T>() => T extends B ? 1 : 2 ? true : false;

// Exported so `noUnusedLocals` does not remove the only thing keeping the form
// and the contract in step. Nothing reads it; `tsc -b` checking it is the job.
export type LoginValuesMatchTheContract = Expect<Equal<LoginValues, LoginRequest>>;
