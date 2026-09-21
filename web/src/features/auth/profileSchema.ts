import { z } from "zod";
import type { paths } from "@/lib/api/schema";

/** Form input validation for the "Hồ sơ" card's one editable field (S-10, S-17). */
export const profileSchema = z.object({
  fullName: z
    .string()
    .trim()
    .min(1, "settings.errors.nameRequired")
    .max(200, "settings.errors.nameTooLong"),
});

export type ProfileValues = z.infer<typeof profileSchema>;

type ProfileRequest =
  paths["/auth/me"]["patch"]["requestBody"]["content"]["application/json"];

type Expect<T extends true> = T;
type Equal<A, B> =
  (<T>() => T extends A ? 1 : 2) extends <T>() => T extends B ? 1 : 2 ? true : false;

// Exported so `noUnusedLocals` does not remove the only thing keeping the form
// and the contract in step. Nothing reads it; `tsc -b` checking it is the job.
export type ProfileValuesMatchTheContract = Expect<
  Equal<ProfileValues, ProfileRequest>
>;
