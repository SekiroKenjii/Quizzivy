import { useTranslation } from "react-i18next";
import { Link, useLocation, useNavigate } from "react-router";

import { ErrorActions, ErrorScreen } from "@/app/pages/ErrorScreen";
import { NotFoundArt } from "@/app/pages/errorArt";
import { Button } from "@/components/ui/button";
import { homePathFor } from "@/features/auth/home";
import { useAuthStore } from "@/stores/auth";

/**
 * NotFoundPage is the one 404: the router's catch-all and a route that answered
 * 404 both render it. It shows the path that failed, and "Go back" leaves the
 * app only when there is nowhere in it to go back to.
 */
export default function NotFoundPage() {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const user = useAuthStore((s) => s.user);
  const home = user ? homePathFor(user) : "/login";

  return (
    <ErrorScreen
      art={<NotFoundArt />}
      title={t("notFound.title")}
      body={t("notFound.body")}
      footer={t("notFound.footnote")}
    >
      <div className="bg-muted mt-4 rounded-md px-3 py-2 text-base">
        <code className="text-muted-fg text-meta font-mono [overflow-wrap:anywhere]">
          {location.pathname}
        </code>
      </div>

      <ErrorActions>
        <Button asChild>
          <Link to={home}>{t("notFound.action")}</Link>
        </Button>
        <Button
          variant="outline"
          onClick={() =>
            void (location.key === "default" ? navigate(home) : navigate(-1))
          }
        >
          {t("notFound.back")}
        </Button>
      </ErrorActions>
    </ErrorScreen>
  );
}
