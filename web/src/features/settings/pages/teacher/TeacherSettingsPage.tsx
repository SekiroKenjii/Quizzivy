import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Navigate, useParams } from "react-router";
import { Bell, BookOpen, Lock, Palette, SlidersHorizontal, User } from "lucide-react";
import {
  SettingsLayout,
  type SettingsSection,
} from "@/components/shared/SettingsLayout";
import { useCan } from "@/features/auth/permissions";
import { PageHead } from "@/layouts/shell/PageHead";
import { ApiDocsSection } from "@/features/auth/components/SettingsSections";
import { AppearanceSection } from "@/features/settings/sections/Appearance";
import { AssignmentDefaultsSection } from "@/features/settings/sections/AssignmentDefaults";
import { NotificationsSection } from "@/features/settings/sections/Notifications";
import { ProfileSection } from "@/features/settings/sections/Profile";
import { SecuritySection } from "@/features/settings/sections/Security";

const BASE = "/teacher/settings";

/**
 * TeacherSettingsPage is the Teacher deck's Settings at
 * `/teacher/settings/:section`: Profile (also the bare address), Sign-in &
 * security, Notifications, Assignment defaults for those who may assign,
 * Appearance and, for holders of `system.api_reference` until R5, API
 * reference (DG-157). Every section stays mounted, so unsaved changes
 * survive a visit to another one. Any other section, the pre-R4
 * "preferences" included, is replaced by the bare address.
 */
export default function TeacherSettingsPage() {
  const { t } = useTranslation();
  const { section } = useParams();
  const canReadDocs = useCan("system.api_reference");
  const canAssign = useCan("teaching.assignments.write");
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
        id: "notifications",
        label: t("settings.sections.notifications"),
        icon: Bell,
        to: `${BASE}/notifications`,
        content: <NotificationsSection />,
      },
      ...(canAssign
        ? [
            {
              id: "defaults",
              label: t("settings.sections.defaults"),
              icon: SlidersHorizontal,
              to: `${BASE}/defaults`,
              content: <AssignmentDefaultsSection />,
            },
          ]
        : []),
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
    [t, canReadDocs, canAssign],
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
