import { useTranslation } from "react-i18next";
import { Callout } from "@/components/shared/Callout";
import { useJoinCodesOff } from "@/features/organization/api";

/**
 * JoinCodesOffCallout says the centre has turned off joining with a code,
 * where a teacher would look for the class's code. It shows nothing while
 * the centre allows it.
 */
export function JoinCodesOffCallout() {
  const { t } = useTranslation();
  const off = useJoinCodesOff();
  if (!off) return null;
  return (
    <Callout tone="warning" size="md">
      {t("classes.joinCodesOff")}
    </Callout>
  );
}
