import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { MediaCard } from "@/features/media/components/MediaCard";
import type { LibraryAsset } from "@/features/media/api";
import "@/lib/i18n";

/**
 * The media card is the second place a teacher starts audio, beside
 * AudioPlayer, and is held to the same rule (audio-player.test.tsx): Safari
 * honours play() only inside the gesture, so nothing may be awaited above it.
 * jsdom plays nothing, so the element's methods are the seam, as there.
 */
let play: ReturnType<typeof vi.fn>;
let pause: ReturnType<typeof vi.fn>;
let paused = true;

beforeEach(() => {
  paused = true;
  play = vi.fn(function (this: HTMLMediaElement) {
    paused = false;
    this.dispatchEvent(new Event("play"));
    return Promise.resolve();
  });
  pause = vi.fn(function (this: HTMLMediaElement) {
    if (!paused) this.dispatchEvent(new Event("pause"));
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

const ASSET: LibraryAsset = {
  id: "018f0000-0000-7000-8000-0000000000e1",
  kind: "audio",
  url: "https://example.test/part-1.mp3",
  mimeType: "audio/mpeg",
  bytes: 3_984_588,
  durationMs: 252_000,
  originalFilename: "part-1.mp3",
  createdAt: "2026-10-01T00:00:00Z",
  displayName: "Cambridge 15 · Test 2 · Part 1.mp3",
  defaultMaxPlays: 2,
  width: null,
  height: null,
  questionCount: 4,
  usageCount: 0,
  usedIn: [],
};

function renderCard(props: { active?: boolean; onPlay?: (id: string) => void } = {}) {
  return render(
    <MediaCard
      asset={ASSET}
      active={props.active ?? false}
      onPlay={props.onPlay ?? vi.fn()}
      onExpired={vi.fn()}
      onRename={vi.fn()}
      onReplace={vi.fn()}
      onDelete={vi.fn()}
    />,
  );
}

describe("the media card's play button", () => {
  it("calls play() in the same synchronous tick as the click", () => {
    renderCard();

    fireEvent.click(
      screen.getByRole("button", { name: "Phát Cambridge 15 · Test 2 · Part 1.mp3" }),
    );
    expect(play).toHaveBeenCalledTimes(1);
  });

  it("tells the page which file plays only after play() has run", () => {
    const order: string[] = [];
    play.mockImplementation(function (this: HTMLMediaElement) {
      order.push("play");
      paused = false;
      return Promise.resolve();
    });
    renderCard({ onPlay: (id) => order.push(`onPlay ${id}`) });

    fireEvent.click(screen.getByRole("button", { name: /^Phát / }));
    expect(order).toEqual(["play", `onPlay ${ASSET.id}`]);
  });

  it("pauses on the second click and offers play again", () => {
    renderCard({ active: true });

    fireEvent.click(screen.getByRole("button", { name: /^Phát / }));
    fireEvent.click(screen.getByRole("button", { name: /^Tạm dừng / }));

    expect(pause).toHaveBeenCalled();
    expect(screen.getByRole("button", { name: /^Phát / })).toBeVisible();
  });

  it("pauses when another card becomes the one playing", () => {
    const { rerender } = renderCard({ active: true });
    fireEvent.click(screen.getByRole("button", { name: /^Phát / }));
    pause.mockClear();

    rerender(
      <MediaCard
        asset={ASSET}
        active={false}
        onPlay={vi.fn()}
        onExpired={vi.fn()}
        onRename={vi.fn()}
        onReplace={vi.fn()}
        onDelete={vi.fn()}
      />,
    );

    expect(pause).toHaveBeenCalledTimes(1);
  });

  it("never loads or plays the file before the click", () => {
    renderCard();

    expect(document.querySelector("audio")?.getAttribute("preload")).toBe("none");
    expect(play).not.toHaveBeenCalled();
  });
});
