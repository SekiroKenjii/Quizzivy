import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
import QuestionBankPage from "@/features/question-bank/pages/QuestionBankPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const ID = "018f0000-0000-7000-8000-0000000000b1";

const question = {
  id: ID,
  type: "single_choice" as const,
  prompt: "The letter ___ yesterday by the manager.",
  media: null,
  audio: null,
  transcript: null,
  options: [],
  blanks: [],
  points: 1,
  explanation: null,
  sampleAnswer: null,
  tags: ["passive"],
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

let deletes = 0;
let referenced = false;

beforeEach(() => {
  deletes = 0;
  referenced = false;
  server.use(
    http.get(`${BASE}/admin/questions`, () =>
      contractJson("/admin/questions", "get", 200, {
        facets: {
          all: 1,
          single_choice: 1,
          multiple_choice: 0,
          true_false: 0,
          fill_blank: 0,
          short_answer: 0,
        },
        tags: ["passive"],
        bankTotal: 1,
        items: [question],
        page: 1,
        pageSize: 50,
        total: 1,
      }),
    ),
    http.delete(`${BASE}/admin/questions/${ID}`, () => {
      deletes += 1;
      if (referenced) {
        return HttpResponse.json(
          {
            error: {
              code: "QUESTION_REFERENCED",
              message: "Câu hỏi đang nằm trong đề nháp.",
              requestId: "req_1",
              details: {
                tests: [
                  { id: "018f0000-0000-7000-8000-0000000000d1", title: "Unit 5" },
                  { id: "018f0000-0000-7000-8000-0000000000d2", title: "Mid-term" },
                ],
              },
            },
          },
          { status: 409, headers: { "x-request-id": "req_1" } },
        );
      }
      return new HttpResponse(null, { status: 204 });
    }),
  );
});

function renderBank() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/admin/question-bank", element: <QuestionBankPage /> }],
    { initialEntries: ["/admin/question-bank"] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

async function openDelete(user: ReturnType<typeof userEvent.setup>) {
  await screen.findByText(question.prompt);
  await user.click(screen.getByRole("button", { name: "Thao tác" }));
  await user.click(await screen.findByRole("menuitem", { name: "Xoá" }));
  return screen.findByRole("dialog");
}

describe("deleting a question", () => {
  it("asks first, echoing the prompt, then deletes", async () => {
    const user = renderBank();
    const dialog = await openDelete(user);
    expect(within(dialog).getByText("Xoá câu hỏi")).toBeInTheDocument();
    expect(within(dialog).getByText(question.prompt)).toBeInTheDocument();
    expect(deletes).toBe(0);

    await user.click(within(dialog).getByRole("button", { name: "Xoá" }));
    await waitFor(() => expect(deletes).toBe(1));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("explains a draft-test reference instead of failing quietly", async () => {
    referenced = true;
    const user = renderBank();
    const dialog = await openDelete(user);
    await user.click(within(dialog).getByRole("button", { name: "Xoá" }));

    expect(
      await within(dialog).findByText("Chưa xoá được câu hỏi này"),
    ).toBeInTheDocument();
    expect(within(dialog).getByText("Đang dùng trong 2 đề nháp:")).toBeInTheDocument();
    expect(within(dialog).getByRole("link", { name: "Unit 5" })).toHaveAttribute(
      "href",
      "/admin/tests/018f0000-0000-7000-8000-0000000000d1/edit",
    );
    expect(within(dialog).getByRole("link", { name: "Mid-term" })).toBeInTheDocument();
    expect(within(dialog).queryByRole("button", { name: "Xoá" })).toBeNull();
    expect(within(dialog).queryByRole("button", { name: "Huỷ" })).toBeNull();

    await user.click(within(dialog).getAllByRole("button", { name: "Đóng" }).at(-1)!);
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });
});
