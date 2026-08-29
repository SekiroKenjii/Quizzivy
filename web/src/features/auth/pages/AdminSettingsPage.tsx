import { useTranslation } from "react-i18next";
import {
  GoogleSection,
  LanguageSection,
  PasswordSection,
  ProfileSection,
} from "@/features/auth/components/SettingsSections";

/** §8's settings: profile, password, Google, language. */
export default function AdminSettingsPage() {
  const { t } = useTranslation();
  return (
    <div className="max-w-3xl space-y-3">
      <h1 className="mb-4 text-xl font-semibold tracking-tight">{t("nav.settings")}</h1>
      <ProfileSection />
      <PasswordSection />
      <GoogleSection />
      <LanguageSection />
    </div>
  );
}
