import { useState } from "react";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import { expect, it } from "vitest";
import { Table, TableBody, TableCell, TableRow } from "@/components/ui/table";
import { QuestionUsageRow } from "@/features/question-bank/components/QuestionUsageRow";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";

const question = {
  level: null,
  skill: null,
  id: "018f0000-0000-7000-8000-0000000000b1",
  type: "single_choice",
  prompt: "Linked question",
  points: 1,
  tags: [],
  usedInTests: 1,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

function ExpandableRow() {
  const [open, setOpen] = useState(false);
  return (
    <Table>
      <TableBody>
        <TableRow>
          <TableCell>
            <button
              type="button"
              aria-expanded={open}
              aria-controls={`question-usage-${question.id}`}
              onClick={() => setOpen((value) => !value)}
            >
              1 đề
            </button>
          </TableCell>
        </TableRow>
        {open ? (
          <QuestionUsageRow questionId={question.id} prompt={question.prompt} />
        ) : null}
      </TableBody>
    </Table>
  );
}

it("loads attached tests on expansion and renders links in a nested table", async () => {
  const testId = "018f0000-0000-7000-8000-0000000000a1";
  let details = 0;
  server.use(
    http.get(`http://localhost:8080/teacher/questions/${question.id}`, () => {
      details += 1;
      return contractJson("/teacher/questions/{id}", "get", 200, {
        ...question,
        usedIn: [{ id: testId, title: "Grammar outline" }],
      });
    }),
  );
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/teacher/question-bank", element: <ExpandableRow /> }],
    { initialEntries: ["/teacher/question-bank"] },
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
    `/teacher/tests/${testId}/edit`,
  );
  expect(details).toBe(1);
  expect(toggle).toHaveAttribute("aria-expanded", "true");
  await user.click(toggle);
  expect(
    screen.queryByRole("table", { name: "Đề đang gắn câu hỏi: Linked question" }),
  ).not.toBeInTheDocument();
  expect(toggle).toHaveAttribute("aria-expanded", "false");
});
