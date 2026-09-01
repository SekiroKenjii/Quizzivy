import { beforeEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import TestDetailPage from "@/features/tests/pages/TestDetailPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";

/** The DRAFT holds the teacher's latest edit. */
let draft = {
  id: TEST_ID,
  title: "Unit 5",
  description: null,
  status: "published" as const,
  currentVersion: 1,
  totalPoints: 2,
  questionCount: 1,
  audioCount: 0,
  sections: [
    {
      id: "018f0000-0000-7000-8000-0000000000c1",
      ordinal: 0,
      title: "Ngữ pháp",
      instructions: null,
      questionIds: ["018f0000-0000-7000-8000-0000000000b1"],
    },
  ],
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-02T00:00:00Z",
};

/** The published VERSION holds what students actually receive. */
const publishedPrompt = "They ___ to the museum last weekend.";

let previewCalls = 0;

beforeEach(() => {
  previewCalls = 0;
  draft = {
    id: TEST_ID,
    title: "Unit 5",
    description: null,
    status: "published" as const,
    currentVersion: 1,
    totalPoints: 2,
    questionCount: 1,
    audioCount: 0,
    sections: [
      {
        id: "018f0000-0000-7000-8000-0000000000c1",
        ordinal: 0,
        title: "Ngữ pháp",
        instructions: null,
        questionIds: ["018f0000-0000-7000-8000-0000000000b1"],
      },
    ],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-02T00:00:00Z",
  };
  server.use(
    http.get(`${BASE}/admin/tests/:id/versions`, () =>
      contractJson("/admin/tests/{id}/versions", "get", 200, {
        items: [
          {
            id: "018f0000-0000-7000-8000-0000000000f1",
            version: 1,
            totalPoints: 2,
            questionCount: 1,
            audioCount: 0,
            manualCount: 0,
            publishedAt: "2026-01-02T03:00:00Z",
            publishedBy: "Cô Thương",
          },
        ],
      }),
    ),
    http.get(`${BASE}/admin/tests/:id/preview`, () => {
      previewCalls += 1;
      return contractJson("/admin/tests/{id}/preview", "get", 200, {
        version: 1,
        questions: [
          {
            id: "018f0000-0000-7000-8000-0000000000e1",
            type: "single_choice",
            prompt: publishedPrompt,
            media: null,
            audio: null,
            options: [
              { id: "018f0000-0000-7000-8000-0000000000d1", text: "went" },
              { id: "018f0000-0000-7000-8000-0000000000d2", text: "have gone" },
            ],
            blanks: [],
            points: 2,
          },
        ],
      });
    }),
    http.get(`${BASE}/admin/tests/:id`, () =>
      contractJson("/admin/tests/{id}", "get", 200, draft),
    ),
  );
});

function renderDetail() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/admin/tests/:id", element: <TestDetailPage /> },
      { path: "/admin/tests/:id/edit", element: <p>builder</p> },
    ],
    { initialEntries: [`/admin/tests/${TEST_ID}`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

describe("the test detail preview", () => {
  it("renders the published version, not the draft", async () => {
    renderDetail();

    expect(await screen.findByText(publishedPrompt)).toBeInTheDocument();
    expect(screen.getByText("Bản đang phát hành · v1")).toBeInTheDocument();
  });

  it("does not change after the draft is edited", async () => {
    renderDetail();
    await screen.findByText(publishedPrompt);
    expect(screen.getByRole("heading", { name: "Unit 5" })).toBeInTheDocument();

    // The teacher edits the draft in the builder: a new title, a reordered
    // outline, a question swapped out. None of it has been published.
    draft = {
      ...draft,
      title: "Unit 5 (đang sửa)",
      questionCount: 9,
      totalPoints: 99,
      sections: [
        {
          ...draft.sections[0]!,
          questionIds: ["018f0000-0000-7000-8000-0000000000b9"],
        },
      ],
    };
    cleanup();
    renderDetail();

    // The header follows the draft, because that is the thing being edited.
    expect(
      await screen.findByRole("heading", { name: "Unit 5 (đang sửa)" }),
    ).toBeInTheDocument();

    // The preview does not: §7's version holds its own snapshot, so what a
    // student receives is whatever was published.
    expect(screen.getByText(publishedPrompt)).toBeInTheDocument();
    expect(screen.getByText("Bản đang phát hành · v1")).toBeInTheDocument();

    // And it came from /preview, never from the draft's outline.
    expect(previewCalls).toBe(2);
  });

  it("carries no grading key into the preview", async () => {
    renderDetail();
    await screen.findByText(publishedPrompt);

    // StudentQuestion has no isCorrect, so nothing here can leak the answer.
    const body = document.body.textContent ?? "";
    expect(body).not.toContain("isCorrect");
    expect(screen.getByText("went")).toBeInTheDocument();
    expect(screen.queryByText(/đáp án đúng/i)).toBeNull();
  });

  it("lists the version history newest first, with who published it", async () => {
    renderDetail();

    expect(await screen.findByText("v1")).toBeInTheDocument();
    expect(screen.getByText(/Cô Thương/)).toBeInTheDocument();
  });

  it("offers the builder when there is nothing published to preview", async () => {
    server.use(
      http.get(`${BASE}/admin/tests/:id/preview`, () =>
        Response.json(
          { error: { code: "TEST_NOT_PUBLISHED", message: "Chưa phát hành." } },
          { status: 409 },
        ),
      ),
      http.get(`${BASE}/admin/tests/:id/versions`, () =>
        contractJson("/admin/tests/{id}/versions", "get", 200, { items: [] }),
      ),
    );
    renderDetail();

    expect(
      await screen.findByText("Đề này chưa phát hành nên chưa có bản để xem."),
    ).toBeInTheDocument();
    expect(screen.getByText("Đề này chưa được phát hành lần nào.")).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "Mở trình soạn đề" })).toHaveLength(2);
  });

  it("is keyboard reachable from the header to the builder", async () => {
    const user = userEvent.setup();
    renderDetail();
    await screen.findByText(publishedPrompt);

    await user.tab();
    await user.tab();

    expect(document.activeElement).toHaveTextContent("Mở trình soạn đề");
  });
});
