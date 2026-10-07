import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import QuestionBankPage from "@/features/question-bank/pages/teacher/QuestionBankPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const ID = "018f0000-0000-7000-8000-0000000000b1";
const COPY = "018f0000-0000-7000-8000-0000000000b2";

const question = (id: string) => ({
  level: null,
  skill: null,
  id,
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
});

let duplicates = 0;

beforeEach(() => {
  duplicates = 0;
  server.use(
    http.get(`${BASE}/teacher/questions`, () =>
      contractJson("/teacher/questions", "get", 200, {
        facets: {
          levels: {
            pre_a1: 0,
            a1: 0,
            a2: 0,
            b1: 0,
            b2: 0,
            c1: 0,
            c2: 0,
          },
          skills: {
            grammar: 0,
            vocabulary: 0,
            reading: 0,
            listening: 0,
            writing: 0,
            speaking: 0,
          },
          all: 1,
          single_choice: 1,
          multiple_choice: 0,
          true_false: 0,
          fill_blank: 0,
          short_answer: 0,
        },
        tags: ["passive"],
        bankTotal: 1,
        items: duplicates === 0 ? [question(ID)] : [question(COPY), question(ID)],
        page: 1,
        pageSize: 50,
        total: duplicates === 0 ? 1 : 2,
      }),
    ),
    http.post(`${BASE}/teacher/questions/${ID}/duplicate`, () => {
      duplicates += 1;
      return contractJson(
        "/teacher/questions/{id}/duplicate",
        "post",
        201,
        question(COPY),
      );
    }),
  );
});

function renderBank() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/question-bank", element: <QuestionBankPage /> },
      { path: "/teacher/question-bank/:id", element: <p>editor</p> },
    ],
    { initialEntries: ["/teacher/question-bank"] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

describe("a bank row (A-06a)", () => {
  it("opens from its prompt as a link and carries the four-item menu", async () => {
    const user = renderBank();

    expect(
      await screen.findByRole("link", {
        name: "The letter ___ yesterday by the manager.",
      }),
    ).toHaveAttribute("href", `/teacher/question-bank/${ID}`);

    await user.click(screen.getByRole("button", { name: "Thao tác" }));
    const menu = await screen.findByRole("menu");
    expect(
      within(menu)
        .getAllByRole("menuitem")
        .map((item) => item.textContent?.trim()),
    ).toEqual(["Mở", "Thêm vào đề thi", "Nhân bản", "Xoá"]);
  });

  it("duplicates into a marked row while keeping the bank open", async () => {
    const user = renderBank();
    await screen.findByRole("link", {
      name: "The letter ___ yesterday by the manager.",
    });

    await user.click(screen.getByRole("button", { name: "Thao tác" }));
    await user.click(await screen.findByRole("menuitem", { name: "Nhân bản" }));

    await waitFor(() => expect(duplicates).toBe(1));
    const links = await screen.findAllByRole("link", {
      name: "The letter ___ yesterday by the manager.",
    });
    expect(links).toHaveLength(2);
    const copy = links.find(
      (link) => link.getAttribute("href") === `/teacher/question-bank/${COPY}`,
    );
    expect(copy).toBeInTheDocument();
    expect(within(copy!.closest("tr")!).getByText("Vừa nhân bản")).toBeInTheDocument();
    expect(screen.queryByText("editor")).not.toBeInTheDocument();
  });
});
