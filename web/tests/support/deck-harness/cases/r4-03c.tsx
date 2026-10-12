import { useState, type ReactElement } from "react";
import { Pause, Play } from "lucide-react";
import { SplitPane } from "@/components/shared/SplitPane";
import { Waveform } from "@/components/shared/Waveform";

const FIRST = (
  <div className="bg-card rounded-xl border p-4">
    <p>Reading</p>
    <p>1. Since 2019, she…</p>
    <p>2. City parks</p>
    <p>3. Public transport</p>
  </div>
);
const SECOND = (
  <div className="bg-card min-h-[900px] rounded-xl border p-4">Prompt</div>
);

/** cases provides the two split handles, stacked panes and deterministic media bars for deck comparison. */
export const cases: Record<string, () => ReactElement> = {
  "split-grip": () => (
    <SplitPane
      label="Resize outline"
      unit="px"
      defaultSize={300}
      min={220}
      max={1000}
      minSecond={360}
      first={FIRST}
      second={SECOND}
    />
  ),
  "split-stacked": () => (
    <SplitPane
      label="Resize outline"
      unit="px"
      defaultSize={300}
      min={220}
      max={1000}
      minSecond={360}
      split={false}
      className="gap-y-3.5"
      first={FIRST}
      second={SECOND}
    />
  ),
  "split-line": () => (
    <SplitPane
      label="Resize panes"
      unit="percent"
      defaultSize={42}
      min={30}
      max={65}
      handle="line"
      className="h-[480px]"
      firstClassName="border-r"
      first={<p>Source</p>}
      second={<p>Questions</p>}
    />
  ),
  waveform: function Waveforms() {
    const [playing, setPlaying] = useState(true);
    return (
      <div className="flex flex-col gap-4">
        {[
          { seed: 0, progress: 0 },
          { seed: 2, progress: 0 },
          { seed: 0, progress: playing ? 11 / 28 : 0 },
        ].map(({ seed, progress }, index) => (
          <div key={index} className="bg-muted flex h-28 w-60 items-center gap-3 px-4">
            <button
              type="button"
              onClick={() => {
                if (index === 2) setPlaying(!playing);
              }}
              aria-label={index === 2 && playing ? "Pause audio" : "Play audio"}
              className="bg-primary text-primary-fg grid size-10 shrink-0 place-items-center rounded-full"
            >
              {progress > 0 ? (
                <Pause size={17} aria-hidden="true" />
              ) : (
                <Play size={17} aria-hidden="true" />
              )}
            </button>
            <Waveform seed={seed} progress={progress} />
          </div>
        ))}
      </div>
    );
  },
  "stable-state": function StableState() {
    const [split, setSplit] = useState(true);
    return (
      <div className="flex flex-col gap-4">
        <button onClick={() => setSplit(!split)}>Toggle stacking</button>
        <SplitPane
          label="Resize draft panes"
          unit="px"
          defaultSize={300}
          min={220}
          max={1000}
          minSecond={360}
          split={split}
          className="gap-y-3.5"
          first={
            <label>
              First draft
              <input
                defaultValue="First unsaved draft"
                className="bg-card rounded-ctl w-full border p-2"
              />
            </label>
          }
          second={
            <label>
              Second draft
              <input
                defaultValue="Second unsaved draft"
                className="bg-card rounded-ctl w-full border p-2"
              />
            </label>
          }
        />
      </div>
    );
  },
};
