import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import { ErrorActions, ErrorScreen } from "@/app/pages/ErrorScreen";
import { ForbiddenArt } from "@/app/pages/errorArt";
import { Avatar } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { homePathFor } from "@/features/auth/home";
import { useLogout } from "@/features/auth/useSession";
import { useAuthStore } from "@/stores/auth";

/**
 * ForbiddenPage is the 403 (§5.4): a page, never a redirect, because a
 * redirect hides the misconfiguration. It names the account being refused and
 * offers another one; a signed-out visitor is offered sign-in instead.
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
      {user && (
        <div className="mt-4 flex items-center gap-2.5 rounded-lg border px-3 py-2.5">
          <Avatar name={user.fullName} size="30" />
          <span className="min-w-0 flex-1">
            <span className="text-ui block truncate font-medium">{user.fullName}</span>
            <span className="text-muted-fg text-meta block">
              {t("forbidden.signedIn")}
            </span>
          </span>
        </div>
      )}

      <ErrorActions>
        {user ? (
          <>
            <Button asChild>
              <Link to={homePathFor(user)}>{t("forbidden.home")}</Link>
            </Button>
            <Button variant="outline" onClick={() => void logout()}>
              {t("forbidden.switchAccount")}
            </Button>
          </>
        ) : (
          <Button asChild>
            <Link to="/login">{t("forbidden.signIn")}</Link>
          </Button>
        )}
      </ErrorActions>
    </ErrorScreen>
  );
}
