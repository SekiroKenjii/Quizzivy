import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
import { Toaster } from "@/components/ui/sonner";
import QuestionEditorPage from "@/features/question-bank/pages/teacher/QuestionEditorPage";
import TestBuilderPage from "@/features/tests/pages/teacher/TestBuilderPage";
import { api } from "@/lib/api/client";
import vi_ from "@/lib/i18n/locales/vi.json";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import { aiError, aiOn, BASE, month } from "../ai/support";
import "@/lib/i18n";

vi.mock("@/lib/api/client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api/client")>();
  return { ...actual, api: vi.fn(actual.api) };
});

const ai = vi_.ai;
const QUESTION_ID = "018f0000-0000-7000-8000-0000000000b1";
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const SECTION_ID = "018f0000-0000-7000-8000-0000000000c1";
const EXPLANATION = "“Went” is the past simple, which a finished time needs.";
const SHORT = "A finished time takes the past simple.";
const ENGLISH = "Yesterday is a finished time, so the past simple is right.";
const SRC = resolve(import.meta.dirname, "../../../src/features/question-bank");

function option(index: number, text: string, isCorrect = false) {
  return {
    id: `018f0000-0000-7000-8000-0000000000d${index}`,
    ordinal: index,
    text,
    isCorrect,
  };
}

function question(overrides: Record<string, unknown> = {}) {
  return {
    id: QUESTION_ID,
    type: "single_choice" as const,
    level: null,
    skill: null,
    prompt: "They ___ to the museum yesterday.",
    media: null,
    audio: null,
    transcript: null,
    options: [option(1, "went", true), option(2, "have gone")],
    blanks: [],
    points: 1,
    explanation: null,
    sampleAnswer: null,
    tags: [],
    usedInTests: 0,
    usedIn: [],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

let stored = question();
let events: string[] = [];
let explanationBodies: unknown[] = [];
let patches: Record<string, unknown>[] = [];

beforeEach(() => {
  stored = question();
  events = [];
  explanationBodies = [];
  patches = [];
  vi.mocked(api).mockClear();
  aiOn();
  server.use(
    http.get(`${BASE}/teacher/questions/:id`, () =>
      contractJson("/teacher/questions/{id}", "get", 200, stored),
    ),
    http.patch(`${BASE}/teacher/questions/:id`, async ({ request }) => {
      const body = (await request.json()) as Record<string, unknown>;
      patches.push(body);
      events.push("patch question");
      return contractJson("/teacher/questions/{id}", "patch", 200, stored);
    }),
    http.post(`${BASE}/teacher/ai/questions/:id/explanation`, async ({ request }) => {
      const body = (await request.json()) as { length?: string; language: string };
      explanationBodies.push(body);
      events.push("explain");
      let text = body.length === "short" ? SHORT : EXPLANATION;
      if (body.language === "en") text = ENGLISH;
      return contractJson("/teacher/ai/questions/{id}/explanation", "post", 200, {
        text,
        usage: { credits: 300, left: 18500 },
      });
    }),
    http.post(`${BASE}/teacher/ai/questions/:id/distractors`, () => {
      events.push("distract");
      return contractJson("/teacher/ai/questions/{id}/distractors", "post", 200, {
        items: [
          { text: "has gone", why: "Mixes up the present perfect." },
          { text: "goes", why: "Ignores the time word." },
        ],
        usage: { credits: 250, left: 18550 },
      });
    }),
  );
});

afterEach(() => {
  useAuthStore.setState({ accessToken: null, user: null });
});

function renderBank() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/question-bank/:id", element: <QuestionEditorPage /> },
      { path: "/teacher/question-bank/new", element: <QuestionEditorPage /> },
    ],
    { initialEntries: [`/teacher/question-bank/${QUESTION_ID}`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

async function renderBankReady() {
  const user = renderBank();
  await screen.findByDisplayValue("have gone");
  return user;
}

function calls() {
  return vi.mocked(api).mock.calls.map(([method, path]) => `${method} ${path}`);
}

describe("the bank's question editor", () => {
  it("draws neither suggestion while the server has no assistant", async () => {
    aiOn({ ai: false });
    await renderBankReady();
    await waitFor(() => expect(calls()).toContain("get /public/status"));
    expect(screen.queryByRole("button", { name: ai.explain.write })).toBeNull();
    expect(screen.queryByRole("button", { name: ai.distract.suggest })).toBeNull();
  });

  it("offers wrong options only on a single or multiple choice question", async () => {
    stored = question({
      type: "short_answer",
      options: [],
      sampleAnswer: "went",
    });
    await renderBankReady().catch(() => undefined);
    expect(
      await screen.findByRole("button", { name: ai.explain.write }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: ai.distract.suggest })).toBeNull();
  });

  it("writes an explanation into the field, which only the teacher's Save writes", async () => {
    const user = await renderBankReady();
    await user.click(await screen.findByRole("button", { name: ai.explain.write }));
    const panel = await screen.findByRole("region", { name: ai.explain.title });
    expect(within(panel).getByText(EXPLANATION)).toBeInTheDocument();
    expect(
      within(panel).getByText("Đã dùng 300 tín dụng · còn 18.500"),
    ).toBeInTheDocument();
    expect(explanationBodies).toEqual([{ language: "vi" }]);
    expect(screen.getByRole("button", { name: ai.explain.again })).toBeInTheDocument();

    await user.click(within(panel).getByRole("button", { name: ai.explain.shorter }));
    const shorter = await screen.findByRole("region", { name: ai.explain.title });
    expect(within(shorter).getByText(SHORT)).toBeInTheDocument();
    await user.click(
      within(shorter).getByRole("button", { name: ai.explain.inEnglish }),
    );
    const english = await screen.findByRole("region", { name: ai.explain.title });
    expect(within(english).getByText(ENGLISH)).toBeInTheDocument();
    expect(explanationBodies).toEqual([
      { language: "vi" },
      { language: "vi", length: "short" },
      { language: "en" },
    ]);

    await user.click(within(english).getByRole("button", { name: ai.explain.use }));
    expect(await screen.findByText(ai.explain.used)).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: ai.explain.title })).toBeNull();
    expect(await screen.findByText(ENGLISH)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: ai.explain.write })).toHaveFocus();
    expect(patches).toEqual([]);

    await user.click(screen.getByRole("button", { name: "Lưu" }));
    await waitFor(() => expect(patches).toHaveLength(1));
    expect(patches[0]).toMatchObject({ explanation: ENGLISH });
  });

  it("discards a suggestion without touching the field", async () => {
    const user = await renderBankReady();
    await user.click(await screen.findByRole("button", { name: ai.explain.write }));
    const panel = await screen.findByRole("region", { name: ai.explain.title });
    await user.click(within(panel).getByRole("button", { name: ai.explain.discard }));
    expect(screen.queryByRole("region", { name: ai.explain.title })).toBeNull();
    expect(screen.queryByText(EXPLANATION)).toBeNull();
    expect(screen.getByRole("button", { name: "Lưu" })).toBeDisabled();
  });

  it("guards with the deck's toasts and asks for a save while the stored question differs", async () => {
    const user = await renderBankReady();
    const write = await screen.findByRole("button", { name: ai.explain.write });

    const correct = screen.getByDisplayValue("went");
    await user.clear(correct);
    await user.click(write);
    expect(await screen.findByText(ai.guard.key)).toBeInTheDocument();
    await user.type(correct, "goes");
    await user.click(write);
    expect(await screen.findByText(ai.guard.save)).toBeInTheDocument();
    expect(events).toEqual([]);
  });

  it("adds a suggested wrong option to the first empty option, then as a new one", async () => {
    const user = await renderBankReady();
    await user.click(
      screen.getByRole("button", { name: vi_.questionEditor.addOption }),
    );
    await user.click(await screen.findByRole("button", { name: ai.distract.suggest }));
    const panel = await screen.findByRole("region", { name: ai.distract.title });
    const rows = within(panel).getAllByRole("listitem");
    expect(rows).toHaveLength(2);
    expect(
      within(rows[0]!).getByText("Mixes up the present perfect."),
    ).toBeInTheDocument();

    await user.click(within(rows[0]!).getByRole("button", { name: ai.distract.add }));
    expect(screen.getByDisplayValue("has gone")).toBeInTheDocument();
    expect(screen.getAllByRole("textbox", { name: /^Lựa chọn/ })).toHaveLength(3);
    expect(
      within(rows[0]!).getByRole("button", { name: ai.distract.added }),
    ).toHaveAttribute("aria-disabled", "true");

    await user.click(within(rows[1]!).getByRole("button", { name: ai.distract.add }));
    expect(screen.getByDisplayValue("goes")).toBeInTheDocument();
    expect(screen.getAllByRole("textbox", { name: /^Lựa chọn/ })).toHaveLength(4);
    expect(patches).toEqual([]);

    await user.click(within(panel).getByRole("button", { name: ai.distract.close }));
    expect(screen.queryByRole("region", { name: ai.distract.title })).toBeNull();
    expect(screen.getByRole("button", { name: ai.distract.suggest })).toHaveFocus();
  });

  it("stops at eight options with the editor's own toast", async () => {
    stored = question({
      options: ["a", "b", "c", "d", "e", "f", "g", "h"].map((text, index) =>
        option(index + 1, text, index === 0),
      ),
    });
    const user = renderBank();
    await screen.findByDisplayValue("h");
    await user.click(await screen.findByRole("button", { name: ai.distract.suggest }));
    const panel = await screen.findByRole("region", { name: ai.distract.title });
    await user.click(
      within(panel).getAllByRole("button", { name: ai.distract.add })[0]!,
    );
    expect(await screen.findByText("Tối đa 8 lựa chọn")).toBeInTheDocument();
    expect(screen.queryByDisplayValue("has gone")).toBeNull();
  });

  it("says the assistant did not respond on a 503", async () => {
    server.use(
      http.post(`${BASE}/teacher/ai/questions/:id/distractors`, () =>
        aiError(503, "AI_UNAVAILABLE"),
      ),
    );
    const user = await renderBankReady();
    await user.click(await screen.findByRole("button", { name: ai.distract.suggest }));
    const panel = await screen.findByRole("region", { name: ai.distract.title });
    expect(within(panel).getByText(ai.unavailable)).toBeInTheDocument();
    expect(within(panel).queryAllByRole("listitem")).toHaveLength(0);
  });

  it("names the block on a 402 and then holds the buttons with the reason", async () => {
    server.use(
      http.post(`${BASE}/teacher/ai/questions/:id/explanation`, () =>
        aiError(402, "AI_CREDITS_EXHAUSTED", { scope: "pool" }),
      ),
    );
    const user = await renderBankReady();
    await user.click(await screen.findByRole("button", { name: ai.explain.write }));
    const panel = await screen.findByRole("region", { name: ai.explain.title });
    expect(within(panel).getByText(ai.blocked.pool)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: ai.explain.write })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
  });

  it("draws both disabled with the reason while the teacher is paused", async () => {
    aiOn({ usage: month({ paused: true }) });
    const user = await renderBankReady();
    await waitFor(() =>
      expect(screen.getByRole("button", { name: ai.explain.write })).toHaveAttribute(
        "aria-disabled",
        "true",
      ),
    );
    const write = screen.getByRole("button", { name: ai.explain.write });
    const suggest = screen.getByRole("button", { name: ai.distract.suggest });
    expect(suggest).toHaveAttribute("aria-disabled", "true");
    await user.click(write);
    await user.click(suggest);
    await user.hover(write);
    expect(await screen.findAllByText(ai.blocked.paused)).not.toHaveLength(0);
    expect(events).toEqual([]);
  });

  it("writes nothing from the suggestions: only the AI operations and the reads are called", async () => {
    const user = await renderBankReady();
    await user.click(await screen.findByRole("button", { name: ai.explain.write }));
    const panel = await screen.findByRole("region", { name: ai.explain.title });
    await user.click(within(panel).getByRole("button", { name: ai.explain.use }));
    await user.click(screen.getByRole("button", { name: ai.distract.suggest }));
    const options = await screen.findByRole("region", { name: ai.distract.title });
    await user.click(
      within(options).getAllByRole("button", { name: ai.distract.add })[0]!,
    );

    expect(
      calls().filter((call) => !/^(get |post \/teacher\/ai\/)/.test(call)),
    ).toEqual([]);
    for (const file of [
      "useAiSuggestion.ts",
      "questionAi.ts",
      "components/AiExplanationPanel.tsx",
      "components/AiDistractorsPanel.tsx",
      "components/ExplanationField.tsx",
    ]) {
      const source = readFileSync(resolve(SRC, file), "utf8");
      expect(source, file).not.toMatch(
        /@\/lib\/api\/client|question-bank\/api|\.\.\/api"/,
      );
    }
  });
});

describe("the builder's question editor", () => {
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
        id: SECTION_ID,
        ordinal: 0,
        title: "Ngữ pháp",
        instructions: null,
        questionIds: [QUESTION_ID],
      },
    ],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-02T00:00:00Z",
  };

  beforeEach(() => {
    stored = question();
    server.use(
      http.get(`${BASE}/teacher/tests/:id`, () =>
        contractJson("/teacher/tests/{id}", "get", 200, test),
      ),
      http.get(`${BASE}/teacher/tests/:id/versions`, () => HttpResponse.json([])),
    );
  });

  async function renderBuilder() {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const router = createMemoryRouter(
      [{ path: "/teacher/tests/:id/edit", element: <TestBuilderPage /> }],
      { initialEntries: [`/teacher/tests/${TEST_ID}/edit`] },
    );
    render(
      <QueryClientProvider client={client}>
        <RouterProvider router={router} />
        <Toaster />
      </QueryClientProvider>,
    );
    await screen.findByDisplayValue("have gone");
    return userEvent.setup();
  }

  it("saves the pending edit before asking, and autosaves the explanation it uses", async () => {
    const user = await renderBuilder();
    await user.type(screen.getByDisplayValue("have gone"), "!");
    await user.click(screen.getByText(vi_.builder.editor.moreOptions));
    await user.click(await screen.findByRole("button", { name: ai.explain.write }));
    const panel = await screen.findByRole("region", { name: ai.explain.title });
    expect(events).toEqual(["patch question", "explain"]);
    expect(patches[0]).toMatchObject({
      options: expect.arrayContaining([
        expect.objectContaining({ text: "have gone!" }),
      ]),
    });

    await user.click(within(panel).getByRole("button", { name: ai.explain.use }));
    await waitFor(() => expect(patches).toHaveLength(2), { timeout: 4000 });
    expect(patches[1]).toMatchObject({ explanation: EXPLANATION });
  });

  it("adds a suggested wrong option through the editor's autosave", async () => {
    const user = await renderBuilder();
    await user.click(await screen.findByRole("button", { name: ai.distract.suggest }));
    expect(events).toEqual(["distract"]);
    const panel = await screen.findByRole("region", { name: ai.distract.title });
    await user.click(
      within(panel).getAllByRole("button", { name: ai.distract.add })[0]!,
    );
    expect(screen.getByDisplayValue("has gone")).toBeInTheDocument();
    await waitFor(() => expect(patches).toHaveLength(1), { timeout: 4000 });
    expect(patches[0]).toMatchObject({
      options: expect.arrayContaining([
        expect.objectContaining({ text: "has gone", isCorrect: false }),
      ]),
    });
  });
});
