import { useState } from "react";
import { useQuery, useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  Laptop,
  LogOut,
  MonitorSmartphone,
  Smartphone,
  Tablet,
  type LucideIcon,
} from "lucide-react";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { SettingsCard } from "@/components/shared/SettingsCard";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/sonner";
import {
  SESSIONS_KEY,
  listSessions,
  revokeOtherSessions,
  revokeSession,
  type Session,
} from "@/features/settings/api";
import { failureMessage } from "@/lib/api/errors";
import { formatRelative } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { authStore } from "@/stores/auth";

const KIND_ICON: Record<Session["deviceKind"], LucideIcon> = {
  computer: Laptop,
  phone: Smartphone,
  tablet: Tablet,
  unknown: MonitorSmartphone,
};

const ROW = "flex items-center gap-3 border-t px-4.5 py-3 first:border-t-0";
const SKELETON_ROWS = [0, 1, 2] as const;

type Revoking = { kind: "one"; familyId: string } | { kind: "others" } | null;

/**
 * DevicesSection is the Signed-in devices card of the teacher's Sign-in &
 * security settings, on `GET /auth/sessions`: one row per live session, the
 * calling one first as "This device", each other one with its own Sign out,
 * and Sign out other devices behind a confirmation. A revoke moves the
 * caller's own session epoch, so while one is in flight every Sign out is
 * off, and the list is read again once it settles, through the client's one
 * refresh. Only the server's labels are shown: an unrecognised device reads
 * "Unknown device", and a location is shown as given (DG-164).
 */
export function DevicesSection() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const sessions = useQuery({
    queryKey: SESSIONS_KEY,
    queryFn: ({ signal }) => listSessions(signal),
  });
  const [revoking, setRevoking] = useState<Revoking>(null);
  const [confirming, setConfirming] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const busy = revoking !== null;

  function revoke(next: Exclude<Revoking, null>, work: () => Promise<string>) {
    const lease = authStore.captureActor();
    setRevoking(next);
    setError(null);
    work()
      .then(
        (done) => {
          if (!authStore.isCurrent(lease)) return;
          setConfirming(false);
          toast(done);
        },
        (cause: unknown) => {
          if (!authStore.isCurrent(lease)) return;
          setError(failureMessage(cause, t("api.failed")));
        },
      )
      .then(() => queryClient.invalidateQueries({ queryKey: SESSIONS_KEY }))
      .finally(() => {
        if (authStore.isCurrent(lease)) setRevoking(null);
      });
  }

  function signOutOne(familyId: string) {
    revoke({ kind: "one", familyId }, async () => {
      await revokeSession(familyId);
      return t("settings.devices.signedOutOne");
    });
  }

  function signOutOthers() {
    revoke({ kind: "others" }, async () => {
      const revoked = await revokeOtherSessions();
      if (revoked === 0) return t("settings.devices.noOthers");
      if (revoked === 1) return t("settings.devices.signedOutOthersOne");
      return t("settings.devices.signedOutOthersMany", { count: revoked });
    });
  }

  return (
    <SettingsCard
      title={t("settings.devices.title")}
      description={t("settings.devices.hint")}
      className="overflow-hidden"
      action={
        <Button
          type="button"
          variant="outline"
          className="h-8.5"
          disabled={busy}
          onClick={() => setConfirming(true)}
        >
          {t("settings.devices.signOutOthers")}
        </Button>
      }
    >
      {error === null ? null : (
        <p role="alert" className="text-danger-ink border-b px-4.5 py-3 text-sm">
          {error}
        </p>
      )}
      <DeviceList
        query={sessions}
        busy={busy}
        signingOut={revoking?.kind === "one" ? revoking.familyId : null}
        onSignOut={signOutOne}
      />
      <ConfirmDialog
        open={confirming}
        onOpenChange={(open) => {
          if (!busy) setConfirming(open);
        }}
        title={t("settings.devices.confirmTitle")}
        description={t("settings.devices.confirmBody")}
        confirmLabel={t("settings.devices.signOut")}
        icon={LogOut}
        destructive
        pending={revoking?.kind === "others"}
        onConfirm={signOutOthers}
      />
    </SettingsCard>
  );
}

function DeviceList({
  query,
  busy,
  signingOut,
  onSignOut,
}: Readonly<{
  query: UseQueryResult<Session[]>;
  busy: boolean;
  signingOut: string | null;
  onSignOut: (familyId: string) => void;
}>) {
  const { t } = useTranslation();
  if (query.isPending) {
    return (
      <div role="status" aria-label={t("common.loading")}>
        {SKELETON_ROWS.map((row) => (
          <div key={row} className={ROW}>
            <Skeleton className="size-8.5 flex-none rounded-lg" />
            <span className="flex flex-1 flex-col gap-1.5">
              <Skeleton className="h-3.5 w-40" />
              <Skeleton className="h-3 w-28" />
            </span>
          </div>
        ))}
      </div>
    );
  }
  if (query.isError) {
    return (
      <div className="flex flex-wrap items-center gap-3 px-4.5 py-4">
        <p role="alert" className="text-muted-fg text-ui flex-1">
          {t("settings.devices.loadFailed")}
        </p>
        <Button
          type="button"
          variant="outline"
          className="h-8.5"
          onClick={() => void query.refetch()}
        >
          {t("common.retry")}
        </Button>
      </div>
    );
  }
  return (
    <ul aria-label={t("settings.devices.title")}>
      {query.data.map((session) => (
        <DeviceRow
          key={session.familyId}
          session={session}
          busy={busy}
          signingOut={signingOut === session.familyId}
          onSignOut={onSignOut}
        />
      ))}
    </ul>
  );
}

function DeviceRow({
  session,
  busy,
  signingOut,
  onSignOut,
}: Readonly<{
  session: Session;
  busy: boolean;
  signingOut: boolean;
  onSignOut: (familyId: string) => void;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const Icon = KIND_ICON[session.deviceKind];
  const name = session.device ?? t("settings.devices.unknown");
  const when = session.current
    ? t("settings.devices.now")
    : formatRelative(session.lastUsedAt, locale);
  const meta = session.location === null ? when : `${session.location} · ${when}`;
  return (
    <li className={ROW}>
      <span className="bg-muted grid size-8.5 flex-none place-items-center rounded-lg">
        <Icon aria-hidden="true" className="size-4" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="text-ui flex flex-wrap items-center gap-2 leading-normal font-medium">
          <span className="min-w-0 break-words">{name}</span>
          {session.current ? (
            <span className="bg-success-soft text-success-ink text-caption rounded-full px-1.75 leading-normal font-medium">
              {t("settings.devices.thisDevice")}
            </span>
          ) : null}
        </span>
        <span className="text-muted-fg text-meta block leading-normal break-words">
          {meta}
        </span>
      </span>
      {session.current ? null : (
        <Button
          type="button"
          variant="ghost"
          className="text-danger-ink hover:bg-danger-soft hover:text-danger-ink dark:hover:bg-danger-soft text-meta in-data-[scale=deck]:text-meta h-7.5 flex-none rounded-[7px] px-2.5 font-normal in-data-[scale=deck]:h-7.5 in-data-[scale=deck]:px-2.5"
          disabled={busy}
          aria-label={t("settings.devices.signOutNamed", { name })}
          onClick={() => onSignOut(session.familyId)}
        >
          {signingOut
            ? t("settings.devices.signingOut")
            : t("settings.devices.signOut")}
        </Button>
      )}
    </li>
  );
}
