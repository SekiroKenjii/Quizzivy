import { z } from "zod";
import type { paths } from "@/lib/api/schema";

/** AI_REQUEST_MIN is the fewest credits a request may ask for. */
export const AI_REQUEST_MIN = 1000;

/** AI_REQUEST_MAX is the most credits a request may ask for. */
export const AI_REQUEST_MAX = 100_000;

/** AI_REASON_MAX is the longest reason a request may give, in characters. */
export const AI_REASON_MAX = 500;

/**
 * aiCreditRequestSchema is the "Ask for more credits" form: a whole number
 * of credits from 1,000 to 100,000, typed as digits, and a reason of at most
 * 500 characters, trimmed.
 */
export const aiCreditRequestSchema = z.object({
  amount: z
    .string()
    .trim()
    .regex(/^\d+$/, "settings.ai.request.errors.amount")
    .transform(Number)
    .pipe(
      z
        .number()
        .int()
        .min(AI_REQUEST_MIN, "settings.ai.request.errors.amount")
        .max(AI_REQUEST_MAX, "settings.ai.request.errors.amount"),
    ),
  reason: z
    .string()
    .trim()
    .min(1, "settings.ai.request.errors.reasonRequired")
    .max(AI_REASON_MAX, "settings.ai.request.errors.reasonTooLong"),
});

/** AiCreditRequestValues is what the form's fields hold. */
export type AiCreditRequestValues = z.input<typeof aiCreditRequestSchema>;

/** AiCreditRequestOutput is the request the form submits. */
export type AiCreditRequestOutput = z.output<typeof aiCreditRequestSchema>;

type CreditRequestBody =
  paths["/teacher/ai/credit-requests"]["post"]["requestBody"]["content"]["application/json"];
type Expect<T extends true> = T;
type Equal<A, B> =
  (<T>() => T extends A ? 1 : 2) extends <T>() => T extends B ? 1 : 2 ? true : false;

/** AiCreditRequestMatchesTheContract pins the form's output to `POST /teacher/ai/credit-requests`. */
export type AiCreditRequestMatchesTheContract = Expect<
  Equal<AiCreditRequestOutput, CreditRequestBody>
>;
