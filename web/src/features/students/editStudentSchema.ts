import { z } from "zod";

export const editStudentSchema = z.object({
  fullName: z
    .string()
    .trim()
    .min(1, "students.errors.nameRequired")
    .max(200, "students.errors.nameTooLong"),
  email: z
    .string()
    .trim()
    .email("students.errors.emailInvalid")
    .max(254, "students.errors.emailInvalid"),
});

export type EditStudentValues = z.infer<typeof editStudentSchema>;
