import { useTranslation } from "react-i18next";
import { RadioCards } from "@/components/shared/form/RadioCard";
import type { UserDelivery } from "@/features/users/api";

/**
 * DeliveryChoice is how a new password reaches the person: a link to choose
 * one, emailed (the default), or a temporary password shown once to the
 * Admin. While the centre has turned password sign-in off, the server
 * refuses the second with `PASSWORD_SIGN_IN_DISABLED`.
 */
export function DeliveryChoice({
  value,
  onChange,
}: Readonly<{
  value: UserDelivery;
  onChange: (delivery: UserDelivery) => void;
}>) {
  const { t } = useTranslation();
  return (
    <RadioCards
      label={t("users.delivery.label")}
      value={value}
      onChange={(next) =>
        onChange(next === "temporary_password" ? "temporary_password" : "email_link")
      }
      options={[
        {
          value: "email_link",
          label: t("users.delivery.link"),
          hint: t("users.delivery.linkHint"),
        },
        {
          value: "temporary_password",
          label: t("users.delivery.password"),
          hint: t("users.delivery.passwordHint"),
        },
      ]}
    />
  );
}
