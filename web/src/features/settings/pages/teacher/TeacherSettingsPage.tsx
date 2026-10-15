import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Navigate, useParams } from "react-router";
import { Bell, Lock, Palette, SlidersHorizontal, Sparkles, User } from "lucide-react";
import {
  SettingsLayout,
  type SettingsSection,
} from "@/components/shared/SettingsLayout";
import { ADMIN_PATHS } from "@/app/adminPaths";
import { useCan } from "@/features/auth/permissions";
import { PageHead } from "@/layouts/shell/PageHead";
import { AiAssistantSection } from "@/features/settings/sections/AiAssistant";
import { AppearanceSection } from "@/features/settings/sections/Appearance";
import { AssignmentDefaultsSection } from "@/features/settings/sections/AssignmentDefaults";
import { NotificationsSection } from "@/features/settings/sections/Notifications";
import { ProfileSection } from "@/features/settings/sections/Profile";
import { SecuritySection } from "@/features/settings/sections/Security";
import { UnsavedChangesGuard } from "@/features/settings/UnsavedChangesGuard";

const BASE = "/teacher/settings";

/**
 * TeacherSettingsPage is the Teacher deck's Settings at
 * `/teacher/settings/:section`: Profile (also the bare address), Sign-in &
 * security, Notifications, Assignment defaults for those who may assign,
 * Appearance and AI assistant for holders of `ai.use` (which reads nothing
 * until it is first shown). Every section stays mounted, so unsaved changes
 * survive a visit to another one, and leaving Settings with unsaved changes
 * asks first. The API reference moved to the Admin console in R5 (T-R5.27),
 * so `api` goes there for who may read it; any other section, the pre-R4
 * "preferences" included, is replaced by the bare address.
 */
export default function TeacherSettingsPage() {
  const { t } = useTranslation();
  const { section } = useParams();
  const canReadDocs = useCan("system.api_reference");
  const canAssign = useCan("teaching.assignments.write");
  const canUseAi = useCan("ai.use");
  const aiShown = section === "ai";
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
      ...(canUseAi
        ? [
            {
              id: "ai",
              label: t("settings.sections.ai"),
              icon: Sparkles,
              to: `${BASE}/ai`,
              content: <AiAssistantSection shown={aiShown} />,
            },
          ]
        : []),
    ],
    [t, canAssign, canUseAi, aiShown],
  );

  if (section === "api") {
    return <Navigate replace to={canReadDocs ? ADMIN_PATHS.api : BASE} />;
  }
  if (section !== undefined && !sections.some(({ id }) => id === section)) {
    return <Navigate replace to={BASE} />;
  }

  return (
    <div className="flex flex-col gap-4.5">
      <PageHead title={t("nav.settings")} description={t("settings.pageHint")} />
      <UnsavedChangesGuard base={BASE}>
        <SettingsLayout
          label={t("settings.navigation")}
          active={section ?? "profile"}
          sections={sections}
        />
      </UnsavedChangesGuard>
    </div>
  );
}
