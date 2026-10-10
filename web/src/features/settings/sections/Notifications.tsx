import { useState } from "react";
import { useTranslation } from "react-i18next";
import { DirtyBar } from "@/components/shared/DirtyBar";
import { SettingsCard } from "@/components/shared/SettingsCard";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { toast } from "@/components/ui/sonner";
import type {
  NotificationEvent,
  NotificationPreference,
} from "@/features/notifications/api";
import { useNotificationPreferences } from "@/features/notifications/useNotificationPreferences";
import { failureMessage } from "@/lib/api/errors";

const TEACHER_EVENTS = [
  { event: "attempt.submitted", key: "submitted" },
  { event: "attempt.flagged", key: "flagged" },
  { event: "assignment.closing", key: "closing" },
] as const satisfies readonly { event: NotificationEvent; key: string }[];

const GRID = "grid grid-cols-[minmax(0,1fr)_64px] items-center gap-3 px-4.5";

type Switches = ReadonlyMap<NotificationEvent, boolean>;

function switchesOf(rows: readonly NotificationPreference[]): Switches {
  return new Map(rows.map((row) => [row.event, row.inApp]));
}

/**
 * NotificationsSection is the Notifications card of the teacher's settings:
 * "Choose where each update reaches you." with an In app switch for each of
 * the teacher's updates, A student submits, An attempt is flagged and An
 * assignment is about to close. Email arrives in R7, and New message and
 * Weekly summary with it. A change raises the DirtyBar; Save changes sends
 * all five switches, the student's two as they were read, and Discard
 * restores the saved ones.
 */
export function NotificationsSection() {
  const { t } = useTranslation();
  const { query, save } = useNotificationPreferences();
  const [edits, setEdits] = useState<Switches>(new Map());
  const [error, setError] = useState<string | null>(null);
  const saved = query.data;
  const savedSwitches = saved === undefined ? undefined : switchesOf(saved);
  const dirty =
    savedSwitches !== undefined &&
    [...edits].some(([event, on]) => savedSwitches.get(event) !== on);

  const isOn = (event: NotificationEvent) =>
    edits.get(event) ?? savedSwitches?.get(event) ?? true;

  function toggle(event: NotificationEvent, on: boolean) {
    setEdits((current) => new Map(current).set(event, on));
  }

  function submit() {
    if (saved === undefined) return;
    setError(null);
    save.mutate(
      saved.map((row) => ({ ...row, inApp: edits.get(row.event) ?? row.inApp })),
      {
        onSuccess: () => {
          setEdits(new Map());
          toast(t("settings.saved"));
        },
        onError: (cause) => setError(failureMessage(cause, t("api.failed"))),
      },
    );
  }

  return (
    <>
      <SettingsCard
        title={t("settings.notifications.title")}
        description={t("settings.notifications.hint")}
        className="overflow-hidden"
      >
        <div
          className={`${GRID} bg-muted text-muted-fg text-meta py-2.5 leading-normal font-medium`}
          aria-hidden="true"
        >
          <span>{t("settings.notifications.update")}</span>
          <span className="text-center">{t("settings.notifications.inApp")}</span>
        </div>
        {query.isPending ? (
          <div role="status" aria-label={t("common.loading")}>
            {TEACHER_EVENTS.map(({ event }) => (
              <div key={event} className={`${GRID} border-t py-3`}>
                <span className="flex flex-col gap-1.5">
                  <Skeleton className="h-3.5 w-48" />
                  <Skeleton className="h-3 w-32" />
                </span>
                <Skeleton className="mx-auto h-5.5 w-9.5 rounded-full" />
              </div>
            ))}
          </div>
        ) : null}
        {query.isError ? (
          <div className="flex flex-wrap items-center gap-3 border-t px-4.5 py-4">
            <p role="alert" className="text-muted-fg text-ui flex-1">
              {t("settings.notifications.loadFailed")}
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
        ) : null}
        {saved === undefined
          ? null
          : TEACHER_EVENTS.map(({ event, key }) => (
              <div key={event} className={`${GRID} border-t py-3`}>
                <span className="min-w-0">
                  <span
                    id={`notify-${key}`}
                    className="text-ui block leading-normal font-medium"
                  >
                    {t(`settings.notifications.events.${key}`)}
                  </span>
                  <span className="text-muted-fg text-meta block leading-normal">
                    {t(`settings.notifications.events.${key}Hint`)}
                  </span>
                </span>
                <span className="flex justify-center">
                  <Switch
                    checked={isOn(event)}
                    aria-label={t("settings.notifications.inAppFor", {
                      update: t(`settings.notifications.events.${key}`),
                    })}
                    disabled={save.isPending}
                    onCheckedChange={(on) => toggle(event, on)}
                  />
                </span>
              </div>
            ))}
      </SettingsCard>
      <DirtyBar
        dirty={dirty}
        saving={save.isPending}
        error={error}
        onDiscard={() => {
          setError(null);
          setEdits(new Map());
        }}
        onSave={submit}
      />
    </>
  );
}
