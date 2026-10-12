import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, MemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import GradingPage from "@/features/attempts/pages/teacher/GradingPage";
import { GradingQueueAside } from "@/features/attempts/pages/teacher/GradingQueueAside";
import { GradingAnswerCard } from "@/features/attempts/pages/teacher/GradingAnswerCard";
import type { GradingQueue, GradingQueueItem } from "@/features/attempts/api";
import { viewport } from "@tests/support/viewport";
import { useAuthStore } from "@/stores/auth";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";
import "@/features/attempts/gradingMessages";
import { ASSIGNMENT_ID, ATTEMPT_ID, BASE, ESSAY_ID, STUDENT_ID } from "./fixtures";

const first: GradingQueueItem = {
  attemptId: ATTEMPT_ID,
  questionId: ESSAY_ID,
  assignmentId: ASSIGNMENT_ID,
  assignmentTitle: "Unit 5",
  studentId: STUDENT_ID,
  studentName: "Nguyễn Đức Minh",
  questionNumber: 2,
  type: "short_answer",
  prompt: "Morning habits",
  answer: { type: "text", value: "First line" },
  points: 1,
  score: null,
  comment: null,
};

const second: GradingQueueItem = {
  ...first,
  attemptId: "018f0000-0000-7000-8000-0000000000a8",
  studentId: "018f0000-0000-7000-8000-0000000000e2",
  studentName: "Trần Thu Hà",
};

const queue: GradingQueue = {
  items: [first, second],
  groups: [],
  answersRemaining: 2,
  studentsWaiting: 2,
};

const scrolled = vi.fn();

beforeEach(() => {
  scrolled.mockClear();
  Element.prototype.scrollIntoView = function (this: Element) {
    scrolled(this);
  };
});

afterEach(() => {
  Reflect.deleteProperty(Element.prototype, "scrollIntoView");
});

function aside(selected: GradingQueueItem) {
  return (
    <GradingQueueAside
      queue={queue}
      items={[first, second]}
      selected={selected}
      mode="student"
      busy={false}
      onSelect={vi.fn()}
    />
  );
}

function card() {
  return render(
    <MemoryRouter>
      <GradingAnswerCard
        item={first}
        draft={undefined}
        busy={false}
        finishReady={false}
        position={1}
        total={2}
        onScore={vi.fn()}
        onComment={vi.fn()}
        onPrevious={vi.fn()}
        onNext={vi.fn()}
        onRetryMaterial={vi.fn()}
        onReview={vi.fn()}
      />
    </MemoryRouter>,
  );
}

it("from 768 makes the queue a column whose list scrolls inside it", () => {
  viewport("desktop");
  render(aside(first));
  const panel = screen.getByRole("complementary");
  expect(panel).toHaveClass("min-[768px]:flex", "min-[768px]:flex-col");
  const list = within(panel).getAllByRole("button")[0]!.parentElement!;
  expect(list).toHaveClass(
    "overflow-y-auto",
    "min-[768px]:min-h-0",
    "min-[768px]:flex-[1_1_auto]",
    "min-[768px]:max-h-140",
  );
});

it("below 768 keeps the queue's list at its 180px cap", () => {
  viewport("phone");
  render(aside(first));
  const list = within(screen.getByRole("complementary")).getAllByRole("button")[0]!
    .parentElement!;
  expect(list).toHaveClass("max-h-45", "overflow-y-auto");
});

it("keeps the active queue row in view when the selection moves", () => {
  const { rerender } = render(aside(first));
  expect(scrolled).toHaveBeenLastCalledWith(
    screen.getByRole("button", { current: true }),
  );
  rerender(aside(second));
  const active = screen.getByRole("button", { current: true });
  expect(active).toHaveAccessibleName(/Trần Thu Hà/);
  expect(scrolled).toHaveBeenLastCalledWith(active);
});

it("from 768 scrolls the card's body inside the card and keeps its footer in place", () => {
  viewport("desktop");
  card();
  const region = screen.getByRole("region");
  expect(region).toHaveClass("flex-col", "overflow-hidden", "min-[768px]:min-h-0");
  const [body, footer] = [...region.children];
  expect(body).toHaveClass(
    "min-[768px]:min-h-0",
    "min-[768px]:flex-[1_1_auto]",
    "min-[768px]:overflow-y-auto",
  );
  expect(footer).toHaveClass("mt-auto", "border-t");
});

it("from 768 holds the page's panes at the viewport's height beside each other", async () => {
  viewport("desktop");
  useAuthStore.getState().setSession("token", teacherUser);
  server.use(
    http.get(`${BASE}/teacher/grading/queue`, () =>
      contractJson("/teacher/grading/queue", "get", 200, queue),
    ),
    http.get(`${BASE}/teacher/attempts`, () =>
      contractJson("/teacher/attempts", "get", 200, {
        items: [],
        total: 0,
        page: 1,
        pageSize: 100,
      }),
    ),
  );
  const router = createMemoryRouter(
    [{ path: "/teacher/grading", element: <GradingPage /> }],
    { initialEntries: ["/teacher/grading"] },
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  const panes = (await screen.findByRole("complementary")).parentElement!;
  expect(panes).toHaveClass(
    "flex-wrap",
    "items-start",
    "min-[768px]:min-h-0",
    "min-[768px]:flex-[1_1_auto]",
    "min-[768px]:flex-nowrap",
    "min-[768px]:items-stretch",
  );
  const page = panes.parentElement!;
  expect(page).toHaveClass(
    "min-[768px]:h-[calc(100vh-120px)]",
    "min-[768px]:min-h-140",
  );
  expect(within(page).getByRole("heading", { level: 1 })).toBeInTheDocument();
  expect(
    within(page).queryByRole("region", {
      name: "Bài sẵn sàng để xem lại hoặc hoàn tất",
    }),
  ).toBeNull();
});
