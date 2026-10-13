import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  createEvent,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EnginePreview } from "@/features/take-test";
import {
  getAttempt,
  recordAudioPlay,
  recordGroupAudioPlay,
  saveAnswers,
  submitAttempt,
  type StudentGroup,
  type StudentQuestion,
  type StudentSection,
} from "@/features/take-test/api";
import { writeDraft } from "@/features/take-test/draft";
import { useGroupPlaybackStore } from "@/features/take-test/groupPlayback";
import { writeOpenQuestion } from "@/features/take-test/openQuestion";
import { useTakeTestStore } from "@/features/take-test/store";
import { EngineWidthContext } from "@/features/take-test/width";
import { enterFullscreen, exitFullscreen } from "@/features/integrity/fullscreen";
import {
  recordAudioEvent,
  recordSharedAudioEvent,
  useIntegrityMonitor,
} from "@/features/integrity/useIntegrityMonitor";
import { notify } from "@/lib/toast";
import "@/lib/i18n";

vi.mock("@/features/take-test/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/features/take-test/api")>();
  return {
    ...actual,
    getAttempt: vi.fn(),
    saveAnswers: vi.fn(),
    submitAttempt: vi.fn(),
    recordAudioPlay: vi.fn(),
    recordGroupAudioPlay: vi.fn(),
  };
});
vi.mock("@/features/take-test/draft", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/features/take-test/draft")>();
  return { ...actual, writeDraft: vi.fn() };
});
vi.mock("@/features/take-test/openQuestion", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@/features/take-test/openQuestion")>();
  return { ...actual, writeOpenQuestion: vi.fn() };
});
vi.mock("@/features/integrity/fullscreen", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@/features/integrity/fullscreen")>();
  return { ...actual, enterFullscreen: vi.fn(), exitFullscreen: vi.fn() };
});
vi.mock("@/features/integrity/useIntegrityMonitor", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@/features/integrity/useIntegrityMonitor")>();
  return {
    ...actual,
    useIntegrityMonitor: vi.fn(actual.useIntegrityMonitor),
    recordAudioEvent: vi.fn(),
    recordSharedAudioEvent: vi.fn(),
  };
});

const ID = (tail: string) => `018f0000-0000-7000-8000-00000000${tail}`;
const SECTION: StudentSection = { id: ID("5001"), title: "Phần 1", instructions: null };
const ASSET = ID("a001");

const audio = (id: string, url: string) => ({
  id,
  kind: "audio" as const,
  mimeType: "audio/mpeg" as const,
  bytes: 2048,
  durationMs: 10_000,
  originalFilename: "clip.mp3",
  createdAt: "2026-10-01T00:00:00.000Z",
  url,
});

const listening: StudentQuestion = {
  id: ID("q001"),
  sectionId: SECTION.id,
  type: "single_choice",
  prompt: "Listen and choose.",
  points: 1,
  options: [
    { id: ID("o011"), text: "At the post office" },
    { id: ID("o012"), text: "At the station" },
  ],
  media: audio(ID("m001"), "https://assets.example/q1.mp3"),
  audio: {
    maxPlays: 1,
    allowSeek: false,
    showTranscriptAfterSubmit: false,
    allowPause: true,
  },
};

const written: StudentQuestion = {
  id: ID("q002"),
  sectionId: SECTION.id,
  type: "short_answer",
  prompt: "Say where you live.",
  points: 1,
};

const grouped: StudentQuestion = {
  id: ID("q003"),
  sectionId: SECTION.id,
  type: "single_choice",
  prompt: "Where does the walk start?",
  points: 1,
  options: [
    { id: ID("o031"), text: "At the bridge" },
    { id: ID("o032"), text: "At the gate" },
  ],
};

const group: StudentGroup = {
  id: ID("g001"),
  sectionId: SECTION.id,
  title: "A guided walk",
  questionIds: [grouped.id],
  stimuli: [
    {
      id: ID("t001"),
      title: "The old town",
      content: {
        format: "semantic_v1",
        blocks: [
          {
            type: "paragraph",
            content: [
              { type: "text", text: "The walk starts at the bridge.", marks: [] },
            ],
          },
          { type: "audio", assetId: ASSET, label: "Tour guide" },
        ],
      },
      gaps: [],
    },
  ],
  recordings: [
    {
      id: ID("r001"),
      assetId: ASSET,
      policy: {
        maxPlays: 1,
        allowSeek: false,
        showTranscriptAfterSubmit: false,
        allowPause: true,
      },
    },
  ],
  assets: [audio(ASSET, "https://assets.example/walk.mp3")],
};

function Preview({ width }: Readonly<{ width: number }>) {
  return (
    <EngineWidthContext value={width}>
      <EnginePreview
        title="Unit 4 · Listening"
        questions={[listening, written, grouped]}
        sections={[SECTION]}
        groups={[group]}
      />
    </EngineWidthContext>
  );
}

const storeWrites = vi.fn();
const playbackWrites = vi.fn();
let unsubscribe: (() => void)[] = [];
let setItem: ReturnType<typeof vi.spyOn>;
let fetchSpy: ReturnType<typeof vi.spyOn>;
let requestFullscreen: ReturnType<typeof vi.fn>;

beforeEach(() => {
  storeWrites.mockClear();
  playbackWrites.mockClear();
  unsubscribe = [
    useTakeTestStore.subscribe(storeWrites),
    useGroupPlaybackStore.subscribe(playbackWrites),
  ];
  setItem = vi.spyOn(Storage.prototype, "setItem");
  fetchSpy = vi.spyOn(globalThis, "fetch");
  requestFullscreen = vi.fn(() => Promise.resolve());
  Object.defineProperty(Element.prototype, "requestFullscreen", {
    configurable: true,
    value: requestFullscreen,
  });
  Object.defineProperty(HTMLMediaElement.prototype, "play", {
    configurable: true,
    value: vi.fn(() => Promise.resolve()),
  });
  Object.defineProperty(HTMLMediaElement.prototype, "pause", {
    configurable: true,
    value: vi.fn(),
  });
});

afterEach(() => {
  for (const stop of unsubscribe) stop();
  vi.restoreAllMocks();
  vi.clearAllMocks();
});

function expectNoSideEffect() {
  expect(storeWrites).not.toHaveBeenCalled();
  expect(playbackWrites).not.toHaveBeenCalled();
  expect(useTakeTestStore.getState().answers).toEqual({});
  expect(useTakeTestStore.getState().audioPlays).toEqual({});
  expect(useGroupPlaybackStore.getState().pending).toEqual([]);
  expect(saveAnswers).not.toHaveBeenCalled();
  expect(submitAttempt).not.toHaveBeenCalled();
  expect(getAttempt).not.toHaveBeenCalled();
  expect(recordAudioPlay).not.toHaveBeenCalled();
  expect(recordGroupAudioPlay).not.toHaveBeenCalled();
  expect(writeDraft).not.toHaveBeenCalled();
  expect(writeOpenQuestion).not.toHaveBeenCalled();
  expect(setItem).not.toHaveBeenCalled();
  expect(fetchSpy).not.toHaveBeenCalled();
  expect(useIntegrityMonitor).not.toHaveBeenCalled();
  expect(recordAudioEvent).not.toHaveBeenCalled();
  expect(recordSharedAudioEvent).not.toHaveBeenCalled();
  expect(enterFullscreen).not.toHaveBeenCalled();
  expect(exitFullscreen).not.toHaveBeenCalled();
  expect(requestFullscreen).not.toHaveBeenCalled();
}

describe("the engine's preview", () => {
  it("answers, flags, plays and finishes without a store, a save, an integrity record, fullscreen or a play counted", async () => {
    const user = userEvent.setup();
    const info = vi.spyOn(notify, "info");
    render(<Preview width={1250} />);

    await user.click(screen.getByRole("button", { name: /Phát/ }));
    await user.click(screen.getByRole("radio", { name: /At the station/ }));
    await user.click(screen.getByRole("button", { name: "Đánh dấu xem lại" }));
    expect(screen.getByRole("button", { name: "Đã đánh dấu" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    await user.click(screen.getByRole("button", { name: "Câu sau" }));
    const field = screen.getByRole("textbox", { name: "Bài làm của bạn" });
    await user.type(field, "Hà Nội");
    const paste = createEvent.paste(field);
    fireEvent(field, paste);
    expect(paste.defaultPrevented).toBe(false);
    await user.click(screen.getByRole("button", { name: "Câu sau" }));
    await user.click(screen.getAllByRole("button", { name: /Phát/ })[0]!);
    await user.click(screen.getByRole("radio", { name: /At the bridge/ }));
    await user.click(screen.getByRole("button", { name: "Hoàn tất" }));

    expect(info).toHaveBeenCalledWith(
      "Đây là bản xem trước. Học viên nộp bài ở đây.",
      expect.anything(),
    );
    expect(screen.queryByRole("dialog")).toBeNull();
    expectNoSideEffect();
  });

  it("has no leave button and no Submit, but the chip, a pill with no time and a save line that claims no save", async () => {
    const user = userEvent.setup();
    render(<Preview width={1250} />);
    const header = screen.getByRole("banner");

    expect(within(header).queryByRole("button")).toBeNull();
    expect(screen.queryByRole("button", { name: "Thoát khỏi bài làm" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Nộp bài" })).toBeNull();
    const chip = within(header).getByText("Góc nhìn học viên");
    expect(chip).toHaveClass("h-7.5", "rounded-full", "bg-info-soft", "text-info-ink");
    expect(chip.querySelector("svg")).toHaveClass("size-3.5");
    const pill = within(header).getByRole("img", {
      name: "Thời gian làm bài được đặt khi giao bài",
    });
    expect(pill).toHaveClass("h-9", "rounded-full", "mx-auto");
    expect(pill.textContent).toBe("––:––");
    expect(screen.queryByRole("timer")).toBeNull();
    expect(within(header).getByText("Câu trả lời ở đây không được lưu.")).toBeVisible();

    await user.click(screen.getByRole("radio", { name: /At the station/ }));
    expect(screen.queryByText("Đang lưu…")).toBeNull();
    expect(screen.queryByText("Đã lưu tất cả câu trả lời")).toBeNull();
    expect(screen.queryByText(/Lượt rời trang/)).toBeNull();
    expect(screen.queryByRole("button", { name: /toàn màn hình/i })).toBeNull();
  });

  it("keeps what is chosen while it is open, and forgets it once it closes", async () => {
    const user = userEvent.setup();
    const { unmount } = render(<Preview width={1250} />);

    await user.click(screen.getByRole("radio", { name: /At the station/ }));
    await user.click(screen.getByRole("button", { name: "Câu sau" }));
    await user.type(screen.getByRole("textbox", { name: "Bài làm của bạn" }), "Huế");
    await user.click(screen.getByRole("button", { name: "Câu trước" }));
    expect(screen.getByRole("radio", { name: /At the station/ })).toBeChecked();
    await user.click(screen.getByRole("button", { name: "Câu sau" }));
    expect(screen.getByRole("textbox", { name: "Bài làm của bạn" })).toHaveValue("Huế");

    unmount();
    render(<Preview width={1250} />);
    expect(screen.getByRole("radio", { name: /At the station/ })).not.toBeChecked();
    expectNoSideEffect();
  });

  it("takes the engine's keys while the focus is inside it", async () => {
    const user = userEvent.setup();
    render(<Preview width={1250} />);

    await user.click(screen.getByRole("radio", { name: /At the post office/ }));
    await user.keyboard("b");
    expect(screen.getByRole("radio", { name: /At the station/ })).toBeChecked();
    await user.keyboard("f");
    expect(screen.getByRole("button", { name: "Đã đánh dấu" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("textbox", { name: "Bài làm của bạn" })).toBeVisible();
    expectNoSideEffect();
  });

  it("does not take the focus as it opens on a page", () => {
    const before = document.createElement("button");
    document.body.append(before);
    before.focus();
    render(<Preview width={1250} />);
    expect(before).toHaveFocus();
    before.remove();
  });

  it("draws the phone's engine at the phone frame's width, with the drawer and the grid button, and opens the sheet over itself", async () => {
    const user = userEvent.setup();
    const { container } = render(<Preview width={388} />);
    const header = screen.getByRole("banner");

    expect(within(header).queryByText("Unit 4 · Listening")).toBeNull();
    expect(within(header).getByText("Góc nhìn học viên")).toBeVisible();
    expect(screen.queryByRole("complementary", { name: "Các phần" })).toBeNull();
    const grid = screen.getByRole("button", { name: "Câu 1 · đã làm 0 trên 3" });

    await user.click(grid);
    const sheet = await screen.findByRole("dialog", { name: "Danh sách câu" });
    expect(container.firstElementChild).toContainElement(sheet);
    await user.click(within(sheet).getByRole("button", { name: /^Câu 3/ }));
    expect(await screen.findByRole("button", { name: "Mở ngữ liệu" })).toBeVisible();
    expectNoSideEffect();
  });
});

describe("the store spy", () => {
  it("would catch a write: the subscription sees the store change", () => {
    render(<Preview width={1250} />);
    act(() => {
      useTakeTestStore.getState().reset();
    });
    expect(storeWrites).toHaveBeenCalled();
  });
});
