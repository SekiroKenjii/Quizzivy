import { queryOptions } from "@tanstack/react-query";
import { isAiOff } from "@/features/ai/api";
import { api } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { components } from "@/lib/api/schema";

type Schemas = components["schemas"];

/** AiAdminCredits is the Admin's view of the month: the pool, the teachers, the requests and the rules. */
export type AiAdminCredits = Schemas["AiAdminCredits"];
/** AiPool is the centre's month of credits. */
export type AiPool = Schemas["AiPool"];
/** AiRules is what applies to every teacher without a custom allowance. */
export type AiRules = Schemas["AiRules"];
/** AiRulesInput is the body of `PATCH /admin/ai/rules`. */
export type AiRulesInput = Schemas["AiRulesInput"];
/** AiTeacherMonth is one teacher's month on the Admin page. */
export type AiTeacherMonth = Schemas["AiTeacherMonth"];
/** AiTeacherStatus is where a teacher stands against their allowance. */
export type AiTeacherStatus = Schemas["AiTeacherStatus"];
/** AiAllowanceInput is the body of `PUT /admin/ai/teachers/{userId}/allowance`. */
export type AiAllowanceInput = Schemas["AiAllowanceInput"];
/** AiPendingRequest is a request waiting for an Admin. */
export type AiPendingRequest = Schemas["AiPendingRequest"];

/** ADMIN_CREDITS_KEY is the query key of the Admin's month of AI credits, shared by the page and the sidebar's badge. */
export const ADMIN_CREDITS_KEY = ["admin", "ai-credits"] as const;

/**
 * adminCreditsQuery reads `GET /admin/ai/credits`. A server without an
 * assistant answers 501 at once and is not asked again, so the page can say
 * so; any other failure follows the app's default of two retries for a server
 * fault and none for a refusal.
 */
export function adminCreditsQuery() {
  return queryOptions({
    queryKey: ADMIN_CREDITS_KEY,
    queryFn: ({ signal }) => api("get", "/admin/ai/credits", { signal }),
    retry: (failureCount, cause) => {
      if (isAiOff(cause)) return false;
      if (cause instanceof ApiError && cause.status >= 400 && cause.status < 500)
        return false;
      return failureCount < 2;
    },
  });
}

/** addAiCredits adds credits to this month's pool and records the caller as the approver. */
export function addAiCredits(credits: number) {
  return api("post", "/admin/ai/credits/additions", { body: { credits } });
}

/** updateAiRules changes the supplied rules and keeps the others. */
export function updateAiRules(body: AiRulesInput) {
  return api("patch", "/admin/ai/rules", { body });
}

/** setAiAllowance changes how much one teacher may use this month, or pauses or resumes them. */
export function setAiAllowance(userId: string, body: AiAllowanceInput) {
  return api("put", "/admin/ai/teachers/{userId}/allowance", {
    path: { userId },
    body,
  });
}

/** approveAiCreditRequest raises the teacher's allowance by the request's amount. */
export function approveAiCreditRequest(id: string) {
  return api("post", "/admin/ai/credit-requests/{id}/approve", { path: { id } });
}

/** declineAiCreditRequest closes the request without raising the allowance. */
export function declineAiCreditRequest(id: string) {
  return api("post", "/admin/ai/credit-requests/{id}/decline", { path: { id } });
}
