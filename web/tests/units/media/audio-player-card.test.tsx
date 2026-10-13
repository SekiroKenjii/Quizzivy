import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { AudioPlayer, type PlayCount } from "@/features/media/components/AudioPlayer";
import { forgetAllPlaces } from "@/features/media/playback";
import i18n from "@/lib/i18n";

let play: ReturnType<typeof vi.fn>;
let paused = true;

beforeEach(() => {
  paused = true;
  play = vi.fn(() => {
    paused = false;
    return Promise.resolve();
  });
  Object.defineProperty(HTMLMediaElement.prototype, "play", {
    configurable: true,
    value: play,
  });
  Object.defineProperty(HTMLMediaElement.prototype, "pause", {
    configurable: true,
    value: vi.fn(() => {
      paused = true;
    }),
  });
  Object.defineProperty(HTMLMediaElement.prototype, "paused", {
    configurable: true,
    get: () => paused,
  });
});

afterEach(async () => {
  forgetAllPlaces();
  vi.restoreAllMocks();
  await act(() => i18n.changeLanguage("vi"));
});

const audio = () => document.querySelector("audio") as HTMLAudioElement;
const card = () => screen.getByRole("group", { name: "Asking the way" });
const button = () => within(card()).getByRole("button");

function mount(plays: PlayCount = { played: 0, limit: 2 }, allowSeek = false) {
  return render(
    <AudioPlayer
      layout="card"
      src="/a.mp3"
      label="Asking the way"
      durationMs={154_000}
      allowSeek={allowSeek}
      plays={plays}
    />,
  );
}

function finish() {
  audio().currentTime = 154;
  fireEvent.timeUpdate(audio());
  paused = true;
  fireEvent.ended(audio());
}

describe("the recording card", () => {
  it("calls play() in the same synchronous tick as the click", () => {
    const onPlay = vi.fn();
    render(
      <AudioPlayer layout="card" src="/a.mp3" label="Asking the way" onPlay={onPlay} />,
    );
    fireEvent.click(button());
    expect(play).toHaveBeenCalledTimes(1);
    expect(play.mock.invocationCallOrder[0]).toBeLessThan(
      onPlay.mock.invocationCallOrder[0]!,
    );
  });

  it("is the deck's card: a 52px round button, a 6px track in the accent, the time and the counter", () => {
    mount();
    expect(card()).toHaveClass(
      "bg-card",
      "shadow-card",
      "rounded-2xl",
      "border",
      "p-4.5",
    );
    expect(button()).toHaveAccessibleName("Phát");
    expect(button()).toHaveClass("size-13", "rounded-full", "bg-primary");
    expect(button().querySelector("svg")).toHaveClass("size-5.5");
    const track = card().querySelector(".bg-muted")!;
    expect(track).toHaveClass("h-1.5");
    expect(track.firstElementChild).toHaveClass("bg-brand");
    expect(within(card()).getByText("0:00 / 2:34")).toHaveClass("whitespace-nowrap");
    expect(within(card()).getByText("Lượt nghe 1/2").parentElement).toHaveClass(
      "text-meta",
      "tabular-nums",
    );
    expect(within(card()).getByText("Bấm phát khi bạn đã sẵn sàng.")).toHaveClass(
      "text-sm",
      "text-muted-fg",
    );
    expect(screen.queryByRole("slider")).toBeNull();
  });

  it("lays the seek control over the track when skipping ahead is allowed", () => {
    mount({ played: 0, limit: 2 }, true);
    const slider = screen.getByRole("slider", { name: "Tua" });
    expect(slider.parentElement).toContainElement(card().querySelector(".bg-muted"));
  });

  it("reads Playing, then Paused with Resume, while a play is in progress", () => {
    mount({ played: 1, limit: 2 });
    fireEvent.click(button());
    fireEvent.play(audio());
    expect(button()).toHaveAccessibleName("Tạm dừng");
    expect(within(card()).getByText("Đang phát")).toBeInTheDocument();

    audio().currentTime = 61;
    fireEvent.timeUpdate(audio());
    paused = true;
    fireEvent.pause(audio());
    expect(button()).toHaveAccessibleName("Nghe tiếp");
    expect(within(card()).getByText("Đã tạm dừng")).toBeInTheDocument();
    expect(within(card()).getByText("1:01 / 2:34")).toBeInTheDocument();
    expect(within(card()).getByText("Lượt nghe 1/2")).toBeInTheDocument();
  });

  it("reads Finished with the plays left, and Play again, after a play ends", () => {
    mount({ played: 1, limit: 2 });
    fireEvent.click(button());
    finish();
    expect(button()).toHaveAccessibleName("Phát lại");
    expect(within(card()).getByText("Đã nghe xong · còn 1 lượt")).toBeInTheDocument();
    expect(within(card()).getByText("2:34 / 2:34")).toBeInTheDocument();
    expect(card().querySelector(".bg-brand")).toHaveStyle({ width: "100%" });
  });

  it("stays live at the limit, with no lock, and says the extra plays are recorded", () => {
    mount({ played: 2, limit: 2 });
    expect(button()).toHaveAccessibleName("Phát lại");
    expect(button()).toBeEnabled();
    expect(
      within(card()).getByText("Bạn đã dùng hết 2 lượt nghe."),
    ).toBeInTheDocument();
    expect(within(card()).getByText("Đã nghe 2/2 lượt")).toBeInTheDocument();
    expect(
      within(card()).getByText(
        "Bạn vẫn có thể nghe tiếp. Lượt nghe thêm được ghi lại để giáo viên xem.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(button());
    expect(play).toHaveBeenCalledTimes(1);
  });

  it("reads Play n of N during the last allowed play, and Played n/N past it", () => {
    const at = (played: number) => (
      <AudioPlayer
        layout="card"
        src="/a.mp3"
        label="Asking the way"
        durationMs={154_000}
        allowSeek={false}
        plays={{ played, limit: 2 }}
      />
    );
    const { rerender } = render(at(1));
    fireEvent.click(button());
    fireEvent.play(audio());
    rerender(at(2));
    expect(within(card()).getByText("Lượt nghe 2/2")).toBeInTheDocument();
    expect(within(card()).getByText("Đang phát")).toBeInTheDocument();
    rerender(at(3));
    expect(within(card()).getByText("Đã nghe 3/2 lượt")).toBeInTheDocument();
    expect(within(card()).getByText("Đang phát")).toBeInTheDocument();
  });

  it("counts plays with no limit as Play n, and finishes as Finished", () => {
    mount({ played: 1, limit: null });
    expect(within(card()).getByText("Lượt nghe 1")).toBeInTheDocument();
    expect(within(card()).getByText("Đã nghe xong")).toBeInTheDocument();
  });

  it("puts the blocked sentence in the status line's place", async () => {
    play.mockImplementation(() =>
      Promise.reject(new DOMException("blocked", "NotAllowedError")),
    );
    mount();
    fireEvent.click(button());
    const alert = await within(card()).findByRole("alert");
    expect(alert).toHaveTextContent("Trình duyệt chưa phát được âm thanh.");
    expect(within(card()).queryByText("Bấm phát khi bạn đã sẵn sàng.")).toBeNull();
    expect(button()).toHaveAccessibleName("Phát");
  });

  it("is the deck's English", async () => {
    await act(() => i18n.changeLanguage("en"));
    const { unmount } = mount({ played: 0, limit: 2 });
    expect(button()).toHaveAccessibleName("Play");
    expect(within(card()).getByText("Play 1 of 2")).toBeInTheDocument();
    expect(
      within(card()).getByText("Press play when you are ready."),
    ).toBeInTheDocument();
    fireEvent.click(button());
    fireEvent.play(audio());
    expect(within(card()).getByText("Playing")).toBeInTheDocument();
    unmount();

    mount({ played: 1, limit: 2 });
    expect(button()).toHaveAccessibleName("Play again");
    expect(within(card()).getByText("Finished · 1 play left")).toBeInTheDocument();
  });

  it("names the limit in English, in the singular and the plural", async () => {
    await act(() => i18n.changeLanguage("en"));
    const { unmount } = mount({ played: 2, limit: 2 });
    expect(within(card()).getByText("You have used all 2 plays.")).toBeInTheDocument();
    expect(within(card()).getByText("Played 2/2 times")).toBeInTheDocument();
    unmount();
    mount({ played: 1, limit: 1 });
    expect(within(card()).getByText("You have used your 1 play.")).toBeInTheDocument();
  });
});
