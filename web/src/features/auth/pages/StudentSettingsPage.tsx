import { useTranslation } from "react-i18next";
import { Avatar } from "@/components/ui/avatar";
import { Card } from "@/components/ui/card";
import {
  GoogleSection,
  LanguageSection,
  PasswordSection,
  ProfileSection,
} from "@/features/auth/components/SettingsSections";
import { SignOutButton } from "@/features/auth/SignOutButton";
import { formatDate } from "@/lib/i18n/datetime";
import { useAuthStore } from "@/stores/auth";

/** StudentSettingsPage preserves forms while arranging account settings in reading order. */
export default function StudentSettingsPage() {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  return (
    <div className="mx-auto w-full max-w-[720px] space-y-4">
      <h1 className="text-xl font-semibold tracking-tight">{t("nav.settings")}</h1>
      {user && (
        <Card className="flex-row items-center gap-3 p-5">
          <Avatar name={user.fullName} size="lg" />
          <div className="min-w-0 space-y-1">
            <p className="text-base font-semibold break-words">{user.fullName}</p>
            <p className="text-muted-foreground text-sm break-all">{user.email}</p>
            <p className="text-muted-foreground text-xs">
              {t("settings.roleStudent")} · {t("settings.since")}{" "}
              {formatDate(user.createdAt)}
            </p>
          </div>
        </Card>
      )}
      <ProfileSection />
      <PasswordSection />
      <GoogleSection />
      <LanguageSection />
      <SignOutButton
        variant="outline"
        className="text-muted-foreground w-full sm:w-auto"
      />
    </div>
  );
}
