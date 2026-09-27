import { useTranslation } from "react-i18next";
import { CircleCheck, Clock } from "lucide-react";

import { ErrorActions, ErrorScreen } from "@/app/pages/ErrorScreen";
import { MaintenanceArt } from "@/app/pages/errorArt";
import { Button } from "@/components/ui/button";
import { clockTime } from "@/lib/i18n/datetime";

/**
 * MaintenancePage says the API is down for the window it is given, when it
 * will be back, and that nothing is lost. Times are in the app's time zone,
 * with the date added when it is not today. `onCheck` looks again.
 */
export function MaintenancePage({
  window,
  onCheck,
  now = new Date(),
}: Readonly<{
  window: { startsAt: string; endsAt: string };
  onCheck: () => void;
  now?: Date;
}>) {
  const { t } = useTranslation();
  const minutes = Math.max(
    1,
    Math.round((Date.parse(window.endsAt) - Date.parse(window.startsAt)) / 60_000),
  );

  return (
    <ErrorScreen
      art={<MaintenanceArt />}
      title={t("maintenance.title")}
      body={t("maintenance.body", { time: clockTime(window.endsAt, now) })}
      footer={t("maintenance.footnote")}
    >
      <ul className="mt-4 flex flex-col overflow-hidden rounded-lg border">
        <li className="text-ui flex items-center gap-2.5 px-3 py-2.5">
          <Clock aria-hidden="true" className="text-warning-ink size-[15px] shrink-0" />
          <span className="flex-1">
            {t("maintenance.started", { time: clockTime(window.startsAt, now) })}
          </span>
          <span className="text-muted-fg text-meta whitespace-nowrap tabular-nums">
            {t("maintenance.duration", { minutes })}
          </span>
        </li>
        <li className="text-ui flex items-center gap-2.5 border-t px-3 py-2.5">
          <CircleCheck
            aria-hidden="true"
            className="text-success-ink size-[15px] shrink-0"
          />
          <span className="flex-1">{t("maintenance.saved")}</span>
        </li>
      </ul>

      <ErrorActions>
        <Button onClick={onCheck}>{t("maintenance.check")}</Button>
      </ErrorActions>
    </ErrorScreen>
  );
}
