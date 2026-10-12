import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import "@/lib/i18n";

let play: ReturnType<typeof vi.fn>;
let pause: ReturnType<typeof vi.fn>;
let paused = true;

beforeEach(() => {
  paused = true;
  play = vi.fn(() => {
    paused = false;
    return Promise.resolve();
  });
  pause = vi.fn(() => {
    paused = true;
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
    get: () => paused,
  });
});

afterEach(() => vi.restoreAllMocks());

const audio = () => document.querySelector("audio") as HTMLAudioElement;
const pressPlay = () => fireEvent.click(screen.getByRole("button", { name: "Phát" }));

function refuse(name: string) {
  play.mockImplementation(() => Promise.reject(new DOMException("refused", name)));
}

it("reports the end of playback once", () => {
  const onEnded = vi.fn();
  render(<AudioPlayer src="/a.mp3" label="Audio" onEnded={onEnded} />);

  pressPlay();
  expect(onEnded).not.toHaveBeenCalled();

  fireEvent.ended(audio());
  expect(onEnded).toHaveBeenCalledTimes(1);
});

it("reports a play the browser blocked, and still says it could not play", async () => {
  refuse("NotAllowedError");
  const onBlocked = vi.fn();
  render(<AudioPlayer src="/a.mp3" label="Audio" onBlocked={onBlocked} />);

  pressPlay();

  expect(await screen.findByRole("alert")).toBeInTheDocument();
  expect(onBlocked).toHaveBeenCalledTimes(1);
});

it.each(["NotSupportedError"])("does not report %s as blocked", async (name) => {
  refuse(name);
  const onBlocked = vi.fn();
  render(<AudioPlayer src="/a.mp3" label="Audio" onBlocked={onBlocked} />);

  pressPlay();

  expect(await screen.findByRole("alert")).toBeInTheDocument();
  expect(onBlocked).not.toHaveBeenCalled();
});

it("still calls play() in the click's own tick with the new callbacks passed", () => {
  const onPlay = vi.fn();
  const onEnded = vi.fn();
  const onBlocked = vi.fn();
  render(
    <AudioPlayer
      src="/a.mp3"
      label="Audio"
      onPlay={onPlay}
      onEnded={onEnded}
      onBlocked={onBlocked}
    />,
  );

  pressPlay();

  expect(play).toHaveBeenCalledTimes(1);
  expect(onPlay).toHaveBeenCalledTimes(1);
  expect(play.mock.invocationCallOrder[0]).toBeLessThan(
    onPlay.mock.invocationCallOrder[0]!,
  );
  expect(onEnded).not.toHaveBeenCalled();
  expect(onBlocked).not.toHaveBeenCalled();
});

it("survives a parent that passes new functions on every render", () => {
  const added = vi.spyOn(EventTarget.prototype, "addEventListener");
  const listeners = () =>
    added.mock.calls.filter(
      ([type], call) => type === "ended" && added.mock.contexts[call] === audio(),
    ).length;
  const first = vi.fn();
  const latest = vi.fn();
  const { rerender } = render(
    <AudioPlayer src="/a.mp3" label="Audio" onEnded={first} />,
  );
  const mounted = listeners();
  expect(mounted).toBeGreaterThan(0);

  rerender(<AudioPlayer src="/a.mp3" label="Audio" onEnded={latest} />);
  fireEvent.ended(audio());

  expect(latest).toHaveBeenCalledTimes(1);
  expect(first).not.toHaveBeenCalled();
  expect(listeners()).toBe(mounted);
});
