import { render } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { vi } from "vitest";
import { DeckScale } from "@/components/ui/deck-scale";
import QuestionBankPage from "@/features/question-bank/pages/teacher/QuestionBankPage";

/** BANK_API is the stubbed API origin the bank page calls. */
export const BANK_API = "http://localhost:8080";

/** bankQuestion is a bank row as `listQuestions` returns it, with `overrides` applied. */
export function bankQuestion(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    level: null,
    skill: null,
    id: "018f0000-0000-7000-8000-0000000000b1",
    type: "single_choice" as const,
    prompt: "Người phụ nữ đề nghị làm gì?",
    media: null,
    audio: null,
    transcript: null,
    options: [],
    blanks: [],
    points: 2,
    explanation: null,
    sampleAnswer: null,
    tags: ["unit-5", "listening"],
    usedInTests: 3,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

/** bankFacets is the facet block of a `listQuestions` answer, every count zero unless given. */
export function bankFacets(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    levels: { pre_a1: 0, a1: 0, a2: 0, b1: 0, b2: 0, c1: 0, c2: 0 },
    skills: {
      grammar: 0,
      vocabulary: 0,
      reading: 0,
      listening: 0,
      writing: 0,
      speaking: 0,
    },
    all: 0,
    single_choice: 0,
    multiple_choice: 0,
    true_false: 0,
    fill_blank: 0,
    short_answer: 0,
    ...overrides,
  };
}

/** bankPage is a whole `listQuestions` answer: `items` on one page, with `overrides` applied. */
export function bankPage(
  items: readonly ReturnType<typeof bankQuestion>[],
  overrides: Partial<Record<string, unknown>> = {},
) {
  return {
    facets: bankFacets(),
    tags: [],
    bankTotal: items.length,
    items,
    page: 1,
    pageSize: 10,
    total: items.length,
    ...overrides,
  };
}

/**
 * renderBank renders the bank page on a deck surface at `path`, with the
 * editor's routes as stand-ins, and returns a user-event session that drives
 * fake timers when `fakeTimers` is set.
 */
export function renderBank(path = "/teacher/question-bank", fakeTimers = false) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      {
        path: "/teacher/question-bank",
        element: (
          <DeckScale>
            <QuestionBankPage />
          </DeckScale>
        ),
      },
      { path: "/teacher/question-bank/new", element: <p>new question page</p> },
      { path: "/teacher/question-bank/:id", element: <p>editor page</p> },
    ],
    { initialEntries: [path] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return {
    router,
    user: userEvent.setup(fakeTimers ? { advanceTimers: vi.advanceTimersByTime } : {}),
  };
}
