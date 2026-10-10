import { useId } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import type { NotificationEvent } from "../api";
import { STUDENT_EVENTS } from "../kinds";
import { withPreference } from "../preferences";
import { useNotificationPreferences } from "../useNotificationPreferences";

const COPY: Record<(typeof STUDENT_EVENTS)[number], { title: string; hint: string }> = {
  "assignment.due_soon": {
    title: "notifications.settings.dueSoon",
    hint: "notifications.settings.dueSoonHint",
  },
  "result.ready": {
    title: "notifications.settings.resultReady",
    hint: "notifications.settings.resultReadyHint",
  },
};

/**
 * StudentNotificationsSection is the Notifications card of the student's
 * Settings: "Test due soon" and "Result ready", each switch turning its
 * event on or off in the app and by email at once (DG-82). A switch saves as
 * it moves and shows the new value while the save is out; a failed save puts
 * the saved value back and says so. The other rows the deck draws arrive
 * with their releases: messages in R7, the practice reminder in R10.
 */
export function StudentNotificationsSection() {
  const { t } = useTranslation();
  const prefix = useId();
  const { query, save } = useNotificationPreferences();
  const rows = save.isPending ? save.variables : query.data;

  const change = (event: NotificationEvent, on: boolean) => {
    if (rows === undefined) return;
    save.mutate(withPreference(rows, event, { inApp: on, email: on }));
  };

  return (
    <section
      aria-label={t("student.settings.sections.notifications")}
      className="bg-card shadow-card overflow-hidden rounded-xl border"
    >
      {query.isPending ? (
        <div
          role="status"
          aria-label={t("common.loading")}
          className="flex flex-col gap-3 p-4"
        >
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
        </div>
      ) : null}
      {query.isError ? (
        <div role="alert" className="flex flex-col items-start gap-2 p-4">
          <p className="text-sm">{t("notifications.settings.loadFailed")}</p>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => void query.refetch()}
          >
            {t("common.retry")}
          </Button>
        </div>
      ) : null}
      {rows === undefined
        ? null
        : STUDENT_EVENTS.map((event) => {
            const row = rows.find((candidate) => candidate.event === event);
            const id = `${prefix}-${event}`;
            return (
              <div
                key={event}
                className="flex items-center gap-3.5 border-t px-4 py-3.5 first:border-t-0"
              >
                <span className="min-w-0 flex-1">
                  <span id={id} className="block text-base leading-normal font-medium">
                    {t(COPY[event].title)}
                  </span>
                  <span
                    id={`${id}-hint`}
                    className="text-muted-fg text-meta block leading-normal"
                  >
                    {t(COPY[event].hint)}
                  </span>
                </span>
                <Switch
                  size="lg"
                  checked={row?.inApp ?? false}
                  aria-labelledby={id}
                  aria-describedby={`${id}-hint`}
                  disabled={save.isPending || row === undefined}
                  onCheckedChange={(on) => change(event, on)}
                />
              </div>
            );
          })}
      {save.isError ? (
        <p role="alert" className="text-danger-ink border-t px-4 py-2.5 text-sm">
          {t("notifications.settings.saveFailed")}
        </p>
      ) : null}
    </section>
  );
}
