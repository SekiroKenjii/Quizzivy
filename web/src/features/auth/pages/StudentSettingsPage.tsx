import { useTranslation } from "react-i18next";
import {
  GoogleSection,
  LanguageSection,
  PasswordSection,
} from "@/features/auth/components/SettingsSections";

/**
 * §9's settings: three sections, and no profile block -- a student's name and
 * email come from the teacher or from Google, and there is nothing here for
 * them to edit.
 */
export default function StudentSettingsPage() {
  const { t } = useTranslation();
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold tracking-tight">{t("nav.settings")}</h1>
      <PasswordSection />
      <GoogleSection />
      <LanguageSection />
    </div>
  );
}
