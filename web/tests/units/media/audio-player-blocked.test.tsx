import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { flushSync } from "react-dom";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import i18n from "@/lib/i18n";

const SENTENCE =
  "Trình duyệt chưa phát được âm thanh. Hãy bấm phát lại; nếu vẫn không nghe được, hãy kiểm tra quyền phát âm thanh của trang này.";
const SENTENCE_IN_ENGLISH =
  "Your browser did not start the audio. Press play again; if it still does not play, check this site's sound permission.";

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

afterEach(async () => {
  vi.restoreAllMocks();
  await act(() => i18n.changeLanguage("vi"));
});

const audio = () => document.querySelector("audio") as HTMLAudioElement;
const pressPlay = () => fireEvent.click(screen.getByRole("button", { name: "Phát" }));

function refuse(name: string) {
  play.mockImplementation(() => Promise.reject(new DOMException("refused", name)));
}

function start() {
  paused = false;
  return Promise.resolve();
}

async function settle() {
  await act(async () => {
    await Promise.resolve();
  });
}

it("says the browser did not start the audio, and leaves the player in place", async () => {
  refuse("NotAllowedError");
  const onBlocked = vi.fn();
  render(
    <AudioPlayer
      src="/a.mp3"
      label="Audio"
      onBlocked={onBlocked}
      onRetry={() => undefined}
    />,
  );

  pressPlay();

  expect(await screen.findByRole("alert")).toHaveTextContent(SENTENCE);
  expect(screen.getByRole("button", { name: "Phát" })).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Thử lại" })).not.toBeInTheDocument();
  expect(onBlocked).toHaveBeenCalledTimes(1);
});

it("takes the sentence away when a later press plays", async () => {
  refuse("NotAllowedError");
  render(<AudioPlayer src="/a.mp3" label="Audio" />);
  pressPlay();
  await screen.findByRole("alert");

  play.mockImplementation(start);
  pressPlay();
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  fireEvent.play(audio());

  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Tạm dừng" })).toBeInTheDocument();
});

it("takes the sentence away when the audio starts without a press", async () => {
  refuse("NotAllowedError");
  render(<AudioPlayer src="/a.mp3" label="Audio" />);
  pressPlay();
  await screen.findByRole("alert");

  fireEvent.play(audio());

  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
});

it("shows nothing when the student paused before the audio started", async () => {
  refuse("AbortError");
  const onBlocked = vi.fn();
  render(
    <AudioPlayer
      src="/a.mp3"
      label="Audio"
      onBlocked={onBlocked}
      onRetry={() => undefined}
    />,
  );

  pressPlay();
  await settle();

  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Phát" })).toBeInTheDocument();
  expect(onBlocked).not.toHaveBeenCalled();
});

it("still shows the expired-link card when the file cannot be played", async () => {
  refuse("NotSupportedError");
  render(<AudioPlayer src="/a.mp3" label="Audio" onRetry={() => undefined} />);

  pressPlay();

  expect(await screen.findByRole("alert")).toHaveTextContent("hết hạn");
  expect(screen.getByRole("button", { name: "Thử lại" })).toBeInTheDocument();
});

it("still shows it for a rejection that is not a DOMException", async () => {
  play.mockImplementation(() => Promise.reject(new Error("x")));
  render(<AudioPlayer src="/a.mp3" label="Audio" onRetry={() => undefined} />);

  pressPlay();

  expect(await screen.findByRole("alert")).toHaveTextContent("hết hạn");
  expect(screen.getByRole("button", { name: "Thử lại" })).toBeInTheDocument();
});

it("says it in English", async () => {
  await i18n.changeLanguage("en");
  refuse("NotAllowedError");
  render(<AudioPlayer src="/a.mp3" label="Audio" />);

  fireEvent.click(screen.getByRole("button", { name: "Play" }));

  expect(await screen.findByRole("alert")).toHaveTextContent(SENTENCE_IN_ENGLISH);
});

it("still calls play() first, before the count and before anything is set", async () => {
  refuse("NotAllowedError");
  const onPlay = vi.fn();
  const onBlocked = vi.fn();
  const onSeekBlocked = vi.fn();
  render(
    <AudioPlayer
      src="/a.mp3"
      label="Audio"
      onPlay={onPlay}
      onBlocked={onBlocked}
      onSeekBlocked={onSeekBlocked}
    />,
  );
  pressPlay();
  await screen.findByRole("alert");
  play.mockClear();
  onPlay.mockClear();
  let sentenceShownWhenPlayRan = false;
  play.mockImplementation(() => {
    flushSync(() => undefined);
    sentenceShownWhenPlayRan = screen.queryByRole("alert") !== null;
    return start();
  });

  pressPlay();

  expect(play).toHaveBeenCalledTimes(1);
  expect(onPlay).toHaveBeenCalledTimes(1);
  expect(play.mock.invocationCallOrder[0]).toBeLessThan(
    onPlay.mock.invocationCallOrder[0]!,
  );
  expect(sentenceShownWhenPlayRan).toBe(true);
});
