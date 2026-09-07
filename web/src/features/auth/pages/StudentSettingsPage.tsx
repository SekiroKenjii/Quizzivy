import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { Avatar } from "@/components/ui/avatar";
import { Separator } from "@/components/ui/separator";
import { PageAside } from "@/components/shared/PageAside";
import { PanelLabel, PanelRow } from "@/components/shared/PanelLabel";
import {
  GoogleSection,
  LanguageSection,
  PasswordSection,
} from "@/features/auth/components/SettingsSections";
import { SignOutButton } from "@/features/auth/SignOutButton";
import { fetchMyClasses } from "@/features/classes/api";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { formatDate } from "@/lib/i18n/datetime";
import { useAuthStore } from "@/stores/auth";

/**
 * §9's settings: three sections, and no profile block -- a student's name and
 * email come from the teacher or from Google, and there is nothing here for
 * them to edit. From 1024px it is S-17: the sections in a grid, and the
 * account -- who, how they sign in, since when, in how many classes, and the
 * way out -- in F-11's panel.
 */
export default function StudentSettingsPage() {
  const { t } = useTranslation();
  const wide = useMediaQuery("(min-width: 1024px)");
  const user = useAuthStore((s) => s.user);
  const classes = useQuery({
    queryKey: ["my-classes"],
    queryFn: ({ signal }) => fetchMyClasses(signal),
    enabled: wide,
  });

  if (!wide) {
    return (
      <div className="space-y-3">
        <PasswordSection />
        <GoogleSection />
        <LanguageSection />
        <SignOutButton variant="outline" className="text-muted-foreground w-full" />
      </div>
    );
  }

  const signIn = [
    user?.hasPassword ? t("settings.signInPassword") : null,
    user?.linkedProviders.includes("google") ? t("settings.signInGoogle") : null,
  ].filter((s): s is string => s !== null);

  return (
    <div className="space-y-5">
      <h1 className="text-xl font-semibold tracking-tight">{t("nav.settings")}</h1>
      <div className="grid grid-cols-2 items-start gap-4">
        <PasswordSection />
        <GoogleSection />
        <LanguageSection />
      </div>
      {user && (
        <PageAside label={t("settings.account")}>
          <div>
            <PanelLabel>{t("settings.account")}</PanelLabel>
            <div className="flex items-center gap-3">
              <Avatar name={user.fullName} size="lg" />
              <div className="min-w-0">
                <p className="truncate text-sm font-medium">{user.fullName}</p>
                <p className="text-muted-foreground truncate text-xs">{user.email}</p>
              </div>
            </div>
          </div>
          <div className="space-y-2">
            <PanelRow label={t("settings.role")}>{t("settings.roleStudent")}</PanelRow>
            <PanelRow label={t("settings.signInWith")}>
              {signIn.length === 0 ? "—" : signIn.join(" · ")}
            </PanelRow>
            <PanelRow label={t("settings.since")}>
              {formatDate(user.createdAt)}
            </PanelRow>
            <PanelRow label={t("settings.classCount")}>
              <span className="tabular-nums">
                {classes.data === undefined ? "—" : classes.data.items.length}
              </span>
            </PanelRow>
          </div>
          <Separator />
          <SignOutButton variant="outline" className="text-muted-foreground w-full" />
        </PageAside>
      )}
    </div>
  );
}
