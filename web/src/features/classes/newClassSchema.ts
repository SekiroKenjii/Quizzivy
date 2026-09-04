import { z } from "zod";

export const newClassSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, "classes.errors.nameRequired")
    .max(120, "classes.errors.nameTooLong"),
  description: z.string(),
  selfJoinEnabled: z.boolean(),
});

export type NewClassValues = z.infer<typeof newClassSchema>;
