import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import { Toaster, toast } from "@/components/ui/sonner";
import TestBuilderPage from "@/features/tests/pages/teacher/TestBuilderPage";
import { PublishDialog } from "@/features/tests/components/PublishDialog";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const QUESTION_ID = "018f0000-0000-7000-8000-0000000000b1";
const PUBLISHED_AT = "2026-01-05T00:00:00.000000Z";

/**
 * Publishing in place (T-R4.31b): the builder stays open after a publish and
 * takes the version the publish answered with as the base of its next save.
 * The mock enforces §8's guard as the server does: a PATCH whose
 * expectedUpdatedAt is not the current version is a 409 STALE_WRITE, and a
 * publish moves the version.
 */
let currentVersion = "2026-01-02T00:00:00.000000Z";
let staleWrites = 0;
let accepted = 0;
let publishes: unknown[] = [];
let correct = true;
let refuse = false;

function testBody() {
  return {
    skills: [],
    assignments: { live: 0, scheduled: 0, closed: 0 },
    unpublishedChanges: null,
    id: TEST_ID,
    title: "Unit 5",
    description: null,
    status: publishes.length > 0 ? ("published" as const) : ("draft" as const),
    currentVersion: publishes.length,
    nextVersion: publishes.length + 1,
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
    updatedAt: currentVersion,
  };
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  currentVersion = "2026-01-02T00:00:00.000000Z";
  staleWrites = 0;
  accepted = 0;
  publishes = [];
  correct = true;
  refuse = false;
  server.use(
    http.get(`${BASE}/teacher/tests/:id`, () =>
      contractJson("/teacher/tests/{id}", "get", 200, testBody()),
    ),
    http.get(`${BASE}/teacher/questions/:id`, () =>
      contractJson("/teacher/questions/{id}", "get", 200, {
        level: null,
        skill: null,
        id: QUESTION_ID,
        type: "single_choice" as const,
        prompt: "Chọn từ đúng",
        media: null,
        audio: null,
        transcript: null,
        options: [
          {
            id: "018f0000-0000-7000-8000-0000000000d1",
            ordinal: 1,
            text: "went",
            isCorrect: correct,
          },
          {
            id: "018f0000-0000-7000-8000-0000000000d2",
            ordinal: 2,
            text: "go",
            isCorrect: false,
          },
        ],
        blanks: [],
        points: 1,
        explanation: "Quá khứ đơn.",
        sampleAnswer: null,
        tags: [],
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
      }),
    ),
    http.patch(`${BASE}/teacher/tests/:id`, async ({ request }) => {
      const body = (await request.json()) as { expectedUpdatedAt: string };
      if (body.expectedUpdatedAt !== currentVersion) {
        staleWrites += 1;
        return Response.json(
          { error: { code: "STALE_WRITE", message: "Đã có thay đổi ở nơi khác." } },
          { status: 409 },
        );
      }
      accepted += 1;
      currentVersion = `2026-01-06T00:00:0${accepted}.000000Z`;
      return contractJson("/teacher/tests/{id}", "patch", 200, testBody());
    }),
    http.post(`${BASE}/teacher/tests/:id/publish`, async ({ request }) => {
      if (refuse)
        return Response.json(
          {
            error: {
              code: "PUBLISH_VALIDATION_FAILED",
              message: "Chưa phát hành được.",
            },
            violations: [
              {
                rule: "choice_has_correct_option",
                message: "Chưa có đáp án đúng.",
                questionId: QUESTION_ID,
              },
            ],
          },
          { status: 409 },
        );
      publishes.push(await request.json());
      currentVersion = PUBLISHED_AT;
      return contractJson("/teacher/tests/{id}/publish", "post", 201, {
        id: "018f0000-0000-7000-8000-0000000000e1",
        version: 1,
        totalPoints: 1,
        questionCount: 1,
        audioCount: 0,
        manualCount: 0,
        skills: [],
        publishedAt: PUBLISHED_AT,
        publishedBy: "Lan",
        assignmentCount: 0,
        changeNote: "Bản đầu",
        testUpdatedAt: PUBLISHED_AT,
      });
    }),
  );
});

afterEach(() => {
  toast.dismiss();
  vi.useRealTimers();
});

async function renderBuilder() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/tests/:id/edit", element: <TestBuilderPage /> },
      { path: "/teacher/assignments/new", element: <p>Giao bài mới</p> },
    ],
    { initialEntries: [`/teacher/tests/${TEST_ID}/edit`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  await screen.findByRole("button", { name: "Tên đề thi" });
  return { user, router };
}

async function rename(user: ReturnType<typeof userEvent.setup>, text: string) {
  await user.click(screen.getByRole("button", { name: "Tên đề thi" }));
  const title = screen.getByRole<HTMLInputElement>("textbox", { name: "Tên đề thi" });
  title.setSelectionRange(title.value.length, title.value.length);
  await user.type(title, text);
  await vi.advanceTimersByTimeAsync(1500);
}

async function published() {
  expect(
    await screen.findByText(
      "Đã phát hành. Giờ bạn có thể giao bài.",
      {},
      { timeout: 3000 },
    ),
  ).toBeVisible();
  expect(screen.queryByRole("dialog")).toBeNull();
}

describe("publishing from the builder", () => {
  it("publishes with the change note, stays open and says so with Assign", async () => {
    const { user, router } = await renderBuilder();

    await user.click(screen.getByRole("button", { name: "Phát hành" }));
    const dialog = await screen.findByRole("dialog", { name: "Phát hành đề thi?" });
    expect(within(dialog).getByText("Mọi câu hỏi đều có đáp án đúng")).toBeVisible();
    expect(within(dialog).getByText("Mọi câu hỏi đều có điểm")).toBeVisible();
    expect(
      within(dialog).queryByText("Mọi nhóm đều có tiêu đề hoặc hướng dẫn"),
    ).toBeNull();
    await user.type(within(dialog).getByLabelText(/Ghi chú thay đổi/), "  Bản đầu  ");
    await user.click(within(dialog).getByRole("button", { name: "Phát hành" }));

    await waitFor(() => expect(publishes).toEqual([{ changeNote: "Bản đầu" }]));
    await published();
    expect(router.state.location.pathname).toBe(`/teacher/tests/${TEST_ID}/edit`);

    await user.click(screen.getByRole("button", { name: "Giao bài" }));
    expect(router.state.location.pathname).toBe("/teacher/assignments/new");
    expect(router.state.location.search).toBe(`?test=${TEST_ID}`);
  });

  it("saves an edit made after publishing without a STALE_WRITE", async () => {
    const { user } = await renderBuilder();

    await user.click(screen.getByRole("button", { name: "Phát hành" }));
    const dialog = await screen.findByRole("dialog", { name: "Phát hành đề thi?" });
    await user.click(within(dialog).getByRole("button", { name: "Phát hành" }));
    await waitFor(() => expect(publishes).toHaveLength(1));
    await published();

    await rename(user, " v2");

    await waitFor(() => expect(accepted).toBe(1));
    expect(staleWrites).toBe(0);
    expect(screen.queryByText(/mở ở nơi khác/)).toBeNull();
  });

  it("still shows the notice when another tab moved the version after publishing", async () => {
    const { user } = await renderBuilder();

    await user.click(screen.getByRole("button", { name: "Phát hành" }));
    const dialog = await screen.findByRole("dialog", { name: "Phát hành đề thi?" });
    await user.click(within(dialog).getByRole("button", { name: "Phát hành" }));
    await waitFor(() => expect(publishes).toHaveLength(1));
    await published();

    currentVersion = "2026-01-07T00:00:00.000000Z";
    await rename(user, " v2");

    expect(await screen.findByText(/mở ở nơi khác/)).toBeInTheDocument();
    expect(staleWrites).toBe(1);
  });

  it("names a question publishing would refuse, refuses to send, and Fix it goes there", async () => {
    correct = false;
    const { user } = await renderBuilder();

    await user.click(screen.getByRole("button", { name: "Phát hành" }));
    const dialog = await screen.findByRole("dialog", { name: "Phát hành đề thi?" });
    expect(within(dialog).queryByText("Mọi câu hỏi đều có đáp án đúng")).toBeNull();
    await user.click(within(dialog).getByRole("button", { name: "Phát hành" }));

    expect(
      await within(dialog).findByText("Hãy sửa các mục trên trước khi phát hành."),
    ).toBeVisible();
    expect(publishes).toHaveLength(0);

    await user.click(within(dialog).getByRole("button", { name: "Sửa" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.getByRole("heading", { name: "Câu 1" })).toBeVisible();
  });

  it("shows the server's violations when it refuses", async () => {
    refuse = true;
    const { user } = await renderBuilder();

    await user.click(screen.getByRole("button", { name: "Phát hành" }));
    const dialog = await screen.findByRole("dialog", { name: "Phát hành đề thi?" });
    await user.click(within(dialog).getByRole("button", { name: "Phát hành" }));

    expect(await within(dialog).findByText(/Chưa có đáp án đúng\./)).toBeVisible();
    expect(within(dialog).getByRole("button", { name: "Sửa" })).toBeVisible();
    expect(publishes).toHaveLength(0);
  });
});

describe("Publish test? with shared-context groups", () => {
  it("lists that every group has a title once the test holds a group", () => {
    render(
      <PublishDialog
        open
        pending={false}
        error={null}
        problems={[]}
        violations={null}
        groupCount={2}
        onOpenChange={vi.fn()}
        onGoTo={vi.fn()}
        onPublish={vi.fn()}
      />,
    );
    const dialog = screen.getByRole("dialog", { name: "Phát hành đề thi?" });
    expect(within(dialog).getByText("Mọi câu hỏi đều có đáp án đúng")).toBeVisible();
    expect(
      within(dialog).getByText("Mọi nhóm đều có tiêu đề hoặc hướng dẫn"),
    ).toBeVisible();
  });
});
