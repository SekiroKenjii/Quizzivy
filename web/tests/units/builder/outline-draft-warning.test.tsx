import { beforeAll, beforeEach, expect, it } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import TestBuilderPage from "@/features/tests/pages/teacher/TestBuilderPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const QUESTION_ID = "018f0000-0000-7000-8000-0000000000b1";

const test = {
  skills: [],
  assignments: { live: 0, scheduled: 0, closed: 0 },
  unpublishedChanges: null,
  id: TEST_ID,
  title: "Unit 5",
  description: null,
  status: "draft" as const,
  currentVersion: 0,
  nextVersion: 1,
  totalPoints: 1,
  questionCount: 1,
  audioCount: 0,
  sections: [
    {
      id: "018f0000-0000-7000-8000-0000000000c1",
      ordinal: 0,
      title: "Ngữ pháp",
      instructions: null,
      questionIds: [QUESTION_ID],
    },
  ],
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-02T00:00:00Z",
};

const question = {
  level: null,
  skill: null,
  id: QUESTION_ID,
  type: "multiple_choice" as const,
  prompt: "Which are mammals?",
  media: null,
  audio: null,
  transcript: null,
  selectCount: 2,
  options: [
    {
      id: "018f0000-0000-7000-8000-0000000000d1",
      ordinal: 0,
      text: "cat",
      isCorrect: true,
    },
    {
      id: "018f0000-0000-7000-8000-0000000000d2",
      ordinal: 1,
      text: "dog",
      isCorrect: true,
    },
    {
      id: "018f0000-0000-7000-8000-0000000000d3",
      ordinal: 2,
      text: "bird",
      isCorrect: false,
    },
  ],
  blanks: [],
  points: 1,
  explanation: null,
  sampleAnswer: null,
  tags: [],
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

let creates = 0;

// The outline is a lazy chunk; loading it here keeps its first transform out
// of the tests' own time.
beforeAll(async () => {
  await import("@/features/tests/components/OutlineTree");
}, 30_000);

beforeEach(() => {
  creates = 0;
  server.use(
    http.patch(`${BASE}/teacher/questions/:id`, () =>
      contractJson("/teacher/questions/{id}", "patch", 200, question),
    ),
    http.post(`${BASE}/teacher/questions`, () => {
      creates += 1;
      return new Response(null, { status: 500 });
    }),
    http.get(`${BASE}/teacher/tests/:id`, () =>
      contractJson("/teacher/tests/{id}", "get", 200, test),
    ),
    http.get(`${BASE}/teacher/questions/:id`, () =>
      contractJson("/teacher/questions/{id}", "get", 200, question),
    ),
  );
});

// QC-R6B-2: the schema keeps an unkeyed choose N from saving, so the stored
// question stays keyed; the outline row reads the editor's draft instead.
function renderBuilder() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/teacher/tests/:id/edit", element: <TestBuilderPage /> }],
    { initialEntries: [`/teacher/tests/${TEST_ID}/edit`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

// QC-R6B-12: a question it could not open is not added behind a refused draft.
it("adds no question while the selected draft cannot be saved", async () => {
  const user = renderBuilder();
  // Wait for the outline before making the draft unkeyed.
  const add = await screen.findByRole(
    "button",
    { name: /^Thêm câu hỏi/ },
    { timeout: 4000 },
  );
  await user.click(
    await screen.findByRole("checkbox", { name: "Đánh dấu B là đáp án đúng" }),
  );
  await user.click(add);
  expect(
    await screen.findByText("Chưa lưu được câu hỏi. Thử lưu lại trước khi chuyển câu."),
  ).toBeInTheDocument();
  expect(creates).toBe(0);
}, 15_000);

it("warns on the outline row while the selected choose N draft is unkeyed", async () => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/teacher/tests/:id/edit", element: <TestBuilderPage /> }],
    { initialEntries: [`/teacher/tests/${TEST_ID}/edit`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  const user = userEvent.setup();
  const markB = await screen.findByRole("checkbox", {
    name: "Đánh dấu B là đáp án đúng",
  });
  expect(screen.queryAllByText("Chưa có đáp án đúng")).toHaveLength(0);

  await user.click(markB);
  const warning = await screen.findByText("Chưa có đáp án đúng", {
    selector: ".sr-only",
  });
  // The warning names the row it describes, as a stored unkeyed question does.
  expect(
    document.querySelector(`[aria-describedby="${warning.id}"]`),
  ).toHaveTextContent("Which are mammals?");

  await user.click(markB);
  await waitFor(() =>
    expect(screen.queryAllByText("Chưa có đáp án đúng")).toHaveLength(0),
  );

  // Too many ticks warn as well: three ticked where students choose two.
  const markC = screen.getByRole("checkbox", { name: "Đánh dấu C là đáp án đúng" });
  await user.click(markC);
  expect(
    await screen.findByText("Chưa có đáp án đúng", { selector: ".sr-only" }),
  ).toBeInTheDocument();
  await user.click(markC);
  await waitFor(() =>
    expect(screen.queryAllByText("Chưa có đáp án đúng")).toHaveLength(0),
  );
}, 15_000);
