import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Navigate, useParams } from "react-router";
import { BookOpen, Lock, Palette, User } from "lucide-react";
import {
  SettingsLayout,
  type SettingsSection,
} from "@/components/shared/SettingsLayout";
import { useCan } from "@/features/auth/permissions";
import { PageHead } from "@/layouts/shell/PageHead";
import { ApiDocsSection } from "@/features/auth/components/SettingsSections";
import { AppearanceSection } from "@/features/settings/sections/Appearance";
import { ProfileSection } from "@/features/settings/sections/Profile";
import { SecuritySection } from "@/features/settings/sections/Security";

const BASE = "/teacher/settings";

/**
 * TeacherSettingsPage is the Teacher deck's Settings at
 * `/teacher/settings/:section`: Profile (also the bare address), Sign-in &
 * security, Appearance and, for holders of `system.api_reference` until R5,
 * API reference. Notifications and Assignment defaults join with T-R4.44
 * (DG-156). Every section stays mounted, so an unsaved profile survives a
 * visit to another one. Any other section, the pre-R4 "preferences"
 * included, is replaced by the bare address.
 */
export default function TeacherSettingsPage() {
  const { t } = useTranslation();
  const { section } = useParams();
  const canReadDocs = useCan("system.api_reference");
  const sections = useMemo<SettingsSection[]>(
    () => [
      {
        id: "profile",
        label: t("settings.sections.profile"),
        icon: User,
        to: BASE,
        content: <ProfileSection />,
      },
      {
        id: "security",
        label: t("settings.sections.security"),
        icon: Lock,
        to: `${BASE}/security`,
        content: <SecuritySection />,
      },
      {
        id: "appearance",
        label: t("settings.sections.appearance"),
        icon: Palette,
        to: `${BASE}/appearance`,
        content: <AppearanceSection />,
      },
      ...(canReadDocs
        ? [
            {
              id: "api",
              label: t("settings.sections.api"),
              icon: BookOpen,
              to: `${BASE}/api`,
              content: <ApiDocsSection />,
            },
          ]
        : []),
    ],
    [t, canReadDocs],
  );

  if (section !== undefined && !sections.some(({ id }) => id === section)) {
    return <Navigate replace to={BASE} />;
  }

  return (
    <div className="flex flex-col gap-4.5">
      <PageHead title={t("nav.settings")} description={t("settings.pageHint")} />
      <SettingsLayout
        label={t("settings.navigation")}
        active={section ?? "profile"}
        sections={sections}
      />
    </div>
  );
}
