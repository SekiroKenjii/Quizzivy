import { useMemo, useRef } from "react";
import { useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  addAiCredits,
  ADMIN_CREDITS_KEY,
  approveAiCreditRequest,
  declineAiCreditRequest,
  setAiAllowance,
  updateAiRules,
  type AiAdminCredits,
  type AiAllowanceInput,
  type AiPendingRequest,
  type AiRulesInput,
  type AiTeacherMonth,
} from "@/features/ai-credits/api";
import { credits } from "@/features/ai/credits";
import { failureMessage } from "@/lib/api/errors";
import { useLocale } from "@/lib/i18n/useLocale";
import { notify } from "@/lib/toast";

function patch(client: QueryClient, change: (view: AiAdminCredits) => AiAdminCredits) {
  client.setQueryData<AiAdminCredits>(ADMIN_CREDITS_KEY, (view) =>
    view === undefined ? view : change(view),
  );
}

function reread(client: QueryClient) {
  return client.invalidateQueries({ queryKey: ADMIN_CREDITS_KEY });
}

/**
 * useLatch is a one-at-a-time gate for a write that a render cannot lag:
 * `take` answers false until `release`, so a second click in the same frame
 * as the first, which still reads `isPending` as false, sends nothing.
 */
export function useLatch() {
  const held = useRef(false);
  return useMemo(
    () => ({
      take: () => {
        if (held.current) return false;
        held.current = true;
        return true;
      },
      release: () => {
        held.current = false;
      },
    }),
    [],
  );
}

/**
 * useAddCredits adds credits to this month's pool. The answer is the pool, and
 * it replaces the cached one at once; the month is read again for what depends
 * on it, the projection among them. Every write here is sent at once, online
 * or not, so an offline click fails where it was made instead of landing
 * unseen on reconnect.
 */
export function useAddCredits() {
  const client = useQueryClient();
  return useMutation({
    networkMode: "always",
    mutationFn: addAiCredits,
    onSuccess: (pool) => {
      patch(client, (view) => ({ ...view, pool }));
      void reread(client);
    },
  });
}

/**
 * useSaveAllowance changes one teacher's allowance or pause. The answer is the
 * teacher's month and replaces their row; the month is read again for the
 * sum of allowances and the counts.
 */
export function useSaveAllowance() {
  const client = useQueryClient();
  return useMutation({
    networkMode: "always",
    mutationFn: ({ userId, body }: { userId: string; body: AiAllowanceInput }) =>
      setAiAllowance(userId, body),
    onSuccess: (teacher: AiTeacherMonth) => {
      patch(client, (view) => ({
        ...view,
        teachers: view.teachers.map((row) =>
          row.userId === teacher.userId ? teacher : row,
        ),
      }));
      void reread(client);
    },
  });
}

/**
 * useSaveRules changes the rules named in the body. The answer carries the
 * rules after the change and replaces the cached ones; the month is read
 * again, since a new default allowance rewrote this month's allowances.
 */
export function useSaveRules() {
  const client = useQueryClient();
  return useMutation({
    networkMode: "always",
    mutationFn: (body: AiRulesInput) => updateAiRules(body),
    onSuccess: ({ rules }) => {
      patch(client, (view) => ({ ...view, rules }));
      void reread(client);
    },
  });
}

/**
 * useDecideRequest approves or declines one request, with the toast that says
 * what happened. The request leaves the cached list on success and
 * `onDecided` is called, for a page to put focus where the card was. On a refusal
 * the month is read again, so a request decided elsewhere or from an earlier
 * month leaves the list, and the server's own sentence is shown.
 */
export function useDecideRequest(request: AiPendingRequest, onDecided: () => void) {
  const { t } = useTranslation();
  const locale = useLocale();
  const client = useQueryClient();
  return useMutation({
    networkMode: "always",
    mutationFn: (decision: "approve" | "decline") =>
      decision === "approve"
        ? approveAiCreditRequest(request.id)
        : declineAiCreditRequest(request.id),
    onSuccess: (decided, decision) => {
      patch(client, (view) => ({
        ...view,
        requests: view.requests.filter((row) => row.id !== request.id),
      }));
      void reread(client);
      notify.success(
        decision === "approve"
          ? t("aiCredits.request.approved", {
              name: request.teacherName,
              allowance: credits(decided.allowance, locale),
            })
          : t("aiCredits.request.declined", { name: request.teacherName }),
      );
      onDecided();
    },
    onError: (cause) => {
      void reread(client);
      notify.error(failureMessage(cause, t("aiCredits.request.failed")));
    },
  });
}
