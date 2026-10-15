import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  signInImpactQuery,
  type SignInImpact,
  type SignInPolicy,
} from "@/features/organization/api";
import { domainsValid, samePolicy } from "@/features/organization/settingsFormSchema";
import { useDebounced } from "@/lib/useDebounced";

const ASK_AFTER_MS = 400;

/**
 * SignInRefusal is the answer of a save the server refused with
 * `SIGN_IN_POLICY_LOCKOUT`: the policy it judged and the message it gave.
 */
export type SignInRefusal = Readonly<{ policy: SignInPolicy; message: string }>;

/**
 * SignInVerdict is what the settings page knows about the sign-in policy
 * being edited. `impact` is the server's count for exactly this policy, and
 * only once it is asked and answered. `reason` is the rule the policy breaks,
 * in words, and is set whenever a save would be refused: no method is on, the
 * server's count says the caller or every Admin would be locked out, or the
 * server has just refused this very policy.
 */
export type SignInVerdict = Readonly<{
  impact: SignInImpact | undefined;
  reason: string | null;
}>;

/**
 * useSignInVerdict judges `policy` against `saved`. While they differ it
 * asks the server what the policy would do, a moment after the last edit,
 * and it asks about nothing it knows to be refused: a policy with no method
 * on or a domain the server would not take. A policy equal to the saved one
 * is never asked about.
 */
export function useSignInVerdict(
  saved: SignInPolicy,
  policy: SignInPolicy,
  refusal: SignInRefusal | null,
): SignInVerdict {
  const { t } = useTranslation();
  const settledKey = useDebounced(JSON.stringify(policy), ASK_AFTER_MS);
  const settled = useMemo(() => JSON.parse(settledKey) as SignInPolicy, [settledKey]);
  const noMethod = !policy.google && !policy.password;
  const asking =
    samePolicy(settled, policy) &&
    !samePolicy(policy, saved) &&
    !noMethod &&
    domainsValid(policy);
  const answer = useQuery({ ...signInImpactQuery(settled), enabled: asking }).data;
  const impact = asking ? answer : undefined;

  if (noMethod)
    return { impact: undefined, reason: t("systemSettings.signIn.noMethod") };
  if (impact?.lockout === true)
    return {
      impact,
      reason: t(
        impact.actorLocked
          ? "systemSettings.signIn.lockedYou"
          : "systemSettings.signIn.lockedAdmins",
      ),
    };
  const refused = refusal !== null && samePolicy(refusal.policy, policy);
  return { impact, reason: refused ? refusal.message : null };
}
