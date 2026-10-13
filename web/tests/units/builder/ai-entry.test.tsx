import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import TestBuilderPage from "@/features/tests/pages/teacher/TestBuilderPage";
import vi_ from "@/lib/i18n/locales/vi.json";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { assistantUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import { aiOn, BASE, month } from "../ai/support";
import "@/lib/i18n";

const ai = vi_.ai;
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const SECTION_ID = "018f0000-0000-7000-8000-0000000000c1";
const QUESTION_ID = "018f0000-0000-7000-8000-0000000000b1";
const CREATED_ID = "018f0000-0000-7000-8000-0000000000b9";
const WRONG_OPTION = "018f0000-0000-7000-8000-0000000000d2";

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

const question = {
  level: null,
  skill: null,
  id: QUESTION_ID,
  type: "single_choice" as const,
  prompt: "They ___ to the museum.",
  media: null,
  audio: null,
  transcript: null,
  options: [
    {
      id: "018f0000-0000-7000-8000-0000000000d1",
      ordinal: 0,
      text: "went",
      isCorrect: true,
    },
    { id: WRONG_OPTION, ordinal: 1, text: "have gone", isCorrect: false },
  ],
  blanks: [],
  points: 1,
  explanation: null,
  sampleAnswer: null,
  tags: [],
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

let events: string[] = [];

beforeEach(() => {
  events = [];
  server.use(
    http.get(`${BASE}/teacher/tests/:id`, () =>
      contractJson("/teacher/tests/{id}", "get", 200, test),
    ),
    http.get(`${BASE}/teacher/questions/:id`, ({ params }) =>
      contractJson(
        "/teacher/questions/{id}",
        "get",
        200,
        params["id"] === CREATED_ID ? created : question,
      ),
    ),
    http.patch(`${BASE}/teacher/tests/:id`, async ({ request }) => {
      const body = (await request.json()) as {
        title: string;
        sections: { questionIds: string[] }[];
      };
      events.push(
        `patch test ${body.title} ${body.sections[0]?.questionIds.join(",")}`,
      );
      return contractJson("/teacher/tests/{id}", "patch", 200, {
        ...test,
        title: body.title,
        sections: [
          { ...test.sections[0]!, questionIds: body.sections[0]!.questionIds },
        ],
        updatedAt: "2026-01-03T00:00:00Z",
      });
    }),
  );
});

afterEach(() => {
  useAuthStore.setState({ accessToken: null, user: null });
});

const created = {
  ...question,
  id: CREATED_ID,
  type: "short_answer" as const,
  prompt: "What do cities rethink?",
  options: [],
  level: "b2" as const,
  sampleAnswer: "open land",
  tags: [ai.draftTag],
};

async function renderBuilder() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/teacher/tests/:id/edit", element: <TestBuilderPage /> }],
    { initialEntries: [`/teacher/tests/${TEST_ID}/edit`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  await screen.findByDisplayValue("have gone");
  return { user: userEvent.setup() };
}

describe("the builder's AI entry points", () => {
  it("are not drawn while the server has no assistant", async () => {
    aiOn({ ai: false });
    await renderBuilder();
    expect(screen.getByRole("button", { name: vi_.builder.preview })).toBeVisible();
    expect(screen.queryByRole("button", { name: ai.entry.check })).toBeNull();
    expect(screen.queryByRole("button", { name: ai.entry.generate })).toBeNull();
  });

  it("are not drawn, and the status is not read, without ai.use", async () => {
    let read = false;
    server.use(
      http.get(`${BASE}/public/status`, () => {
        read = true;
        return contractJson("/public/status", "get", 200, {
          maintenance: null,
          ai: true,
        });
      }),
    );
    useAuthStore.setState({ accessToken: "token", user: assistantUser });
    await renderBuilder();
    expect(screen.queryByRole("button", { name: ai.entry.generate })).toBeNull();
    expect(read).toBe(false);
  });

  it("sit before Preview, as the deck's title bar has them", async () => {
    aiOn();
    await renderBuilder();
    const check = await screen.findByRole("button", { name: ai.entry.check });
    const bar = check.parentElement!;
    expect(
      within(bar)
        .getAllByRole("button")
        .map((button) => button.getAttribute("aria-label") ?? button.textContent),
    ).toEqual([
      vi_.builder.versions,
      ai.entry.check,
      ai.entry.generate,
      vi_.builder.preview,
      vi_.builder.publish,
    ]);
  });

  it("stay visible but do nothing while the teacher is paused, with the reason as a tooltip", async () => {
    aiOn({ usage: month({ paused: true }) });
    const { user } = await renderBuilder();
    const generate = await screen.findByRole("button", { name: ai.entry.generate });
    await waitFor(() => expect(generate).toHaveAttribute("aria-disabled", "true"));
    act(() => generate.focus());
    expect(await screen.findByRole("tooltip")).toHaveTextContent(ai.blocked.paused);
    await user.click(generate);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("save pending edits before a panel opens, as Preview does", async () => {
    aiOn();
    const { user } = await renderBuilder();
    await user.click(screen.getByRole("button", { name: vi_.builder.titleLabel }));
    await user.type(
      screen.getByRole("textbox", { name: vi_.builder.titleLabel }),
      " (AI)",
    );
    await user.click(await screen.findByRole("button", { name: ai.entry.check }));
    await screen.findByRole("dialog");
    expect(events).toEqual([`patch test Unit 5 (AI) ${QUESTION_ID}`]);
  });

  it("add the kept questions to the open question's section and open the first", async () => {
    aiOn();
    let posted: Record<string, unknown> = {};
    server.use(
      http.post(`${BASE}/teacher/ai/questions/generate`, () =>
        contractJson("/teacher/ai/questions/generate", "post", 200, {
          items: [
            {
              type: "short_answer",
              prompt: "What do cities rethink?",
              answerKey: ["open land"],
              explanation: "",
            },
          ],
          usage: { credits: 500, left: 1000 },
        }),
      ),
      http.post(`${BASE}/teacher/questions`, async ({ request }) => {
        posted = (await request.json()) as Record<string, unknown>;
        events.push("create question");
        return contractJson("/teacher/questions", "post", 201, created);
      }),
    );
    const { user } = await renderBuilder();
    await user.click(await screen.findByRole("button", { name: ai.entry.generate }));
    const dialog = await screen.findByRole("dialog");
    await user.type(
      within(dialog).getByLabelText(ai.generate.source),
      "Cities around the world are rethinking how they use open land.",
    );
    await user.click(within(dialog).getByRole("button", { name: ai.generate.run }));
    await user.click(
      await within(dialog).findByRole("button", { name: "Thêm 1 câu vào Ngữ pháp" }),
    );

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(posted).toMatchObject({
      type: "short_answer",
      prompt: "What do cities rethink?",
      level: "b2",
      sampleAnswer: "open land",
      tags: [ai.draftTag],
    });
    expect(await screen.findByDisplayValue("open land")).toBeVisible();
    await waitFor(
      () => expect(events).toContain(`patch test Unit 5 ${QUESTION_ID},${CREATED_ID}`),
      { timeout: 4000 },
    );
  });

  it("write a one-click fix into the question and reload its open editor", async () => {
    aiOn();
    let patched: { options: { id: string | null; text: string }[] } | null = null;
    server.use(
      http.post(`${BASE}/teacher/ai/tests/:id/check`, () =>
        contractJson("/teacher/ai/tests/{id}/check", "post", 200, {
          findings: [
            {
              severity: "worth_a_look",
              questionId: QUESTION_ID,
              number: 1,
              type: "single_choice",
              title: "They ___ to the museum.",
              issue: "Option B could be argued.",
              fix: {
                kind: "option_text",
                description: "Use “has gone”.",
                label: "Use this option",
                payload: { optionId: WRONG_OPTION, text: "has gone" },
              },
            },
          ],
          usage: { credits: 400, left: 1000 },
        }),
      ),
      http.patch(`${BASE}/teacher/questions/:id`, async ({ request }) => {
        patched = (await request.json()) as typeof patched;
        return contractJson("/teacher/questions/{id}", "patch", 200, {
          ...question,
          options: [
            question.options[0]!,
            { ...question.options[1]!, text: "has gone" },
          ],
        });
      }),
    );
    const { user } = await renderBuilder();
    await user.click(await screen.findByRole("button", { name: ai.entry.check }));
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: ai.check.run }));
    expect(await within(dialog).findByText("Câu 1")).toBeVisible();
    await user.click(within(dialog).getByRole("button", { name: "Use this option" }));

    await waitFor(() => expect(patched).not.toBeNull());
    expect(patched!.options.map((option) => option.text)).toEqual(["went", "has gone"]);
    await user.keyboard("{Escape}");
    expect(await screen.findByDisplayValue("has gone")).toBeVisible();
  });
});
