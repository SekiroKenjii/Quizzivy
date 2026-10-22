import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import TakeTestPage from "@/features/take-test/pages/TakeTestPage";
import { getAttempt, saveAnswers } from "@/features/take-test/api";
import { useTakeTestStore } from "@/features/take-test/store";
import { session } from "./support";
import "@/lib/i18n";

vi.mock("@/features/take-test/api", () => ({
  getAttempt: vi.fn(),
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
}));

function resizableViewport(initial: number) {
  let width = initial;
  const listeners = new Set<() => void>();
  vi.stubGlobal("matchMedia", (query: string) => {
    const min = /min-width:\s*(\d+)px/.exec(query);
    const max = /max-width:\s*(\d+)px/.exec(query);
    return {
      matches: (!min || width >= Number(min[1])) && (!max || width <= Number(max[1])),
      media: query,
      onchange: null,
      addEventListener: (_type: string, listener: () => void) =>
        listeners.add(listener),
      removeEventListener: (_type: string, listener: () => void) =>
        listeners.delete(listener),
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: () => false,
    } as unknown as MediaQueryList;
  });
  return (next: number) => {
    width = next;
    act(() => listeners.forEach((listener) => listener()));
  };
}

beforeEach(() => {
  useTakeTestStore.getState().reset();
  const now = new Date().toISOString();
  const deadline = new Date(Date.now() + 3_600_000).toISOString();
  vi.mocked(saveAnswers).mockResolvedValue({
    serverTime: now,
    savedAt: now,
    deadlineAt: deadline,
  });
  vi.mocked(getAttempt).mockResolvedValue({
    ...session({
      serverTime: now,
      deadlineAt: deadline,
    }),
    questions: [
      {
        id: "q1",
        sectionId: "s1",
        type: "short_answer",
        prompt: "Describe your weekend",
        points: 1,
      },
      {
        id: "q2",
        sectionId: "s1",
        type: "short_answer",
        prompt: "Describe your school",
        points: 1,
      },
    ],
  });
});

afterEach(() => {
  useTakeTestStore.getState().reset();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

const rail = () => screen.queryByRole("complementary", { name: "Các phần" });
const gridButton = () => screen.queryByRole("button", { name: /^Câu 1 · đã làm / });

function mount() {
  const router = createMemoryRouter(
    [{ path: "/app/attempts/:attemptId", element: <TakeTestPage /> }],
    { initialEntries: ["/app/attempts/att-1"] },
  );
  return render(<RouterProvider router={router} />);
}

async function beside(passage: string) {
  const paper = await getAttempt("att-1");
  vi.mocked(getAttempt).mockResolvedValue({
    ...paper,
    groups: [
      {
        id: "g1",
        sectionId: "s1",
        title: "Bài đọc",
        questionIds: ["q1", "q2"],
        stimuli: [
          {
            id: "st1",
            title: "Bài đọc",
            content: {
              format: "semantic_v1",
              blocks: [
                {
                  type: "paragraph",
                  content: [{ type: "text", text: passage, marks: [] }],
                },
              ],
            },
            gaps: [],
          },
        ],
        recordings: [],
        assets: [],
      },
    ],
    groupAudioPlays: {},
  });
}

it("keeps the answer focused when the window crosses 768 and 900", async () => {
  const resize = resizableViewport(1280);
  const user = userEvent.setup();
  mount();
  await screen.findByText("Describe your weekend");
  const answer = screen.getByRole("textbox");
  await user.type(answer, "We went");
  expect(rail()).toBeInTheDocument();
  expect(answer).toHaveFocus();

  resize(899);
  expect(rail()).toBeNull();
  expect(gridButton()).toBeInTheDocument();
  expect(screen.getByRole("textbox")).toHaveFocus();

  resize(600);
  expect(rail()).toBeNull();
  expect(gridButton()).toBeInTheDocument();
  expect(screen.getByRole("textbox")).toHaveFocus();

  resize(900);
  expect(rail()).toBeInTheDocument();
  expect(gridButton()).toBeNull();
  expect(screen.getByRole("textbox")).toHaveFocus();
  await user.type(screen.getByRole("textbox"), " out");
  expect(screen.getByRole("textbox")).toHaveValue("We went out");
});

it("keeps the answer focused beside shared content when the window crosses 1180 and 768", async () => {
  await beside("Một đoạn văn.");
  const resize = resizableViewport(1280);
  const user = userEvent.setup();
  mount();
  await screen.findByText("Describe your weekend");
  const answer = screen.getByRole("textbox");
  await user.type(answer, "We went");
  expect(rail()).toBeInTheDocument();

  resize(1179);
  expect(rail()).toBeNull();
  expect(gridButton()).toBeInTheDocument();
  expect(screen.getByRole("textbox")).toHaveFocus();

  resize(767);
  expect(rail()).toBeNull();
  expect(screen.getByRole("textbox")).toHaveFocus();

  resize(1180);
  expect(rail()).toBeInTheDocument();
  expect(gridButton()).toBeNull();
  expect(screen.getByRole("textbox")).toHaveFocus();
  expect(screen.getByRole("textbox")).toHaveValue("We went");
});

it("is the wide engine from 768px and the phone engine below it, with the rail from 900", async () => {
  const resize = resizableViewport(768);
  mount();
  await screen.findByText("Describe your weekend");
  expect(rail()).toBeNull();
  expect(gridButton()).toHaveTextContent("Câu 1 · đã làm 0 trên 2");
  expect(screen.getByRole("button", { name: "Câu trước" })).toHaveTextContent("Trước");
  expect(screen.getByText("Câu 1 trên 2 · Trả lời ngắn")).toBeInTheDocument();
  expect(screen.getAllByText("1 điểm · giáo viên chấm tay")).toHaveLength(1);

  resize(767);
  expect(rail()).toBeNull();
  expect(gridButton()).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Câu trước" })).toHaveTextContent(/^$/);
  expect(screen.getByText("Câu 1 trên 2 · Trả lời ngắn")).toBeInTheDocument();
  expect(screen.getAllByText("1 điểm · giáo viên chấm tay")).toHaveLength(1);

  resize(899);
  expect(rail()).toBeNull();
  expect(gridButton()).toBeInTheDocument();

  resize(900);
  expect(rail()).toBeInTheDocument();
  expect(gridButton()).toBeNull();
});
