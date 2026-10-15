import { useTranslation } from "react-i18next";
import { Callout } from "@/components/shared/Callout";
import type { SignInVerdict } from "@/features/organization/signInVerdict";

/**
 * SignInImpact says what the sign-in policy being edited would do: how many
 * people would no longer be able to sign in, as a warning, and, when a save
 * would be refused, the rule it breaks, as a danger note that the page's
 * Save changes also gives as its reason. It draws nothing for a policy that
 * costs no one a way in.
 */
export function SignInImpact({ verdict }: Readonly<{ verdict: SignInVerdict }>) {
  const { t } = useTranslation();
  const { impact, reason } = verdict;
  const people = impact?.affectedUsers ?? 0;
  const admins = impact?.affectedAdmins ?? 0;
  if (people === 0 && reason === null) return null;
  return (
    <Callout
      tone={reason === null ? "warning" : "danger"}
      size="md"
      announce={reason !== null}
      className="[&_p+p]:mt-1"
    >
      {people > 0 ? (
        <p>
          <strong className="font-semibold">
            {t("systemSettings.signIn.impact", { count: people })}
          </strong>
          {admins > 0 ? (
            <> {t("systemSettings.signIn.impactAdmins", { count: admins })}</>
          ) : null}
        </p>
      ) : null}
      {reason === null ? null : <p>{reason}</p>}
    </Callout>
  );
}
