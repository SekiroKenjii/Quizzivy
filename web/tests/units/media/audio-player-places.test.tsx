import { StrictMode, useEffect } from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import { forgetAllPlaces, keepPlace, recallPlace } from "@/features/media/playback";
import "@/lib/i18n";

/**
 * A host that moves from one place to another can keep the same element in
 * the tree (two audio questions in a row). These tests hold the player to
 * one place per instance, and to the memory it was opened under. jsdom plays
 * nothing: `currentTime` and the media events stand in for playback, and
 * `play`, `pause` and `paused` are stubbed, as the canary does.
 */
const sounding = new Set<HTMLMediaElement>();

beforeEach(() => {
  sounding.clear();
  Object.defineProperty(HTMLMediaElement.prototype, "play", {
    configurable: true,
    value: vi.fn(function (this: HTMLMediaElement) {
      sounding.add(this);
      return Promise.resolve();
    }),
  });
  Object.defineProperty(HTMLMediaElement.prototype, "pause", {
    configurable: true,
    value: vi.fn(function (this: HTMLMediaElement) {
      sounding.delete(this);
    }),
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
const button = () => screen.getByRole("button");

async function click(element: HTMLElement) {
  await act(async () => {
    fireEvent.click(element);
  });
}

async function startAndPlayTo(seconds: number) {
  await click(screen.getByRole("button", { name: "Phát" }));
  fireEvent.play(audio());
  audio().currentTime = seconds;
  fireEvent.timeUpdate(audio());
}

function card(
  place: string,
  handlers: { onPlay?: () => void; onResume?: () => void } = {},
) {
  return (
    <AudioPlayer
      layout="card"
      src={`/${place.replace(":", "-")}.mp3`}
      label="R"
      durationMs={60_000}
      allowSeek={false}
      plays={{ played: 0, limit: 2 }}
      place={place}
      onPlay={handlers.onPlay}
      onResume={handlers.onResume}
    />
  );
}

function Page({
  onPlay,
  place = "recording:r1",
}: Readonly<{ onPlay: () => void; place?: string }>) {
  useEffect(() => () => forgetAllPlaces(), []);
  return card(place, { onPlay });
}

it("starts clean when its host gives it another place, with a new element", async () => {
  const onPlay = vi.fn();
  const onResume = vi.fn();
  const view = render(card("question:q1", { onPlay, onResume }));
  await startAndPlayTo(30);
  const first = audio();

  view.rerender(card("question:q2", { onPlay, onResume }));

  expect(audio()).not.toBe(first);
  expect(sounding.has(first)).toBe(false);
  expect(button()).toHaveAccessibleName("Phát");
  expect(screen.getByRole("group", { name: "R" })).toHaveAttribute(
    "data-phase",
    "ready",
  );
  expect(screen.getByText("0:00 / 1:00")).toBeInTheDocument();
  await click(button());
  expect(onPlay).toHaveBeenCalledTimes(2);
  expect(onResume).not.toHaveBeenCalled();
});

it("puts no other place's position on the next one", async () => {
  keepPlace("question:q1", 30);
  const onPlay = vi.fn();
  const onResume = vi.fn();
  const view = render(card("question:q1", { onPlay, onResume }));

  view.rerender(card("question:q2", { onPlay, onResume }));
  fireEvent.loadedMetadata(audio());

  expect(audio().currentTime).toBe(0);
  await click(button());
  expect(onPlay).toHaveBeenCalledTimes(1);
  expect(onResume).not.toHaveBeenCalled();
  expect(recallPlace("question:q1")).toBe(30);
});

it("keeps the place it leaves, and resumes there when its host comes back", async () => {
  const onPlay = vi.fn();
  const onResume = vi.fn();
  const view = render(card("question:q1", { onPlay, onResume }));
  await startAndPlayTo(30);

  view.rerender(card("question:q2", { onPlay, onResume }));
  expect(recallPlace("question:q1")).toBe(30);

  view.rerender(card("question:q1", { onPlay, onResume }));
  expect(button()).toHaveAccessibleName("Nghe tiếp");
  fireEvent.loadedMetadata(audio());
  expect(audio().currentTime).toBe(30);
  await click(button());
  expect(onPlay).toHaveBeenCalledTimes(1);
  expect(onResume).toHaveBeenCalledTimes(1);
});

it("stays the same player, still playing, while its place stays the same", async () => {
  const onPlay = vi.fn();
  const view = render(card("recording:r1", { onPlay }));
  await startAndPlayTo(12);
  const element = audio();

  view.rerender(
    <AudioPlayer
      layout="card"
      src="/recording-r1.mp3"
      label="R"
      durationMs={60_000}
      allowSeek={false}
      plays={{ played: 1, limit: 2 }}
      place="recording:r1"
      onPlay={onPlay}
    />,
  );

  expect(audio()).toBe(element);
  expect(sounding.has(element)).toBe(true);
  expect(button()).toHaveAccessibleName("Tạm dừng");
  expect(onPlay).toHaveBeenCalledTimes(1);
});

it("keeps no place when the memory it was opened under was forgotten before it went away", async () => {
  const onPlay = vi.fn();
  const page = render(<Page onPlay={onPlay} />);
  await startAndPlayTo(30);
  expect(onPlay).toHaveBeenCalledTimes(1);

  page.unmount();

  expect(recallPlace("recording:r1")).toBeNull();
});

it("spends a play on the first press under the next memory", async () => {
  const page = render(<Page onPlay={vi.fn()} />);
  await startAndPlayTo(30);
  page.unmount();

  const onPlay = vi.fn();
  render(<Page onPlay={onPlay} />);
  fireEvent.loadedMetadata(audio());

  expect(button()).toHaveAccessibleName("Phát");
  expect(audio().currentTime).toBe(0);
  await click(button());
  expect(onPlay).toHaveBeenCalledTimes(1);
});

it("keeps places under StrictMode, whose rehearsed unmount forgets them once at mount", async () => {
  const onPlay = vi.fn();
  const view = render(
    <StrictMode>
      <Page onPlay={onPlay} place="question:q1" />
    </StrictMode>,
  );
  await startAndPlayTo(30);

  view.rerender(
    <StrictMode>
      <Page onPlay={onPlay} place="question:q2" />
    </StrictMode>,
  );

  expect(recallPlace("question:q1")).toBe(30);
});
