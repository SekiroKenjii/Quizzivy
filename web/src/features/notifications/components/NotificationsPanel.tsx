import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/sonner";
import { IconTile } from "@/components/shared/IconTile";
import { useLocale } from "@/lib/i18n/useLocale";
import { cn } from "@/lib/utils";
import type { Notification, NotificationAudience } from "../api";
import { audienceOf, describeNotification, type NotificationView } from "../kinds";
import { relativeTime } from "../relativeTime";
import { useMarkNotificationsRead } from "../useMarkNotificationsRead";
import { useNotificationList } from "../useNotificationList";

export type { NotificationAudience };

interface Row {
  notification: Notification;
  view: NotificationView;
}

/**
 * NotificationsPanel is the bell's list, the same in the popover and the
 * sheet: "Mark all read", then one row per notification of the audience's
 * kinds, newest first, with its kind's tile (a neutral one in the muted
 * text colour, as both decks draw it), its sentence in the reader's
 * language, how long ago it came and a lime dot while unread, then "Load
 * more". The list, the marks and "Mark all read" cover only the audience's
 * kinds (DG-170); "Mark all read" is active while a listed row is unread,
 * and says so in a toast when it fails. Neither it nor
 * "Load more" gives up focus while its request is out, and after "Load
 * more" focus moves to the first row it added. Picking a row marks it read without waiting and hands its
 * destination to `onPick`, which may be null. `canGrade` says whether a
 * hand-in with papers to grade goes to Grading or to its own destination.
 * `heading` draws the "Notifications" title, which the sheet draws itself.
 */
export function NotificationsPanel({
  audience,
  canGrade,
  heading,
  onPick,
}: Readonly<{
  audience: NotificationAudience;
  canGrade: boolean;
  heading: boolean;
  onPick: (to: string | null) => void;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const list = useNotificationList(audience);
  const mark = useMarkNotificationsRead(audience);
  const [now] = useState(() => Date.now());
  const rows = useMemo(
    () =>
      list.items.flatMap((notification): Row[] => {
        if (audienceOf(notification.kind) !== audience) return [];
        const view = describeNotification(notification, t, locale, { canGrade });
        return view === null ? [] : [{ notification, view }];
      }),
    [audience, canGrade, list.items, locale, t],
  );
  const markAllOff =
    mark.isPending || rows.every((row) => row.notification.readAt !== null);
  const rowCount = rows.length;
  const loadingMore = list.loadingMore;
  const listRef = useRef<HTMLUListElement>(null);
  const loadMoreRef = useRef<HTMLButtonElement>(null);
  const focusFrom = useRef<number | null>(null);

  useEffect(() => {
    const from = focusFrom.current;
    if (from === null || loadingMore) return;
    focusFrom.current = null;
    const buttons =
      listRef.current?.querySelectorAll<HTMLButtonElement>(":scope > li > button");
    if (buttons === undefined || buttons.length === 0) return;
    const added = buttons[from];
    if (added !== undefined) added.focus();
    else if (!loadMoreRef.current?.isConnected) buttons[buttons.length - 1]!.focus();
  }, [loadingMore, rowCount]);

  const markAll = () => {
    if (markAllOff) return;
    mark.mutate(undefined, {
      onError: () => toast.error(t("notifications.bell.markFailed")),
    });
  };
  const loadMore = () => {
    if (loadingMore) return;
    if (document.activeElement === loadMoreRef.current) focusFrom.current = rowCount;
    list.loadMore();
  };

  const pick = (row: Row) => {
    if (row.notification.readAt === null) mark.mutate([row.notification.id]);
    onPick(row.view.to);
  };

  return (
    <div data-slot="notifications-panel" className="flex min-h-0 flex-col">
      <div
        className={cn(
          "flex flex-none items-center gap-2 border-b px-3.5 py-3",
          heading ? "justify-between" : "justify-end",
        )}
      >
        {heading ? (
          <h2 className="text-base font-semibold">{t("notifications.bell.title")}</h2>
        ) : null}
        <button
          type="button"
          aria-disabled={markAllOff || undefined}
          onClick={markAll}
          className="text-muted-fg hover:text-fg aria-disabled:text-foreground/50 aria-disabled:hover:text-foreground/50 text-meta rounded-sm aria-disabled:cursor-default"
        >
          {t("notifications.bell.markAllRead")}
        </button>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto">
        {list.isPending ? <PanelSkeleton /> : null}
        {list.isError ? (
          <div role="alert" className="flex flex-col items-start gap-2 px-3.5 py-4">
            <p className="text-sm">{t("notifications.bell.failed")}</p>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => void list.refetch()}
            >
              {t("common.retry")}
            </Button>
          </div>
        ) : null}
        {!list.isPending && !list.isError && rows.length === 0 ? (
          <p className="text-muted-fg px-3.5 py-8 text-center text-sm">
            {t("notifications.bell.empty")}
          </p>
        ) : null}
        {rows.length > 0 ? (
          <ul ref={listRef} className="m-0 list-none p-0">
            {rows.map((row) => (
              <li key={row.notification.id} className="border-b">
                <NotificationRow
                  row={row}
                  time={
                    relativeTime(row.notification.createdAt, now, locale) ??
                    t("notifications.bell.justNow")
                  }
                  onPick={() => pick(row)}
                />
              </li>
            ))}
          </ul>
        ) : null}
        {list.hasMore ? (
          <div className="flex justify-center px-3.5 py-2.5">
            <Button
              ref={loadMoreRef}
              type="button"
              variant="ghost"
              size="sm"
              aria-disabled={loadingMore || undefined}
              onClick={loadMore}
            >
              {t("notifications.bell.loadMore")}
            </Button>
          </div>
        ) : null}
      </div>
    </div>
  );
}

function NotificationRow({
  row,
  time,
  onPick,
}: Readonly<{ row: Row; time: string; onPick: () => void }>) {
  const { t } = useTranslation();
  const unread = row.notification.readAt === null;
  return (
    <button
      type="button"
      data-tone={row.view.tone}
      onClick={onPick}
      className="hover:bg-muted flex w-full cursor-pointer gap-2.5 px-3.5 py-3 text-left"
    >
      <IconTile
        icon={row.view.icon}
        size={30}
        tone={row.view.tone}
        className={row.view.tone === "neutral" ? "text-muted-fg" : undefined}
      />
      <span className="min-w-0 flex-1">
        <span className="block text-sm leading-[1.4] [overflow-wrap:anywhere]">
          {row.view.text}
        </span>
        <span className="text-muted-fg mt-0.5 block text-xs">{time}</span>
      </span>
      {unread ? (
        <span className="bg-brand mt-1.5 size-1.75 flex-none rounded-full">
          <span className="sr-only">{t("notifications.bell.unread")}</span>
        </span>
      ) : null}
    </button>
  );
}

function PanelSkeleton() {
  const { t } = useTranslation();
  return (
    <div role="status" aria-label={t("common.loading")} className="flex flex-col">
      {[0, 1, 2].map((index) => (
        <div key={index} className="flex gap-2.5 border-b px-3.5 py-3">
          <Skeleton className="size-7.5 rounded-lg" />
          <div className="flex flex-1 flex-col gap-1.5">
            <Skeleton className="h-3.5 w-4/5" />
            <Skeleton className="h-3 w-1/3" />
          </div>
        </div>
      ))}
    </div>
  );
}
