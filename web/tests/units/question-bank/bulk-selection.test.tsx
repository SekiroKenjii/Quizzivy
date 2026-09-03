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
const A = "018f0000-0000-7000-8000-0000000000a1";
const B = "018f0000-0000-7000-8000-0000000000a2";

function question(id: string, prompt: string) {
  return {
    id,
    type: "short_answer" as const,
    prompt,
    points: 1,
    tags: ["unit-5"],
    usedInTests: 0,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  };
}

let tagged: { questionIds: string[]; tags: string[] } | null = null;

beforeEach(() => {
  tagged = null;
  server.use(
    http.get(`${BASE}/admin/questions`, ({ request }) => {
      const q = new URL(request.url).searchParams.get("q") ?? "";
      const all = [question(A, "Câu một"), question(B, "Câu hai")];
      return contractJson("/admin/questions", "get", 200, {
        items: q === "" ? all : all.filter((x) => x.prompt.includes(q)),
        page: 1,
        pageSize: 50,
        total: 0,
        facets: {
          all: 2,
          single_choice: 0,
          multiple_choice: 0,
          true_false: 0,
          fill_blank: 0,
          short_answer: 2,
        },
        tags: [],
        bankTotal: 0,
      });
    }),
    http.post(`${BASE}/admin/questions/tags`, async ({ request }) => {
      tagged = (await request.json()) as { questionIds: string[]; tags: string[] };
      return HttpResponse.json({ updated: tagged.questionIds.length });
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

describe("A-06's bulk selection", () => {
  it("counts what is selected and offers the three actions", async () => {
    const user = renderBank();

    await user.click(await screen.findByLabelText("Chọn: Câu một"));
    expect(screen.getByText("Đã chọn 1 câu")).toBeInTheDocument();

    await user.click(screen.getByLabelText("Chọn: Câu hai"));
    expect(screen.getByText("Đã chọn 2 câu")).toBeInTheDocument();

    expect(screen.getByRole("button", { name: "Thêm vào đề thi" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Gắn thẻ" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Bỏ chọn" }));
    expect(screen.queryByText(/Đã chọn/)).toBeNull();
  });

  it("keeps a selection when the search filters the row away", async () => {
    const user = renderBank();

    await user.click(await screen.findByLabelText("Chọn: Câu một"));
    await user.type(screen.getByLabelText(/Tìm trong nội dung câu hỏi/), "hai");

    await waitFor(() =>
      expect(within(screen.getByRole("table")).queryByText("Câu một")).toBeNull(),
    );
    expect(screen.getByText("Đã chọn 1 câu")).toBeInTheDocument();
  });

  it("sends every selected id when tagging, not just the visible ones", async () => {
    const user = renderBank();

    await user.click(await screen.findByLabelText("Chọn: Câu một"));
    await user.click(screen.getByLabelText("Chọn: Câu hai"));
    await user.click(screen.getByRole("button", { name: "Gắn thẻ" }));

    await user.type(await screen.findByLabelText("Thẻ"), "unit-6{Enter}");
    await user.click(screen.getByRole("button", { name: "Gắn thẻ" }));

    await waitFor(() => expect(tagged).not.toBeNull());
    expect(tagged?.questionIds.sort()).toEqual([A, B].sort());
    expect(tagged?.tags).toEqual(["unit-6"]);
  });
});

/**
 * The commonest use of this dialog is one tag, and the commonest way to enter
 * one is to type it and press the button. Requiring Enter first would make that
 * a silent no-op.
 */
describe("the bulk tag dialog", () => {
  it("applies a tag left in the input without pressing Enter", async () => {
    const user = renderBank();

    await user.click(await screen.findByLabelText("Chọn: Câu một"));
    await user.click(screen.getByRole("button", { name: "Gắn thẻ" }));
    await user.type(await screen.findByLabelText("Thẻ"), "unit-9");
    await user.click(screen.getByRole("button", { name: "Gắn thẻ" }));

    await waitFor(() => expect(tagged).not.toBeNull());
    expect(tagged?.tags).toEqual(["unit-9"]);
  });
});

/** The rail's chips come from the SERVER, not from the returned page. */
describe("A-06's tag rail", () => {
  it("offers tags no row on this page carries", async () => {
    server.use(
      http.get(`${BASE}/admin/questions`, () =>
        contractJson("/admin/questions", "get", 200, {
          // Neither row mentions unit-9 or past-simple.
          items: [question(A, "Câu một"), question(B, "Câu hai")],
          page: 1,
          pageSize: 50,
          total: 2,
          facets: {
            all: 2,
            single_choice: 0,
            multiple_choice: 0,
            true_false: 0,
            fill_blank: 0,
            short_answer: 2,
          },
          tags: ["listening", "past-simple", "unit-5", "unit-9"],
          bankTotal: 72,
        }),
      ),
    );
    const user = renderBank();

    // All four offered, including the two nothing on this page carries.
    for (const tag of ["listening", "past-simple", "unit-5", "unit-9"]) {
      expect(
        await screen.findByRole("button", { name: tag, pressed: false }),
      ).toBeInTheDocument();
    }

    // And several can be held at once, which is what the page-derived rail made impossible.
    await user.click(screen.getByRole("button", { name: "unit-5", pressed: false }));
    await user.click(screen.getByRole("button", { name: "unit-9", pressed: false }));
    expect(screen.getAllByRole("button", { pressed: true })).toHaveLength(2);
  });

  it("shows the bank size and how much of it is showing", async () => {
    server.use(
      http.get(`${BASE}/admin/questions`, () =>
        contractJson("/admin/questions", "get", 200, {
          items: [question(A, "Câu một"), question(B, "Câu hai")],
          page: 1,
          pageSize: 50,
          total: 2,
          facets: {
            all: 2,
            single_choice: 0,
            multiple_choice: 0,
            true_false: 0,
            fill_blank: 0,
            short_answer: 2,
          },
          tags: [],
          bankTotal: 72,
        }),
      ),
    );
    renderBank();
    // A-06's "180 câu · đang lọc 41".
    expect(await screen.findByText("72 câu · đang lọc 2")).toBeInTheDocument();
  });
});
