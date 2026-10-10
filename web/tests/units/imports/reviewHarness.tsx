import { vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import ImportReviewPage from "@/features/imports/pages/teacher/ImportReviewPage";
import type {
  ImportFinding,
  ImportReview,
  SaveImportReview,
} from "@/features/imports/api";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import {
  capabilities,
  BASE,
  EXAM_SOURCE_ID,
  IMPORT_ID,
  KEY_SOURCE_ID,
  finding,
  option,
  question,
  review,
  section,
  sourceView,
  summary,
  wordImport,
} from "./fixtures";

export const KEY_EVIDENCE = {
  sourceId: KEY_SOURCE_ID,
  blockId: "key-1",
  start: 3,
  end: 4,
};
export const EXAM_EVIDENCE = {
  sourceId: EXAM_SOURCE_ID,
  blockId: "block-q1",
  start: 0,
  end: 5,
};

export function baseline(over: Partial<ImportReview> = {}): ImportReview {
  const q1 = question({
    id: "q1",
    label: "1",
    answer: {
      state: "conflict",
      optionIds: [],
      evidence: [EXAM_EVIDENCE, KEY_EVIDENCE],
      candidates: [
        { value: "A", evidence: [EXAM_EVIDENCE] },
        { value: "B", evidence: [KEY_EVIDENCE] },
      ],
    },
  });
  const q2 = question({
    id: "q2",
    label: "2",
    options: [
      option("q2-a", "A", "red"),
      option("q2-b", "B", "blue"),
      option("q2-c", "C", "green"),
    ],
    answer: { state: "unknown", optionIds: [], evidence: [] },
  });
  const findings: ImportFinding[] = [
    finding({
      id: "f-conflict",
      code: "CONFLICTING_ANSWER_KEYS",
      severity: "blocking",
      target: "q1",
      field: "answer",
      evidence: [EXAM_EVIDENCE, KEY_EVIDENCE],
    }),
    finding({
      id: "f-missing",
      code: "MISSING_ANSWER",
      severity: "blocking",
      target: "q2",
      field: "answer",
    }),
    finding({
      id: "f-irregular",
      code: "IRREGULAR_OPTION_COUNT",
      severity: "review_required",
      target: "q2",
      field: "options",
    }),
    finding({
      id: "f-scoring",
      code: "SCORING_DEFAULTED",
      severity: "informational",
      field: "points",
      count: 2,
    }),
  ];
  return review([section([q1, q2])], findings, {
    summary: summary({ blocking: 2, needsDecision: 1, answersConflicting: 1 }),
    ...over,
  });
}

export interface ReviewServer {
  puts: SaveImportReview[];
  commits: { requestId: string; draftRevision: number }[];
}

export function serveReview(initial: ImportReview, state: ReviewServer) {
  server.use(
    capabilities(),
    http.get(`${BASE}/teacher/imports/:id`, () =>
      contractJson("/teacher/imports/{id}", "get", 200, wordImport()),
    ),
    http.get(`${BASE}/teacher/imports/:id/review`, () =>
      contractJson("/teacher/imports/{id}/review", "get", 200, initial),
    ),
    http.get(`${BASE}/teacher/imports/:id/source`, ({ request }) => {
      const role =
        new URL(request.url).searchParams.get("role") === "answer_key"
          ? "answer_key"
          : "exam";
      const blocks =
        role === "exam"
          ? [
              {
                id: "block-q1",
                text: "1. She ___ to school yesterday.",
                spans: [{ start: 3, end: 6, marks: ["underline" as const] }],
              },
              { id: "block-q2", text: "2. Which colour is the sky?", spans: [] },
            ]
          : [{ id: "key-1", text: "1. B", spans: [] }];
      return contractJson(
        "/teacher/imports/{id}/source",
        "get",
        200,
        sourceView(role, blocks),
      );
    }),
    http.put(`${BASE}/teacher/imports/:id/review`, async ({ request }) => {
      const body = (await request.json()) as SaveImportReview;
      state.puts.push(body);
      return contractJson(
        "/teacher/imports/{id}/review",
        "put",
        200,
        savedFrom(initial, body),
      );
    }),
  );
}

export function savedFrom(initial: ImportReview, body: SaveImportReview): ImportReview {
  return {
    ...initial,
    revision: body.expectedRevision + 1,
    draft: {
      ...initial.draft,
      title: body.title,
      sections: body.sections,
      acknowledged: body.acknowledged,
    },
    findings: initial.findings.map((item) =>
      item.severity === "review_required"
        ? { ...item, acknowledged: body.acknowledged.includes(item.id) }
        : item,
    ),
  };
}

export async function renderReview() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/imports/:id/review", element: <ImportReviewPage /> },
      { path: "/teacher/imports/:id", element: <p>import detail</p> },
      { path: "/teacher/imports", element: <p>history</p> },
      { path: "/teacher/tests/:id/edit", element: <p>builder</p> },
    ],
    { initialEntries: [`/teacher/imports/${IMPORT_ID}/review`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  await screen.findByLabelText("Tên đề");
  return { user: userEvent.setup({ advanceTimers: vi.advanceTimersByTime }), router };
}

/** card is the review card of a question, by question id, or the whole-test card. */
export function card(id: string): HTMLElement {
  const found = document.querySelector<HTMLElement>(`[data-card="${id}"]`);
  if (found === null) throw new Error(`no card ${id}`);
  return found;
}

/** cardHeader is the button that opens and closes a card. */
export function cardHeader(id: string): HTMLElement {
  const found = card(id).querySelector<HTMLElement>("button[aria-expanded]");
  if (found === null) throw new Error(`no header for card ${id}`);
  return found;
}

/** isOpen reports whether a card is the open one. */
export function isOpen(id: string): boolean {
  return (
    document.querySelector(`[data-card="${id}"] > button[aria-expanded="true"]`) !==
    null
  );
}
