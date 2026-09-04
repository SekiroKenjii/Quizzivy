import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { EmptyState, LoadError } from "@/components/shared/ListState";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  getAttemptEvents,
  type AdminQuestion,
  type IntegrityEvent,
} from "@/features/attempts/api";
import { POLL_MS, eventsKey } from "@/features/attempts/keys";
import { formatInTimeZone } from "date-fns-tz";
import { APP_TIME_ZONE } from "@/lib/i18n/datetime";
import { cn } from "@/lib/utils";
import {
  clockSpan,
  hasOpenEpisode,
  timelineRows,
  type TimelineFilter,
} from "../timeline";

const FILTERS: TimelineFilter[] = ["all", "away", "audio", "network"];

/**
 * G-05: the summary strip, then the chronological list. Neutral by design --
 * counts and durations, no verdicts, and the honest-limits card beside it.
 */
export function Timeline({
  attemptId,
  questions,
  live,
  onViewPaper,
}: {
  attemptId: string;
  questions: AdminQuestion[];
  /** Polls while the attempt is still in progress. */
  live: boolean;
  onViewPaper: () => void;
}) {
  const { t } = useTranslation();
  const [filter, setFilter] = useState<TimelineFilter>("all");
  const events = useQuery({
    queryKey: eventsKey(attemptId),
    queryFn: ({ signal }) => getAttemptEvents(attemptId, signal),
    refetchInterval: live ? POLL_MS : false,
    refetchIntervalInBackground: false,
  });

  if (events.isPending) return <TimelineSkeleton />;
  if (events.isError) {
    return (
      <div className="space-y-1">
        <LoadError error={events.error} onRetry={() => void events.refetch()}>
          {t("timeline.loadFailed")}
        </LoadError>
        <p className="text-muted-foreground text-xs">{t("timeline.loadFailedHint")}</p>
      </div>
    );
  }

  const data = events.data;
  const empty = data.events.length === 0;
  const open = hasOpenEpisode(data.events);
  const rows = timelineRows(data.events, filter);
  const numberOf = new Map(questions.map((q, i) => [q.id, i + 1]));

  return (
    <div className="space-y-5">
      <div className="grid grid-cols-6 gap-4">
        <Strip
          label={t("timeline.strip.totalAway")}
          hint={open ? t("timeline.strip.openNotSummed") : null}
          empty={empty}
        >
          {clockSpan(data.summary.totalAwayMs)}
        </Strip>
        <Strip
          label={t("timeline.strip.episodes")}
          hint={open ? t("timeline.strip.oneOpen") : null}
          empty={empty}
        >
          {data.summary.awayEpisodes}
        </Strip>
        <Strip label={t("timeline.strip.pastes")} empty={empty}>
          {data.summary.pasteCount}
        </Strip>
        <Strip label={t("timeline.strip.resumes")} empty={empty}>
          {data.summary.resumeCount}
        </Strip>
        <Strip label={t("timeline.strip.replays")} empty={empty}>
          {data.summary.audioReplays}
        </Strip>
        <Strip label={t("timeline.strip.offline")} empty={empty}>
          {data.summary.offlineEpisodes}
        </Strip>
      </div>

      <div className="grid grid-cols-3 gap-5">
        <Card className="col-span-2 gap-0">
          <CardHeader className="flex items-center justify-between">
            <CardTitle>{t("timeline.title")}</CardTitle>
            {!empty && (
              <div
                className="flex items-center gap-1.5"
                role="group"
                aria-label={t("timeline.filter")}
              >
                {FILTERS.map((value) => (
                  <Button
                    key={value}
                    size="xs"
                    variant={filter === value ? "secondary" : "ghost"}
                    className={cn(filter !== value && "text-muted-foreground")}
                    aria-pressed={filter === value}
                    onClick={() => setFilter(value)}
                  >
                    {t(`timeline.filters.${value}`)}
                  </Button>
                ))}
              </div>
            )}
          </CardHeader>
          <CardContent className="pt-3">
            {empty ? (
              <EmptyState
                action={
                  <Button size="sm" variant="outline" onClick={onViewPaper}>
                    {t("timeline.viewPaper")}
                  </Button>
                }
              >
                {t("timeline.empty")}
              </EmptyState>
            ) : (
              <>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-[90px]">
                        {t("timeline.columns.at")}
                      </TableHead>
                      <TableHead className="w-[90px]">
                        {t("timeline.columns.offset")}
                      </TableHead>
                      <TableHead>{t("timeline.columns.event")}</TableHead>
                      <TableHead className="text-right">
                        {t("timeline.columns.duration")}
                      </TableHead>
                      <TableHead className="text-right">
                        {t("timeline.columns.question")}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <TableRow>
                      <TableCell className="text-muted-foreground tabular-nums">
                        {clockTime(data.startedAt)}
                      </TableCell>
                      <TableCell className="text-muted-foreground tabular-nums">
                        {clockSpan(0)}
                      </TableCell>
                      <TableCell>{t("timeline.kind.started")}</TableCell>
                      <TableCell className="text-muted-foreground text-right">
                        —
                      </TableCell>
                      <TableCell className="text-muted-foreground text-right">
                        {1}
                      </TableCell>
                    </TableRow>
                    {rows.map(({ event, ongoing, playNo }) => (
                      <TableRow key={event.id}>
                        <TableCell className="text-muted-foreground tabular-nums">
                          {clockTime(event.occurredAt)}
                        </TableCell>
                        <TableCell className="text-muted-foreground tabular-nums">
                          {clockSpan(event.offsetMs)}
                        </TableCell>
                        <TableCell>
                          <EventLabel
                            event={event}
                            playNo={playNo}
                            ongoing={ongoing}
                            questions={questions}
                          />
                        </TableCell>
                        <TableCell className="text-right tabular-nums">
                          {ongoing ? (
                            <span className="text-muted-foreground">
                              {t("timeline.ongoing")}
                            </span>
                          ) : event.durationMs == null ? (
                            <span className="text-muted-foreground">—</span>
                          ) : (
                            <span
                              className={cn(
                                event.kind !== "audio_play" && "font-medium",
                              )}
                            >
                              {clockSpan(event.durationMs)}
                            </span>
                          )}
                        </TableCell>
                        <TableCell className="text-right tabular-nums">
                          {event.questionId && numberOf.has(event.questionId) ? (
                            numberOf.get(event.questionId)
                          ) : (
                            <span className="text-muted-foreground">—</span>
                          )}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
                {live && (
                  <p className="text-muted-foreground mt-3 text-xs">
                    {t("timeline.updatedAt", {
                      time: clockTime(new Date(events.dataUpdatedAt)),
                    })}
                    {open ? ` ${t("timeline.openResolves")}` : ""}
                  </p>
                )}
              </>
            )}
          </CardContent>
        </Card>

        <Card className="gap-0 self-start">
          <CardHeader>
            <CardTitle>{t("timeline.help.title")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2.5 pt-3">
            <p className="text-muted-foreground text-sm leading-relaxed">
              {t("timeline.help.cannotSee")}
            </p>
            <p className="text-muted-foreground text-sm leading-relaxed">
              {t("timeline.help.duration")}
            </p>
            <p className="text-muted-foreground text-sm leading-relaxed">
              {t("timeline.help.conversation")}
            </p>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function EventLabel({
  event,
  playNo,
  ongoing,
  questions,
}: {
  event: IntegrityEvent;
  playNo: number | null;
  ongoing: boolean;
  questions: AdminQuestion[];
}) {
  const { t } = useTranslation();
  const label = t(`timeline.kind.${event.kind}`, { defaultValue: event.kind });
  if (event.kind === "audio_play" && playNo !== null) {
    const maxPlays =
      questions.find((q) => q.id === event.questionId)?.audio?.maxPlays ?? null;
    return (
      <>
        {t("timeline.playNo", { label, n: playNo })}
        {maxPlays !== null && playNo > maxPlays && (
          <span className="text-muted-foreground">
            {" "}
            {t("timeline.overLimit", { max: maxPlays })}
          </span>
        )}
      </>
    );
  }
  return (
    <>
      {label}
      {ongoing && (
        <span className="text-muted-foreground"> {t("timeline.stillOpen")}</span>
      )}
    </>
  );
}

function Strip({
  label,
  hint = null,
  empty,
  children,
}: {
  label: string;
  hint?: string | null;
  empty: boolean;
  children: React.ReactNode;
}) {
  return (
    <Card>
      <CardContent>
        <p className="text-muted-foreground text-xs">{label}</p>
        <p
          className={cn(
            "mt-1 text-xl font-semibold tabular-nums",
            empty && "text-muted-foreground",
          )}
        >
          {empty ? "—" : children}
        </p>
        {hint !== null && !empty && (
          <p className="text-muted-foreground mt-1 text-xs">{hint}</p>
        )}
      </CardContent>
    </Card>
  );
}

function TimelineSkeleton() {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      aria-live="polite"
      aria-label={t("common.loading")}
      className="space-y-5"
    >
      <div className="grid grid-cols-6 gap-4">
        {Array.from({ length: 6 }, (_, i) => (
          <Card key={i}>
            <CardContent>
              <Skeleton className="h-3 w-24" />
              <Skeleton className="mt-2 h-7 w-12" />
            </CardContent>
          </Card>
        ))}
      </div>
      <Card>
        <CardContent className="space-y-2">
          {Array.from({ length: 5 }, (_, i) => (
            <Skeleton key={i} className="h-9 w-full" />
          ))}
        </CardContent>
      </Card>
    </div>
  );
}

/** "09:48:02", wall-clock in the app's zone. */
function clockTime(utc: string | Date): string {
  return formatInTimeZone(utc, APP_TIME_ZONE, "HH:mm:ss");
}
