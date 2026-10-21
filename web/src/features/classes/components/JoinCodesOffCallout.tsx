import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { Button } from "@/components/ui/button";
import { Callout } from "@/components/shared/Callout";
import { can } from "@/features/auth/permissions";
import { useJoinCodesOff } from "@/features/organization/api";
import { useAuthStore } from "@/stores/auth";

/**
 * JoinCodesOffCallout says the centre has turned off joining with a code,
 * where a teacher would look for the class's code, and where to turn it on:
 * a link to the sign-in settings for whoever holds `system.settings.write`,
 * and "ask your admin" for anyone else. It shows nothing while the centre
 * allows it.
 */
export function JoinCodesOffCallout() {
  const { t } = useTranslation();
  const off = useJoinCodesOff();
  const mayChange = useAuthStore((s) => can(s.user, "system.settings.write"));
  if (!off) return null;
  return (
    <Callout
      tone="warning"
      size="md"
      actions={
        mayChange ? (
          <Button asChild variant="outline" size="sm">
            <Link to="/admin/settings/sign-in">{t("classes.joinCodesOffOpen")}</Link>
          </Button>
        ) : undefined
      }
    >
      {t("classes.joinCodesOff")}
      {mayChange ? null : <> {t("classes.joinCodesOffAsk")}</>}
    </Callout>
  );
}
