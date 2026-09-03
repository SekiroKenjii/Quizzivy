import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import "@/lib/i18n";

/**
 * jsdom implements no media playback at all, so the element's own methods are
 * the seam. Stubbing them is not a shortcut here -- what these tests are about
 * is WHEN and WHETHER the component calls them, which is exactly what a stub
 * can observe and a real element could not.
 */
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

describe("the player at rest", () => {
  it("offers play and the duration it was handed, without loading anything", () => {
    render(<AudioPlayer src="/a.mp3" label="Audio" durationMs={110_000} />);

    expect(screen.getByRole("button", { name: "Phát" })).toBeInTheDocument();
    expect(screen.getByText("0:00 / 1:50")).toBeInTheDocument();
    expect(audio().getAttribute("preload")).toBe("none");
    expect(play).not.toHaveBeenCalled();
  });

  it("never autoplays", () => {
    render(<AudioPlayer src="/a.mp3" label="Audio" preload="metadata" />);
    expect(play).not.toHaveBeenCalled();
  });
});

/**
 * The iOS rule, and the reason the play count is fired after rather than before:
 * Safari only honours play() from inside the gesture, so nothing may be awaited
 * above it.
 */
describe("starting playback", () => {
  it("calls play() in the same synchronous tick as the click", () => {
    render(<AudioPlayer src="/a.mp3" label="Audio" />);

    fireEvent.click(screen.getByRole("button", { name: "Phát" }));
    expect(play).toHaveBeenCalledTimes(1);
  });

  it("counts the play without waiting for it", () => {
    const onPlay = vi.fn();
    render(<AudioPlayer src="/a.mp3" label="Audio" onPlay={onPlay} />);

    fireEvent.click(screen.getByRole("button", { name: "Phát" }));
    expect(onPlay).toHaveBeenCalledTimes(1);
  });

  it("shows pause once the element reports it is playing", () => {
    render(<AudioPlayer src="/a.mp3" label="Audio" />);
    fireEvent.click(screen.getByRole("button", { name: "Phát" }));
    fireEvent.play(audio());

    expect(screen.getByRole("button", { name: "Tạm dừng" })).toBeInTheDocument();
  });
});

describe("plays remaining", () => {
  it("announces the count, because it is otherwise only visible", () => {
    render(<AudioPlayer src="/a.mp3" label="Audio" hint="Còn 2 lượt nghe" />);
    const hint = screen.getByText("Còn 2 lượt nghe");
    expect(hint).toHaveAttribute("aria-live", "polite");
  });

  // Exhausted is a number, not a disabled button.
  it("still plays when the hint says none are left", () => {
    render(<AudioPlayer src="/a.mp3" label="Audio" hint="Còn 0 lượt nghe" />);
    const button = screen.getByRole("button", { name: "Phát" });
    expect(button).not.toBeDisabled();
    fireEvent.click(button);
    expect(play).toHaveBeenCalledTimes(1);
  });
});

describe("seeking", () => {
  it("offers no seek control when the teacher disallowed it", () => {
    render(<AudioPlayer src="/a.mp3" label="Audio" allowSeek={false} />);
    expect(screen.queryByRole("slider")).not.toBeInTheDocument();
  });

  it("puts the position back and reports it, because OS controls still seek", () => {
    const onSeekBlocked = vi.fn();
    render(
      <AudioPlayer
        src="/a.mp3"
        label="Audio"
        allowSeek={false}
        onSeekBlocked={onSeekBlocked}
      />,
    );

    const element = audio();
    element.currentTime = 42;
    fireEvent.seeking(element);

    expect(element.currentTime).toBe(0);
    expect(onSeekBlocked).toHaveBeenCalledTimes(1);
  });

  it("leaves seeking alone when it is allowed", () => {
    const onSeekBlocked = vi.fn();
    render(
      <AudioPlayer
        src="/a.mp3"
        label="Audio"
        allowSeek
        onSeekBlocked={onSeekBlocked}
      />,
    );

    const element = audio();
    element.currentTime = 42;
    fireEvent.seeking(element);

    expect(element.currentTime).toBe(42);
    expect(onSeekBlocked).not.toHaveBeenCalled();
  });
});

describe("when the file will not load", () => {
  it("says the link expired and offers to fetch a new one", () => {
    const onRetry = vi.fn();
    render(<AudioPlayer src="/a.mp3" label="Audio" onRetry={onRetry} />);
    fireEvent.error(audio());

    expect(screen.getByRole("alert")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  // Keyed by src, so a freshly signed URL is not still wearing the old failure.
  it("clears the failure when a new url arrives", () => {
    const { rerender } = render(<AudioPlayer src="/a.mp3" label="Audio" />);
    fireEvent.error(audio());
    expect(screen.getByRole("alert")).toBeInTheDocument();

    rerender(<AudioPlayer src="/a.mp3?sig=fresh" label="Audio" />);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});

// §11.3: one instance per question, and navigating away releases it. Without
// this the element plays on through a route change, and on iOS the next
// question's audio then cannot start at all.
describe("leaving the question", () => {
  it("pauses on unmount", () => {
    const { unmount } = render(<AudioPlayer src="/a.mp3" label="Audio" />);
    fireEvent.click(screen.getByRole("button", { name: "Phát" }));
    unmount();
    expect(pause).toHaveBeenCalled();
  });
});
