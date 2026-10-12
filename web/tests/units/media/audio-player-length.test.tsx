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

function loadLength(seconds: number) {
  Object.defineProperty(audio(), "duration", { configurable: true, value: seconds });
  fireEvent.loadedMetadata(audio());
}

function moveTo(seconds: number) {
  audio().currentTime = seconds;
  fireEvent.timeUpdate(audio());
}

it("shows the probed length before and after the file's own length is known", () => {
  render(<AudioPlayer src="/a.mp3" label="Audio" durationMs={10_004} />);
  expect(screen.getByText("0:00 / 0:10")).toBeInTheDocument();

  loadLength(9.98);
  expect(screen.getByText("0:00 / 0:10")).toBeInTheDocument();
});

it("falls back to the file's own length when no probed length was given", () => {
  render(<AudioPlayer src="/a.mp3" label="Audio" />);
  expect(screen.getByText("0:00 / 0:00")).toBeInTheDocument();

  loadLength(9.98);
  expect(screen.getByText("0:00 / 0:09")).toBeInTheDocument();
});

it("seeks over the file's own length", () => {
  render(<AudioPlayer src="/a.mp3" label="Audio" durationMs={10_004} allowSeek />);

  loadLength(9.98);
  expect(screen.getByRole("slider")).toHaveAttribute("max", "9.98");
});

it("never shows a position past the total", () => {
  render(<AudioPlayer src="/a.mp3" label="Audio" durationMs={9_000} />);
  loadLength(9.98);

  moveTo(9.9);
  expect(screen.getByText("0:09 / 0:09")).toBeInTheDocument();
});

it("never shows a position past the total when the file runs a second longer", () => {
  render(<AudioPlayer src="/a.mp3" label="Audio" durationMs={9_900} />);
  loadLength(10.02);

  moveTo(10.01);
  expect(screen.getByText("0:09 / 0:09")).toBeInTheDocument();
});
