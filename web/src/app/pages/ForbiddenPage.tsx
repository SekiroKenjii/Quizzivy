import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { Button } from "@/components/ui/button";

/**
 * §5.4: a `student` reaching `/admin/*` gets a 403 PAGE, not a redirect — "a
 * redirect hides the misconfiguration". The guard that routes here arrives in
 * T-1.10; this is its destination.
 *
 * §12: plain, no shame, no alarm iconography.
 */
export default function ForbiddenPage() {
  const { t } = useTranslation();
  return (
    <main className="mx-auto flex min-h-svh max-w-lg flex-col justify-center gap-6 p-6">
      <div className="space-y-2">
        <h1 className="text-xl font-semibold tracking-tight">{t("forbidden.title")}</h1>
        <p className="text-muted-foreground text-sm leading-relaxed">
          {t("forbidden.body")}
        </p>
      </div>
      <div>
        <Button asChild>
          <Link to="/app">{t("forbidden.action")}</Link>
        </Button>
      </div>
    </main>
  );
}
