import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { Ban } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ErrorScreen } from "@/app/pages/ErrorScreen";

/**
 * §5.4: a `student` reaching `/admin/*` gets a 403 PAGE, not a redirect — "a
 * redirect hides the misconfiguration". The deck's S-11 gives it a muted glyph
 * rather than an alarm one: the account is in the wrong place, not in trouble.
 */
export default function ForbiddenPage() {
  const { t } = useTranslation();
  return (
    <ErrorScreen
      icon={<Ban className="text-muted-foreground mx-auto size-8" aria-hidden="true" />}
      title={t("forbidden.title")}
      body={t("forbidden.body")}
    >
      <Button asChild className="mt-5">
        <Link to="/app">{t("forbidden.action")}</Link>
      </Button>
    </ErrorScreen>
  );
}
