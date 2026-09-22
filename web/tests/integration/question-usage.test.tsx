import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import { expect, it } from "vitest";
import QuestionBankPage from "@/features/question-bank/pages/QuestionBankPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";

it("loads attached tests on expansion and renders links in a nested table", async () => {
  const question = {
    id: "018f0000-0000-7000-8000-0000000000b1",
    type: "single_choice",
    prompt: "Linked question",
    points: 1,
    tags: [],
    usedInTests: 1,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  };
  const testId = "018f0000-0000-7000-8000-0000000000a1";
  let details = 0;
  server.use(
    http.get("http://localhost:8080/admin/questions", () =>
      contractJson("/admin/questions", "get", 200, {
        items: [question],
        page: 1,
        pageSize: 20,
        total: 1,
        bankTotal: 1,
        tags: [],
        facets: {
          all: 1,
          single_choice: 1,
          multiple_choice: 0,
          true_false: 0,
          fill_blank: 0,
          short_answer: 0,
        },
      }),
    ),
    http.get(`http://localhost:8080/admin/questions/${question.id}`, () => {
      details += 1;
      return contractJson("/admin/questions/{id}", "get", 200, {
        ...question,
        usedIn: [{ id: testId, title: "Grammar outline" }],
      });
    }),
  );
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
  const toggle = await screen.findByRole("button", { name: "1 đề" });
  expect(details).toBe(0);
  const user = userEvent.setup();
  await user.click(toggle);
  const table = await screen.findByRole("table", {
    name: "Đề đang gắn câu hỏi: Linked question",
  });
  expect(within(table).getByRole("link", { name: "Grammar outline" })).toHaveAttribute(
    "href",
    `/admin/tests/${testId}/edit`,
  );
  expect(details).toBe(1);
  expect(toggle).toHaveAttribute("aria-expanded", "true");
  await user.click(toggle);
  expect(
    screen.queryByRole("table", { name: "Đề đang gắn câu hỏi: Linked question" }),
  ).not.toBeInTheDocument();
  expect(toggle).toHaveAttribute("aria-expanded", "false");
});
