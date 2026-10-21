import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Navigate, useParams } from "react-router";
import { ADMIN_PATHS } from "@/app/adminPaths";
import {
  SettingsLayout,
  type SettingsSection,
} from "@/components/shared/SettingsLayout";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { organizationSettingsQuery } from "@/features/organization/api";
import { SettingsForm } from "@/features/organization/SettingsForm";
import { SYSTEM_SECTIONS } from "@/features/organization/systemSections";
import { UnsavedChangesGuard } from "@/features/settings/UnsavedChangesGuard";
import { PageHead } from "@/layouts/shell/PageHead";
import { useOrganization } from "@/lib/organization";

function SettingsSkeleton() {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      aria-label={t("common.loading")}
      className="bg-card shadow-card rounded-xl border"
    >
      <div className="flex flex-col gap-1.5 border-b px-4.5 py-4">
        <Skeleton className="h-4 w-32" />
        <Skeleton className="h-3.5 w-64 max-w-full" />
      </div>
      <div className="flex flex-col gap-4 p-4.5">
        <div className="flex items-center gap-3.5">
          <Skeleton className="size-14 rounded-[14px]" />
          <Skeleton className="h-8.5 w-28" />
        </div>
        <div className="flex flex-col gap-3.5">
          <Skeleton className="h-9.5 w-full max-w-(--field-max)" />
          <Skeleton className="h-9.5 w-full max-w-(--field-max)" />
          <Skeleton className="h-9.5 w-full max-w-(--field-max)" />
        </div>
      </div>
    </div>
  );
}

function SettingsLoadError({ onRetry }: Readonly<{ onRetry: () => void }>) {
  const { t } = useTranslation();
  return (
    <div className="bg-card shadow-card flex flex-wrap items-center gap-3 rounded-xl border px-4.5 py-4">
      <p role="alert" className="text-muted-fg text-ui min-w-40 flex-1">
        {t("systemSettings.loadFailed")}
      </p>
      <Button type="button" variant="outline" className="h-8.5" onClick={onRetry}>
        {t("common.retry")}
      </Button>
    </div>
  );
}

/**
 * SystemSettingsPage is the Admin console's System settings, at the
 * `settings/:section` route of the console: "Applies to everyone in {organization}." over
 * a navigation of its sections, Organization (also the bare address), Sign-in,
 * Data & privacy and Plan & usage. The first three share one form and one unsaved-changes
 * bar, and leaving with unsaved changes asks first. While the document is
 * read the sections hold a skeleton, and when it cannot be they hold the
 * failure and Retry. Plan & usage is read-only
 * and reads the usage itself. An address with an unknown section is not the console's (`consolePath`) and is sent where an
 * old teacher address goes.
 */
export default function SystemSettingsPage() {
  const { t } = useTranslation();
  const { section } = useParams();
  const settings = useQuery(organizationSettingsQuery());
  const fallback = useOrganization().name;
  const active = section ?? SYSTEM_SECTIONS[0]!.id;

  if (!SYSTEM_SECTIONS.some(({ id }) => id === active)) {
    return <Navigate replace to={ADMIN_PATHS.settings} />;
  }

  const name = settings.data?.organization.name ?? fallback ?? t("app.name");
  const state = settings.isError ? (
    <SettingsLoadError onRetry={() => void settings.refetch()} />
  ) : (
    <SettingsSkeleton />
  );
  const placeholder: SettingsSection[] = SYSTEM_SECTIONS.map(
    ({ id, label, icon, to }) => ({ id, label: t(label), icon, to, content: state }),
  );

  return (
    <div className="flex flex-col gap-4.5">
      <PageHead
        title={t("systemSettings.title")}
        description={t("systemSettings.hint", { organization: name })}
      />
      <UnsavedChangesGuard base={ADMIN_PATHS.settings}>
        {settings.data === undefined ? (
          <SettingsLayout
            label={t("systemSettings.navigation")}
            active={active}
            sections={placeholder}
          />
        ) : (
          <SettingsForm saved={settings.data} active={active} />
        )}
      </UnsavedChangesGuard>
    </div>
  );
}
