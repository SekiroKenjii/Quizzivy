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

const strip = () => screen.queryByRole("button", { name: "Câu 2" });
const countButton = () => screen.queryByRole("button", { name: /^Danh sách câu: / });

function mount() {
  const router = createMemoryRouter(
    [{ path: "/app/attempts/:attemptId", element: <TakeTestPage /> }],
    { initialEntries: ["/app/attempts/att-1"] },
  );
  return render(<RouterProvider router={router} />);
}

it("keeps the answer focused when the window crosses the wide breakpoint", async () => {
  const resize = resizableViewport(1280);
  const user = userEvent.setup();
  mount();
  await screen.findByText("Describe your weekend");
  const answer = screen.getByRole("textbox");
  await user.type(answer, "We went");
  expect(answer).toHaveFocus();

  resize(600);
  expect(countButton()).toBeInTheDocument();
  expect(strip()).toBeNull();
  expect(screen.getByRole("textbox")).toHaveFocus();

  resize(1280);
  expect(countButton()).toBeNull();
  expect(strip()).toBeInTheDocument();
  expect(screen.getByRole("textbox")).toHaveFocus();
  expect(screen.getByRole("textbox")).toHaveValue("We went");
});

it("is the wide engine from 768px and the phone engine below it", async () => {
  const resize = resizableViewport(768);
  mount();
  await screen.findByText("Describe your weekend");
  expect(strip()).toBeInTheDocument();
  expect(countButton()).toBeNull();
  expect(screen.queryByRole("complementary")).toBeNull();
  expect(screen.getByText("Câu 1 trên 2 · Trả lời ngắn")).toBeInTheDocument();
  expect(screen.getAllByText("1 điểm · giáo viên chấm tay")).toHaveLength(1);

  resize(767);
  expect(strip()).toBeNull();
  expect(countButton()).toBeInTheDocument();
  expect(screen.queryByRole("complementary")).toBeNull();
  expect(screen.getByText("Câu 1 trên 2 · Trả lời ngắn")).toBeInTheDocument();
  expect(screen.getAllByText("1 điểm · giáo viên chấm tay")).toHaveLength(1);

  resize(1023);
  expect(strip()).toBeInTheDocument();
  expect(countButton()).toBeNull();
});
