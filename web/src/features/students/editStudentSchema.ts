import { z } from "zod";

const address = z.email("students.errors.emailInvalid");

/**
 * editStudentSchemaFor is the Edit student form's schema. An address equal to the
 * one the student already has is not judged again, because the server keeps a
 * stored address as it is and a student made before its rules can hold an
 * address the pattern here refuses; any other address is judged in full.
 */
export function editStudentSchemaFor(stored: string) {
  return z.object({
    fullName: z
      .string()
      .trim()
      .min(1, "students.errors.nameRequired")
      .max(200, "students.errors.nameTooLong"),
    email: z
      .string()
      .trim()
      .max(254, "students.errors.emailInvalid")
      .pipe(
        z
          .string()
          .refine(
            (value) => value === stored || address.safeParse(value).success,
            "students.errors.emailInvalid",
          ),
      ),
  });
}

export type EditStudentValues = z.infer<ReturnType<typeof editStudentSchemaFor>>;
