import { useTranslation } from "react-i18next";
import {
  GoogleSection,
  LanguageSection,
  PasswordSection,
  ProfileSection,
} from "@/features/auth/components/SettingsSections";
import { PageHeader } from "@/components/shared/PageHeader";

/** §8's settings: profile, password, Google, language. */
export default function AdminSettingsPage() {
  const { t } = useTranslation();
  return (
    <div className="max-w-3xl space-y-3">
      <div className="mb-4">
        <PageHeader variant="title" title={t("nav.settings")} />
      </div>
      <ProfileSection />
      <PasswordSection />
      <GoogleSection />
      <LanguageSection />
    </div>
  );
}
