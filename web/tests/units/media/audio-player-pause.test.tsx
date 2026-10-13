import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import { forgetAllPlaces } from "@/features/media/playback";
import i18n from "@/lib/i18n";

/**
 * jsdom plays nothing, so these tests drive the element by hand: `play`,
 * `pause` and `paused` are stubbed, as the canary does, and the media events
 * stand in for playback.
 */
let play: ReturnType<typeof vi.fn>;
let pause: ReturnType<typeof vi.fn>;
const sounding = new Set<HTMLMediaElement>();

beforeEach(() => {
  sounding.clear();
  play = vi.fn(function (this: HTMLMediaElement) {
    sounding.add(this);
    this.dispatchEvent(new Event("play"));
    return Promise.resolve();
  });
  pause = vi.fn(function (this: HTMLMediaElement) {
    if (!sounding.delete(this)) return;
    this.dispatchEvent(new Event("pause"));
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

afterEach(async () => {
  forgetAllPlaces();
  vi.restoreAllMocks();
  await act(() => i18n.changeLanguage("vi"));
});

const card = (name = "Part 2") => screen.getByRole("group", { name });
const button = (name = "Part 2") => within(card(name)).getByRole("button");
const audioOf = (name = "Part 2") =>
  card(name).querySelector("audio") as HTMLAudioElement;

function playTo(seconds: number, name = "Part 2") {
  audioOf(name).currentTime = seconds;
  fireEvent.timeUpdate(audioOf(name));
}

function player(props: Partial<Parameters<typeof AudioPlayer>[0]> = {}) {
  return (
    <AudioPlayer
      layout="card"
      src="/l2.mp3"
      label="Part 2"
      durationMs={190_000}
      allowSeek={false}
      allowPause={false}
      plays={{ played: 0, limit: 2 }}
      {...props}
    />
  );
}

describe("a recording that cannot be paused", () => {
  it("says so before the first play, and the button is live", () => {
    render(player());
    expect(
      within(card()).getByText(
        "Bấm phát khi bạn đã sẵn sàng. Bài nghe này không tạm dừng được sau khi bắt đầu.",
      ),
    ).toBeInTheDocument();
    expect(button()).toHaveAccessibleName("Phát");
    expect(button()).not.toHaveAttribute("aria-disabled");
  });

  it("calls play() in the same synchronous tick as the click", () => {
    const onPlay = vi.fn();
    render(player({ onPlay }));
    fireEvent.click(button());
    expect(play).toHaveBeenCalledTimes(1);
    expect(play.mock.invocationCallOrder[0]).toBeLessThan(
      onPlay.mock.invocationCallOrder[0]!,
    );
  });

  it("makes the button inert while playing: aria-disabled, dimmed, focusable, and a press does nothing", async () => {
    const user = userEvent.setup();
    render(player());
    fireEvent.click(button());
    playTo(8);

    expect(card()).toHaveAttribute("data-phase", "playing");
    expect(button()).toHaveAccessibleName("Tạm dừng");
    expect(button()).toHaveAttribute("aria-disabled", "true");
    expect(button()).not.toBeDisabled();
    expect(button()).toHaveClass("aria-disabled:opacity-45");

    fireEvent.click(button());
    button().focus();
    expect(button()).toHaveFocus();
    await user.keyboard("{Enter}");
    await user.keyboard(" ");

    expect(pause).not.toHaveBeenCalled();
    expect(card()).toHaveAttribute("data-phase", "playing");
    expect(
      within(card()).getByText("Đang phát · bài nghe này không tạm dừng được"),
    ).toBeInTheDocument();
  });

  it("reads the two sentences in English as the deck has them", async () => {
    await act(() => i18n.changeLanguage("en"));
    render(player());
    expect(
      within(card()).getByText(
        "Press play when you are ready. It can’t be paused once it starts.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(button());
    expect(
      within(card()).getByText("Playing · this recording can’t be paused"),
    ).toBeInTheDocument();
  });

  it("reads Paused when playback stops anyway, and Resume continues it for no play", () => {
    const onPlay = vi.fn();
    const onResume = vi.fn();
    render(player({ onPlay, onResume }));
    fireEvent.click(button());
    playTo(40);

    act(() => {
      sounding.delete(audioOf());
      fireEvent.pause(audioOf());
    });

    expect(card()).toHaveAttribute("data-phase", "paused");
    expect(within(card()).getByText("Đã tạm dừng")).toBeInTheDocument();
    expect(button()).toHaveAccessibleName("Nghe tiếp");
    expect(button()).not.toHaveAttribute("aria-disabled");

    fireEvent.click(button());

    expect(play).toHaveBeenCalledTimes(2);
    expect(onPlay).toHaveBeenCalledTimes(1);
    expect(onResume).toHaveBeenCalledTimes(1);
    expect(button()).toHaveAttribute("aria-disabled", "true");
  });

  it("is paused by another recording starting, and resumes for no play", () => {
    const onPlay = vi.fn();
    const onResume = vi.fn();
    render(
      <>
        {player({ onPlay, onResume })}
        {player({ label: "Part 1", src: "/l1.mp3", allowPause: true })}
      </>,
    );
    fireEvent.click(button());
    playTo(12);
    fireEvent.click(button("Part 1"));

    expect(card()).toHaveAttribute("data-phase", "paused");
    fireEvent.click(button());
    expect(onPlay).toHaveBeenCalledTimes(1);
    expect(onResume).toHaveBeenCalledTimes(1);
  });

  it("is paused by a lock, and resumes for no play once the lock lifts", () => {
    const onPlay = vi.fn();
    const onResume = vi.fn();
    const { rerender } = render(player({ onPlay, onResume }));
    fireEvent.click(button());
    playTo(20);

    rerender(player({ onPlay, onResume, disabled: true }));
    expect(card()).toHaveAttribute("data-phase", "paused");

    rerender(player({ onPlay, onResume }));
    fireEvent.click(button());
    expect(onPlay).toHaveBeenCalledTimes(1);
    expect(onResume).toHaveBeenCalledTimes(1);
  });

  it("comes back paused after its group was left, and resumes for no play", () => {
    const onPlay = vi.fn();
    const onResume = vi.fn();
    const { unmount } = render(player({ place: "recording:l2", onPlay }));
    fireEvent.click(button());
    playTo(33);
    unmount();

    render(player({ place: "recording:l2", onPlay, onResume }));
    expect(card()).toHaveAttribute("data-phase", "paused");
    expect(within(card()).getByText("0:33 / 3:10")).toBeInTheDocument();
    fireEvent.click(button());

    expect(onPlay).toHaveBeenCalledTimes(1);
    expect(onResume).toHaveBeenCalledTimes(1);
  });
});

describe("a recording that can be paused", () => {
  it("pauses on a press, with the plain sentences", () => {
    render(player({ allowPause: true }));
    expect(
      within(card()).getByText("Bấm phát khi bạn đã sẵn sàng."),
    ).toBeInTheDocument();
    fireEvent.click(button());
    expect(within(card()).getByText("Đang phát")).toBeInTheDocument();
    expect(button()).not.toHaveAttribute("aria-disabled");

    fireEvent.click(button());

    expect(pause).toHaveBeenCalledTimes(1);
    expect(card()).toHaveAttribute("data-phase", "ready");
  });

  it("pauses freely for a payload without the field, as from an API that predates it", () => {
    render(player({ allowPause: undefined }));
    fireEvent.click(button());
    playTo(5);
    expect(button()).not.toHaveAttribute("aria-disabled");

    fireEvent.click(button());

    expect(pause).toHaveBeenCalledTimes(1);
    expect(card()).toHaveAttribute("data-phase", "paused");
  });
});

describe("the row player", () => {
  it("keeps its button inert while a recording that cannot be paused plays", () => {
    render(<AudioPlayer src="/q.mp3" label="Question" size="sm" allowPause={false} />);
    const control = screen.getByRole("button", { name: "Phát" });
    fireEvent.click(control);
    const playing = screen.getByRole("button", { name: "Tạm dừng" });
    expect(playing).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(playing);
    expect(pause).not.toHaveBeenCalled();
  });
});
