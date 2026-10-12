import { useState, type ReactElement } from "react";
import { MemoryRouter } from "react-router";
import {
  Activity,
  AudioLines,
  Check,
  Clock,
  FileClock,
  FileText,
  Flag,
  Laptop,
  PencilLine,
  ShieldCheck,
  SquarePen,
} from "lucide-react";
import { Callout } from "@/components/shared/Callout";
import { EventList, type EventListItem } from "@/components/shared/EventList";
import { IconTile } from "@/components/shared/IconTile";
import { LockNotice } from "@/components/shared/LockNotice";
import { BarChart } from "@/components/shared/charts/BarChart";
import { KpiTile } from "@/components/shared/stats/KpiTile";
import { Meter } from "@/components/shared/stats/Meter";
import { ProgressBar } from "@/components/shared/stats/ProgressBar";
import { StatStrip } from "@/components/shared/stats/StatStrip";
import { thresholdTone } from "@/components/shared/stats/progress";
import { Button } from "@/components/ui/button";
import { registerContentElement } from "@/layouts/shell/contentWidth";

const KPI_GRID = "grid grid-cols-[repeat(auto-fit,minmax(210px,1fr))] gap-3";

const CHART = [6, 9, 4, 12, 15, 3, 2, 8, 11, 14, 18, 10, 13, 17];
const SUBMISSIONS = CHART.map((value, index) => {
  const day = String(11 + index);
  return { key: day, label: day, value, title: `${value} submissions · ${day} Sep` };
});

const ITEM_ANALYSIS = [
  { no: 27, percent: 22 },
  { no: 14, percent: 39 },
  { no: 31, percent: 44 },
  { no: 8, percent: 61 },
  { no: 3, percent: 83 },
];

const ICON_SIZES = [28, 30, 32, 34, 36, 40, 42, 44] as const;

const FLAGGED_ATTEMPT: readonly EventListItem[] = [
  { key: "start", tone: "info", text: "Started attempt", time: "19:22" },
  {
    key: "fullscreen-1",
    tone: "danger",
    text: "Left fullscreen · 14 s",
    time: "19:41",
  },
  { key: "tab", tone: "danger", text: "Switched tab · 32 s", time: "19:58" },
  { key: "fullscreen-2", tone: "danger", text: "Left fullscreen · 8 s", time: "20:06" },
  { key: "submit", tone: "success", text: "Submitted", time: "20:14" },
];

const QUIET_ATTEMPT: readonly EventListItem[] = [
  { key: "start", tone: "info", text: "Started attempt", time: "19:04" },
  { key: "autosave", tone: "neutral", text: "Answers autosaved", time: "every 10 s" },
  { key: "submit", tone: "success", text: "Submitted", time: "20:02" },
];

const BANNER_BUTTON = "in-data-[scale=deck]:h-8 in-data-[scale=deck]:rounded-md";

function countedBar(bar: ReactElement, count: string) {
  return (
    <div className="flex w-45 items-center gap-2.5">
      {bar}
      <span className="text-muted-fg text-meta w-10 text-right leading-normal tabular-nums">
        {count}
      </span>
    </div>
  );
}

export const cases: Record<string, () => ReactElement> = {
  "kpi-tiles": () => (
    <MemoryRouter>
      <div className={KPI_GRID}>
        <KpiTile
          label="To grade"
          icon={SquarePen}
          tone="warning"
          value="6"
          hint="4 students · oldest 2 days"
          to="/teacher/grading"
          action="Grade"
        />
        <KpiTile
          label="Flagged attempts"
          icon={Flag}
          tone="danger"
          value="3"
          hint="Focus lost during a test"
          to="/teacher/assignments"
          action="Review"
        />
        <KpiTile
          label="Closing in 24 h"
          icon={Clock}
          tone="muted"
          value="2"
          hint="Reading Mock · 18/24 in"
          to="/teacher/assignments"
          action="Monitor"
        />
        <KpiTile
          label="Taking a test now"
          icon={Activity}
          tone="success"
          live
          value="5"
          hint="Across 2 assignments"
          to="/teacher/assignments"
          action="Watch"
        />
      </div>
    </MemoryRouter>
  ),

  "kpi-tile-plain": () => (
    <div className={KPI_GRID}>
      <KpiTile
        label="Flagged attempts"
        icon={Flag}
        tone="danger"
        value="0"
        hint="No attempt is flagged"
      />
      <KpiTile
        label="Closing in 24 h"
        icon={Clock}
        tone="muted"
        value="0"
        hint="Nothing closes today"
      />
    </div>
  ),

  "stat-strip": () => (
    <div ref={registerContentElement}>
      <StatStrip
        items={[
          { label: "Submitted", value: "18", suffix: " / 24" },
          { label: "In progress", value: "2" },
          { label: "Average", value: "78", suffix: "%" },
          { label: "Flagged", value: "1", tone: "danger" },
        ]}
      />
    </div>
  ),

  progress: () => (
    <div className="flex flex-col gap-4">
      {countedBar(
        <ProgressBar
          className="flex-1"
          value={18}
          max={24}
          label="18 of 24 submitted"
          cap="round"
        />,
        "18/24",
      )}
      {countedBar(
        <ProgressBar
          className="flex-1"
          value={18}
          max={24}
          label="18 of 24 submitted"
        />,
        "18/24",
      )}
      {ITEM_ANALYSIS.map((item) => (
        <div key={item.no} className="flex w-55 items-center gap-2.5">
          <ProgressBar
            className="flex-1"
            value={item.percent}
            label={`Question ${item.no}: ${item.percent}% correct`}
            tone={thresholdTone(item.percent)}
          />
          <span className="text-meta w-9.5 text-right leading-normal tabular-nums">
            {item.percent}%
          </span>
        </div>
      ))}
      <ProgressBar value={81} label="Grammar 81%" size={8} cap="round" />
      <ProgressBar value={62} label="Writing 62%" size={8} cap="round" tone="warning" />
    </div>
  ),

  meter: () => (
    <div className="bg-card shadow-card flex flex-wrap items-center gap-3.5 rounded-xl border px-4 py-3">
      <span className="text-sm font-medium whitespace-nowrap">Storage</span>
      <Meter
        className="flex-[1_1_200px]"
        label="Storage"
        valueText="1.25 GB of 5 GB used: audio 1.05 GB, images 0.2 GB"
        max={5}
        parts={[
          { key: "audio", value: 1.05, tone: "accent" },
          { key: "images", value: 0.2, tone: "info" },
        ]}
      />
      <span className="text-muted-fg text-meta leading-normal whitespace-nowrap tabular-nums">
        1.25 GB of 5 GB · audio 1.05 GB · images 0.2 GB
      </span>
    </div>
  ),

  "bar-chart": () => (
    <BarChart
      caption="Submissions in the last 14 days"
      columns={["Day", "Submissions"]}
      data={SUBMISSIONS}
    />
  ),

  "bar-chart-phone": () => (
    <BarChart
      caption="Submissions in the last 14 days"
      columns={["Day", "Submissions"]}
      data={SUBMISSIONS}
      labelEvery={2}
    />
  ),

  "icon-tiles": () => (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end gap-3">
        {ICON_SIZES.map((size) => (
          <IconTile key={size} icon={size === 44 ? FileClock : FileText} size={size} />
        ))}
      </div>
      <div className="flex flex-wrap items-end gap-3">
        <IconTile icon={FileText} size={32} tone="info" />
        <IconTile icon={AudioLines} size={36} tone="info" />
        <IconTile icon={FileText} size={40} tone="info" />
        <IconTile icon={Check} size={42} tone="success" />
        <IconTile icon={SquarePen} size={30} tone="warning" />
        <IconTile icon={Flag} size={30} tone="danger" />
        <IconTile icon={Laptop} size={34} />
        <IconTile icon={FileText} size={32} tone="accent" />
      </div>
    </div>
  ),

  callouts: function Callouts() {
    const [dismissed, setDismissed] = useState(false);
    return (
      <div className="flex flex-col gap-4">
        <Callout>
          You can leave this page. Processing continues, and you can open it again from
          Imports.
        </Callout>
        <Callout
          icon={ShieldCheck}
          lead="Where your content goes."
          className="gap-3 [&>svg]:mt-px [&>svg]:size-[1.0625rem]"
        >
          Files and pasted text are processed on Quizzivy&apos;s own servers. Your
          centre&apos;s settings do not send exam content to outside AI services. The
          original stays private and students never see it.
        </Callout>
        <Callout tone="info" size="md">
          Review the conversion below before applying it. The original has not changed.
        </Callout>
        <Callout tone="warning" size="sm" announce>
          Only one answer is ticked. Tick another, or switch to Single choice.
        </Callout>
        <Callout
          tone="warning"
          icon={PencilLine}
          lead="The draft has changes that are not published."
          className="text-ui items-center gap-3 rounded-xl leading-normal [&>svg]:mt-0 [&>svg]:size-[1.0625rem]"
          actions={
            <>
              <Button variant="outline" size="sm" className={BANNER_BUTTON}>
                Review draft
              </Button>
              <Button
                size="sm"
                className={`${BANNER_BUTTON} in-data-[scale=deck]:font-semibold`}
              >
                Publish version 4
              </Button>
            </>
          }
        >
          New assignments get version 3 until you publish the draft as version 4.
        </Callout>
        {!dismissed && (
          <Callout tone="info" size="md" onDismiss={() => setDismissed(true)}>
            Reviewing works best on a tablet or computer. You can still check status and
            fix small items here.
          </Callout>
        )}
      </div>
    );
  },

  "callout-tones": () => (
    <div className="flex flex-col gap-4">
      {(["neutral", "info", "warning", "danger", "success"] as const).map((tone) => (
        <Callout key={tone} tone={tone} size="md">
          Review the conversion below before applying it. The original has not changed.
        </Callout>
      ))}
    </div>
  ),

  "lock-notice": () => (
    <div className="flex items-center justify-between gap-3">
      <span className="text-ui font-semibold">Test &amp; timing</span>
      <LockNotice reason="Locked while students are taking the test" />
    </div>
  ),

  "event-list": () => (
    <div className="bg-card flex max-w-95 flex-col gap-6 p-4.5">
      <EventList label="Attempt timeline, Lê Hoàng Nam" items={FLAGGED_ATTEMPT} />
      <EventList label="Attempt timeline, Trần Minh Anh" items={QUIET_ATTEMPT} />
    </div>
  ),
};
