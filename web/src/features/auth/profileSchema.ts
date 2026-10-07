import { z } from "zod";
import type { paths } from "@/lib/api/schema";

/** profileSchema keeps the current settings form's required localized name. */
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

/** ProfileValuesMatchTheContract pins the existing required-name form field. */
export type ProfileValuesMatchTheContract = Expect<
  Equal<ProfileValues, Required<Pick<ProfileRequest, "fullName">>>
>;

const patchName = (limit: number) =>
  z
    .string()
    .refine((value) => [...value].length >= 1 && [...value].length <= limit)
    .refine(
      (value) => [...value.trim()].length >= 1 && [...value.trim()].length <= limit,
    );

/** profilePatchSchema validates the complete closed partial profile request. */
export const profilePatchSchema = z
  .object({
    fullName: patchName(200).optional(),
    displayName: patchName(80).nullable().optional(),
    phone: z
      .string()
      .regex(/^[0-9+ ]{6,20}$/)
      .nullable()
      .optional(),
    locale: z.enum(["vi", "en"]).optional(),
    timeZone: z
      .string()
      .min(1)
      .max(64)
      .refine((value) => value.trim().length >= 1 && value !== "Local")
      .optional(),
  })
  .strict()
  .refine((value) => Object.values(value).some((field) => field !== undefined))
  .transform((value) => ({
    ...(value.fullName === undefined ? {} : { fullName: value.fullName }),
    ...(value.displayName === undefined ? {} : { displayName: value.displayName }),
    ...(value.phone === undefined ? {} : { phone: value.phone }),
    ...(value.locale === undefined ? {} : { locale: value.locale }),
    ...(value.timeZone === undefined ? {} : { timeZone: value.timeZone }),
  }));

export type ProfilePatchValues = z.infer<typeof profilePatchSchema>;

/** ProfilePatchValuesMatchTheContract pins every optional patch property. */
export type ProfilePatchValuesMatchTheContract = Expect<
  Equal<ProfilePatchValues, ProfileRequest>
>;
