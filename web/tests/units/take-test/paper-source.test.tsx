import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useState } from "react";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Paper } from "@/features/take-test/pages/TakeTestPage";
import { PaperSourceContext, type PaperSource } from "@/features/take-test/paper";
import { EngineWidthContext } from "@/features/take-test/width";
import type {
  Answer,
  StudentGroup,
  StudentQuestion,
  StudentSection,
} from "@/features/take-test/api";
import { viewport } from "./support";
import "@/lib/i18n";

const NOW = Date.parse("2026-10-12T08:00:00.000Z");
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
    maxPlays: 2,
    allowSeek: false,
    showTranscriptAfterSubmit: false,
    allowPause: true,
  },
};

const grouped: StudentQuestion = {
  id: ID("q002"),
  sectionId: SECTION.id,
  type: "single_choice",
  prompt: "Where does the walk start?",
  points: 1,
  options: [{ id: ID("o021"), text: "At the bridge" }],
};

const RECORDING = ID("r001");
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
      id: RECORDING,
      assetId: ASSET,
      policy: {
        maxPlays: 2,
        allowSeek: false,
        showTranscriptAfterSubmit: false,
        allowPause: true,
      },
    },
  ],
  assets: [audio(ASSET, "https://assets.example/walk.mp3")],
};

const calls = {
  setAnswer: vi.fn<PaperSource["setAnswer"]>(),
  toggleFlag: vi.fn<PaperSource["toggleFlag"]>(),
  onAudioPlay: vi.fn<PaperSource["onAudioPlay"]>(),
  onAudioEnded: vi.fn<PaperSource["onAudioEnded"]>(),
  onAudioBlocked: vi.fn<PaperSource["onAudioBlocked"]>(),
  onAudioSeekBlocked: vi.fn<PaperSource["onAudioSeekBlocked"]>(),
  noteRecordingPlay: vi.fn<PaperSource["noteRecordingPlay"]>(),
  flushRecordingPlays: vi.fn<PaperSource["flushRecordingPlays"]>(),
  onRecordingEnded: vi.fn<PaperSource["onRecordingEnded"]>(),
  onRecordingBlocked: vi.fn<PaperSource["onRecordingBlocked"]>(),
};

function Host({ deadlineAt = NOW + 10 * 60_000 }: Readonly<{ deadlineAt?: number }>) {
  const [index, setIndex] = useState(0);
  const [answers, setAnswers] = useState<Record<string, Answer>>({});
  const [flags, setFlags] = useState<ReadonlySet<string>>(new Set());
  const source: PaperSource = {
    ...calls,
    title: "Unit 4 · Listening",
    questions: [listening, grouped],
    sections: [SECTION],
    groups: [group],
    answers,
    setAnswer: (questionId, answer) => {
      calls.setAnswer(questionId, answer);
      setAnswers((current) => ({ ...current, [questionId]: answer }));
    },
    flags,
    toggleFlag: (questionId) => {
      calls.toggleFlag(questionId);
      setFlags((current) => new Set([...current, questionId]));
    },
    lock: null,
    submitState: "idle",
    saveStatus: "saved",
    deadlineAt,
    offsetMs: 0,
    timeLeft: () => Math.max(0, deadlineAt - Date.now()),
    questionPlays: {},
    recordingPlays: {},
    pendingRecordings: new Set(),
  };
  return (
    <PaperSourceContext value={source}>
      <Paper
        index={index}
        jumps={0}
        status={null}
        fullscreenBar={false}
        onLeave={vi.fn()}
        onMove={setIndex}
        onJump={setIndex}
        onSubmit={vi.fn()}
        onReload={vi.fn()}
      />
    </PaperSourceContext>
  );
}

let play: ReturnType<typeof vi.fn<() => Promise<void>>>;
let paused = true;

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date", "setInterval", "clearInterval"], now: NOW });
  viewport("desktop");
  paused = true;
  play = vi.fn(() => {
    paused = false;
    return Promise.resolve();
  });
  vi.spyOn(HTMLMediaElement.prototype, "play").mockImplementation(play);
  vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => {
    paused = true;
  });
  vi.spyOn(HTMLMediaElement.prototype, "paused", "get").mockImplementation(
    () => paused,
  );
  for (const call of Object.values(calls)) call.mockReset();
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

const element = () => document.querySelector("audio") as HTMLAudioElement;
const pressPlay = () => fireEvent.click(screen.getByRole("button", { name: "Phát" }));

describe("a paper drawn from a source with no store", () => {
  it("hands an answer and a flag to the source", async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<Host />);

    await user.click(screen.getByRole("radio", { name: /At the station/ }));
    expect(calls.setAnswer).toHaveBeenCalledWith(listening.id, {
      type: "choice",
      optionIds: [ID("o012")],
    });
    expect(screen.getByRole("radio", { name: /At the station/ })).toBeChecked();

    await user.click(screen.getByRole("button", { name: "Đánh dấu xem lại" }));
    expect(calls.toggleFlag).toHaveBeenCalledWith(listening.id);
    expect(screen.getByRole("button", { name: "Đã đánh dấu" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  it("reports a question's play, its end, a blocked play and a jump put back", async () => {
    render(<Host />);

    pressPlay();
    expect(calls.onAudioPlay).toHaveBeenCalledWith(listening.id);
    paused = true;
    fireEvent.ended(element());
    expect(calls.onAudioEnded).toHaveBeenCalledWith(listening.id);

    element().currentTime = 42;
    fireEvent.seeking(element());
    expect(calls.onAudioSeekBlocked).toHaveBeenCalledWith(listening.id);

    play.mockImplementation(() =>
      Promise.reject(new DOMException("blocked", "NotAllowedError")),
    );
    pressPlay();
    await act(async () => {
      await Promise.resolve();
    });
    expect(calls.onAudioBlocked).toHaveBeenCalledWith(listening.id);
    expect(calls.onAudioPlay).toHaveBeenCalledTimes(2);
  });

  it("reports a shared recording's play and its end", async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<Host />);
    await user.click(screen.getByRole("button", { name: /Câu 2/ }));

    const recording = screen.getByRole("group", { name: "Bài nghe 1" });
    fireEvent.click(within(recording).getByRole("button", { name: "Phát" }));
    expect(calls.noteRecordingPlay).toHaveBeenCalledWith(RECORDING);
    vi.setSystemTime(NOW + 4000);
    fireEvent.ended(element());
    expect(calls.onRecordingEnded).toHaveBeenCalledWith(RECORDING, 4000);
    expect(calls.onAudioPlay).not.toHaveBeenCalled();
  });

  it("shows the time the source gives, and follows a deadline it moves", () => {
    const { rerender } = render(<Host />);
    const timer = screen.getByRole("timer");
    expect(timer).toHaveTextContent("10:00");

    rerender(<Host deadlineAt={NOW + 15 * 60_000} />);
    expect(timer).toHaveTextContent("15:00");

    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(timer).toHaveTextContent("14:59");
  });

  it("takes its width from a measured frame over the window's", () => {
    const { unmount } = render(<Host />);
    expect(screen.getByText("Unit 4 · Listening")).toBeInTheDocument();
    unmount();

    render(
      <EngineWidthContext value={360}>
        <Host />
      </EngineWidthContext>,
    );
    expect(screen.queryByText("Unit 4 · Listening")).toBeNull();
  });
});
