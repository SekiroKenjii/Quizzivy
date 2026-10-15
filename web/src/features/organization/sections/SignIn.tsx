import { Controller, get, useFormContext } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { ChipInput } from "@/components/shared/form/ChipInput";
import { SettingsCard } from "@/components/shared/SettingsCard";
import { SignInImpact } from "@/features/organization/components/SignInImpact";
import { SettingsField } from "@/features/organization/components/SettingsField";
import { controlId, describedBy } from "@/features/organization/controls";
import { SwitchRow } from "@/features/organization/components/SwitchRow";
import { fieldMessage } from "@/features/organization/fieldMessage";
import {
  domainOf,
  type SettingsFormValues,
} from "@/features/organization/settingsFormSchema";
import type { SignInVerdict } from "@/features/organization/signInVerdict";

const SWITCHES = [
  { name: "google", label: "google", hint: "googleHint" },
  { name: "password", label: "password", hint: "passwordHint" },
  { name: "forcePasswordChange", label: "force", hint: "forceHint" },
  { name: "joinCodes", label: "joinCodes", hint: "joinCodesHint" },
] as const;

/**
 * SignInSection is the Sign-in card of System settings: the four switches,
 * Google, email and password, forcing a password change at the first sign-in
 * and joining a class with a code, and the Google domains that may sign in,
 * as chips. A domain is kept lower-case and without a leading `@`; Enter, a
 * comma or a space adds one. Under the card, SignInImpact says how many
 * people the policy being edited would lock out. While `saving` the switches
 * stay focusable and ignore a press.
 */
export function SignInSection({
  verdict,
  saving,
}: Readonly<{ verdict: SignInVerdict; saving: boolean }>) {
  const { t } = useTranslation();
  const form = useFormContext<SettingsFormValues>();
  const domainsId = controlId("signIn.allowedGoogleDomains");
  const message = fieldMessage(
    get(form.formState.errors, "signIn.allowedGoogleDomains"),
    t,
  );
  return (
    <>
      <SettingsCard
        title={t("systemSettings.signIn.title")}
        description={t("systemSettings.signIn.hint")}
      >
        <div className="divide-y">
          {SWITCHES.map(({ name, label, hint }) => (
            <Controller
              key={name}
              control={form.control}
              name={`signIn.${name}`}
              render={({ field }) => (
                <SwitchRow
                  id={controlId(`signIn.${name}`)}
                  label={t(`systemSettings.signIn.${label}`)}
                  hint={t(`systemSettings.signIn.${hint}`)}
                  checked={field.value}
                  paused={saving}
                  onChange={field.onChange}
                />
              )}
            />
          ))}
        </div>
        <div className="border-t px-4.5 pt-3 pb-4">
          <SettingsField
            id={domainsId}
            label={t("systemSettings.signIn.domains")}
            hint={t("systemSettings.signIn.domainsHint")}
            error={message}
          >
            <Controller
              control={form.control}
              name="signIn.allowedGoogleDomains"
              render={({ field }) => (
                <ChipInput
                  id={domainsId}
                  label={t("systemSettings.signIn.domains")}
                  values={field.value}
                  onChange={field.onChange}
                  placeholder={t("systemSettings.signIn.domainsAdd")}
                  removeLabel={(domain) =>
                    t("systemSettings.signIn.domainsRemove", { domain })
                  }
                  resolve={domainOf}
                  spaceAdds
                  invalid={message !== undefined}
                  describedBy={describedBy(domainsId, true, message !== undefined)}
                />
              )}
            />
          </SettingsField>
        </div>
      </SettingsCard>
      <SignInImpact verdict={verdict} />
    </>
  );
}
