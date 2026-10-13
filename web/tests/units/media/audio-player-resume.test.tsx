import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import { forgetAllPlaces, keepPlace, recallPlace } from "@/features/media/playback";
import "@/lib/i18n";

/**
 * jsdom plays nothing, so these tests drive the element's clock by hand:
 * `currentTime` and the media events stand in for playback, and `play`,
 * `pause` and `paused` are stubbed, as the canary does.
 */
let play: ReturnType<typeof vi.fn>;
let pause: ReturnType<typeof vi.fn>;
const sounding = new Set<HTMLMediaElement>();

beforeEach(() => {
  sounding.clear();
  play = vi.fn(function (this: HTMLMediaElement) {
    sounding.add(this);
    return Promise.resolve();
  });
  pause = vi.fn(function (this: HTMLMediaElement) {
    sounding.delete(this);
  });
  Object.defineProperty(HTMLMediaElement.prototype, "play", {
    configurable: true,
    value: play,
  });
  Object.defineProperty(HTMLMediaElement.prototype, "pause", {
    configurable: true,
    value: pause,
  });
  Object.defineProperty(HTMLMediaElement.prototype, "paused", {
    configurable: true,
    get(this: HTMLMediaElement) {
      return !sounding.has(this);
    },
  });
});

afterEach(() => {
  forgetAllPlaces();
  vi.restoreAllMocks();
});

const audio = () => document.querySelector("audio") as HTMLAudioElement;
const press = (name: string) => fireEvent.click(screen.getByRole("button", { name }));

function playTo(seconds: number) {
  audio().currentTime = seconds;
  fireEvent.timeUpdate(audio());
}

function stop() {
  sounding.delete(audio());
  fireEvent.pause(audio());
}

function end() {
  sounding.delete(audio());
  fireEvent.ended(audio());
}

function player(props: Partial<Parameters<typeof AudioPlayer>[0]> = {}) {
  return (
    <AudioPlayer
      layout="card"
      src="/a.mp3"
      label="Recording"
      durationMs={154_000}
      allowSeek={false}
      plays={{ played: 0, limit: 2 }}
      {...props}
    />
  );
}

it("counts one play for a pause and a resume", () => {
  const onPlay = vi.fn();
  const onResume = vi.fn();
  render(player({ onPlay, onResume }));

  press("Phát");
  fireEvent.play(audio());
  playTo(5);
  stop();
  press("Nghe tiếp");

  expect(play).toHaveBeenCalledTimes(2);
  expect(onPlay).toHaveBeenCalledTimes(1);
  expect(onResume).toHaveBeenCalledTimes(1);
});

it("counts a second start after the end as a second play", () => {
  const onPlay = vi.fn();
  render(player({ onPlay, plays: { played: 1, limit: 2 } }));

  press("Phát lại");
  playTo(30);
  end();
  press("Phát lại");

  expect(onPlay).toHaveBeenCalledTimes(2);
});

it("counts a start from 0:00 again, as the deck does, after a pause before any sound", () => {
  const onPlay = vi.fn();
  const onResume = vi.fn();
  render(player({ onPlay, onResume }));

  press("Phát");
  stop();
  press("Phát");

  expect(onPlay).toHaveBeenCalledTimes(2);
  expect(onResume).not.toHaveBeenCalled();
});

it("spends nothing on a move inside a play in progress when seeking is allowed", () => {
  const onPlay = vi.fn();
  const onResume = vi.fn();
  render(player({ allowSeek: true, onPlay, onResume }));

  press("Phát");
  playTo(10);
  stop();
  fireEvent.change(screen.getByRole("slider"), { target: { value: "60" } });
  press("Nghe tiếp");

  expect(audio().currentTime).toBe(60);
  expect(onPlay).toHaveBeenCalledTimes(1);
  expect(onResume).toHaveBeenCalledTimes(1);
});

it("keeps the place of a play in progress when it goes away, and forgets it at the end", () => {
  const { unmount } = render(player({ place: "recording:r1" }));
  press("Phát");
  playTo(12);
  unmount();

  expect(pause).toHaveBeenCalled();
  expect(recallPlace("recording:r1")).toBe(12);

  render(player({ place: "recording:r1" }));
  end();
  expect(recallPlace("recording:r1")).toBeNull();
});

it("keeps no place for a play that has not started to sound", () => {
  const { unmount } = render(player({ place: "recording:r1" }));
  press("Phát");
  unmount();
  expect(recallPlace("recording:r1")).toBeNull();
});

it("resumes a remembered play with play() in the click's own tick, and spends no play", () => {
  keepPlace("recording:r1", 37);
  const onPlay = vi.fn();
  const onResume = vi.fn();
  render(player({ place: "recording:r1", onPlay, onResume }));

  expect(screen.getByText("0:37 / 2:34")).toBeInTheDocument();
  expect(screen.getByText("Đã tạm dừng")).toBeInTheDocument();
  press("Nghe tiếp");
  expect(play).toHaveBeenCalledTimes(1);
  expect(play.mock.invocationCallOrder[0]).toBeLessThan(
    onResume.mock.invocationCallOrder[0]!,
  );

  fireEvent.loadedMetadata(audio());
  expect(audio().currentTime).toBe(37);
  playTo(37.25);

  expect(onPlay).not.toHaveBeenCalled();
  expect(recallPlace("recording:r1")).toBe(37);
});

it("puts the place back at mount when the element already has its metadata", () => {
  keepPlace("question:q1", 21);
  vi.spyOn(HTMLMediaElement.prototype, "readyState", "get").mockReturnValue(
    HTMLMediaElement.HAVE_METADATA,
  );
  render(player({ place: "question:q1" }));
  expect(audio().currentTime).toBe(21);
});

it("counts a start from an empty memory", () => {
  const onPlay = vi.fn();
  const onResume = vi.fn();
  render(player({ place: "recording:r1", onPlay, onResume }));

  press("Phát");

  expect(onPlay).toHaveBeenCalledTimes(1);
  expect(onResume).not.toHaveBeenCalled();
});

it("does not report a restore as a blocked seek", () => {
  keepPlace("recording:r1", 37);
  const onSeekBlocked = vi.fn();
  render(player({ place: "recording:r1", onSeekBlocked }));

  fireEvent.loadedMetadata(audio());
  fireEvent.seeking(audio());

  expect(audio().currentTime).toBe(37);
  expect(onSeekBlocked).not.toHaveBeenCalled();
});

it("still refuses and reports a jump the student makes after a restore", () => {
  keepPlace("recording:r1", 37);
  const onSeekBlocked = vi.fn();
  render(player({ place: "recording:r1", onSeekBlocked }));
  fireEvent.loadedMetadata(audio());

  audio().currentTime = 90;
  fireEvent.seeking(audio());

  expect(audio().currentTime).toBe(37);
  expect(onSeekBlocked).toHaveBeenCalledTimes(1);
});

it("counts a resume as a play, and forgets the place, when playback runs short of it", () => {
  keepPlace("recording:r1", 37);
  const onPlay = vi.fn();
  const onResume = vi.fn();
  render(player({ place: "recording:r1", onPlay, onResume }));

  press("Nghe tiếp");
  expect(onPlay).not.toHaveBeenCalled();
  playTo(0.3);

  expect(onResume).toHaveBeenCalledTimes(1);
  expect(onPlay).toHaveBeenCalledTimes(1);
  expect(recallPlace("recording:r1")).toBeNull();
  playTo(0.6);
  expect(onPlay).toHaveBeenCalledTimes(1);
});

it("pauses the player that was sounding when another one starts", () => {
  render(
    <>
      {player({ label: "First" })}
      {player({ label: "Second" })}
    </>,
  );
  const [first, second] = [...document.querySelectorAll("audio")];

  const start = (name: string) =>
    fireEvent.click(
      within(screen.getByRole("group", { name })).getByRole("button", { name: "Phát" }),
    );
  start("First");
  fireEvent.play(first!);
  pause.mockClear();
  start("Second");
  fireEvent.play(second!);

  expect(pause).toHaveBeenCalledTimes(1);
  expect(pause.mock.contexts[0]).toBe(first);
  expect(first!.paused).toBe(true);
  expect(second!.paused).toBe(false);
});
