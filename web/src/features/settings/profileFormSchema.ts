import { z } from "zod";
import type { paths } from "@/lib/api/schema";

const PHONE = /^[0-9+ ]{6,20}$/;

const optional = (field: z.ZodString) =>
  z
    .string()
    .transform((value) => value.trim())
    .pipe(z.union([z.literal(""), field]))
    .transform((value) => (value === "" ? null : value));

/**
 * profileFormSchema is the teacher's Profile form: the name, the name
 * students see, the phone, the language and the time zone. A blank name
 * students see or phone is sent as null, which clears it.
 */
export const profileFormSchema = z.object({
  fullName: z
    .string()
    .trim()
    .min(1, "settings.errors.nameRequired")
    .max(200, "settings.errors.nameTooLong"),
  displayName: optional(z.string().max(80, "settings.errors.displayNameTooLong")),
  phone: optional(z.string().regex(PHONE, "settings.errors.phoneInvalid")),
  locale: z.enum(["vi", "en"]),
  timeZone: z.string().min(1).max(64),
});

/** ProfileFormValues is what the Profile form's fields hold. */
export type ProfileFormValues = z.input<typeof profileFormSchema>;

/** ProfileFormOutput is the whole profile the form submits. */
export type ProfileFormOutput = z.output<typeof profileFormSchema>;

type ProfileRequest =
  paths["/auth/me"]["patch"]["requestBody"]["content"]["application/json"];
type Expect<T extends true> = T;
type Equal<A, B> =
  (<T>() => T extends A ? 1 : 2) extends <T>() => T extends B ? 1 : 2 ? true : false;

/** ProfileFormMatchesTheContract pins the form's output to every field of `PATCH /auth/me`. */
export type ProfileFormMatchesTheContract = Expect<
  Equal<ProfileFormOutput, Required<ProfileRequest>>
>;
