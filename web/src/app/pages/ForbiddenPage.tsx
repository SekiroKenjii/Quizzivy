import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { Avatar } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { ErrorActions, ErrorScreen } from "@/app/pages/ErrorScreen";
import { ForbiddenArt } from "@/app/pages/errorArt";
import { homePathFor } from "@/features/auth/home";
import { useLogout } from "@/features/auth/useSession";
import { useAuthStore } from "@/stores/auth";

/**
 * §5.4: a `student` reaching `/admin/*` gets a 403 PAGE, not a redirect — "a
 * redirect hides the misconfiguration".
 */
export default function ForbiddenPage() {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const logout = useLogout();

  return (
    <ErrorScreen
      art={<ForbiddenArt />}
      title={t("forbidden.title")}
      body={t("forbidden.body")}
      footer={t("forbidden.footnote")}
    >
      {user ? (
        <div className="mt-4 flex items-center gap-2.5 rounded-md border p-3">
          <Avatar name={user.fullName} size="sm" />
          <div className="min-w-0">
            <p className="text-muted-foreground text-xs">{t("forbidden.signedInAs")}</p>
            <p className="truncate text-sm">{user.email}</p>
          </div>
        </div>
      ) : null}

      <ErrorActions>
        <Button asChild>
          <Link to={homePathFor(user)}>{t("forbidden.action")}</Link>
        </Button>
        {/* "Sign in with another account", not "Sign out". */}
        <Button variant="outline" onClick={() => void logout()}>
          {t("forbidden.switchAccount")}
        </Button>
      </ErrorActions>
    </ErrorScreen>
  );
}
