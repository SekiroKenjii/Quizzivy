import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
import TestBuilderPage from "@/features/tests/pages/teacher/TestBuilderPage";
import type {
  GroupBundle,
  GroupUpdateInput,
  StoredGroup,
} from "@/features/question-groups/api";
import vi_ from "@/lib/i18n/locales/vi.json";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { assistantUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import { storedGroupBundle } from "@tests/support/storedGroupBundle";
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
  const { unmount } = render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  await screen.findByDisplayValue("have gone");
  return { user: userEvent.setup(), client, unmount };
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

const GROUP_ID = "018f0000-0000-7000-8000-0000000000f1";
const MEMBER_ID = "018f0000-0000-7000-8000-0000000000e2";
const STIMULUS_ID = "018f0000-0000-7000-8000-0000000000e3";
const PARKS =
  "Parks cool the streets in summer, and cities are planting more trees every year.";
const GROUP_BUTTON = "Thêm 2 câu vào nhóm Reading club";

function member(id: string, prompt: string): GroupBundle["questions"][number] {
  return {
    id,
    input: {
      type: "single_choice",
      level: null,
      skill: null,
      prompt,
      points: 1,
      options: [
        { text: "Cool the streets", isCorrect: true },
        { text: "Heat the streets", isCorrect: false },
      ],
      blanks: [],
      tags: [],
    },
  };
}

function readingClub(questions: GroupBundle["questions"]): StoredGroup {
  return {
    bundle: {
      group: {
        id: GROUP_ID,
        title: "Reading club",
        members: questions.map((item) => ({
          questionId: item.id,
          optionOrder: "shuffle" as const,
        })),
        stimuli: [
          {
            id: STIMULUS_ID,
            title: "Parks",
            content: {
              format: "semantic_v1",
              blocks: [
                {
                  type: "paragraph",
                  content: [{ type: "text", text: PARKS, marks: [] }],
                },
              ],
            },
            gaps: [],
          },
        ],
        recordings: [],
      },
      questions,
    },
    ownerSectionId: SECTION_ID,
    revision: 1,
    testUpdatedAt: test.updatedAt,
    archivedAt: null,
    createdAt: test.createdAt,
    updatedAt: test.updatedAt,
    assets: [],
    unavailableAssetIds: [],
  };
}

function generateAnswers(items: unknown[]) {
  server.use(
    http.post(`${BASE}/teacher/ai/questions/generate`, async ({ request }) => {
      events.push(`generate ${JSON.stringify(await request.json())}`);
      return contractJson("/teacher/ai/questions/generate", "post", 200, {
        items,
        usage: { credits: 500, left: 1000 },
      });
    }),
  );
}

describe("Generate in a test with a shared-context group", () => {
  let stored: StoredGroup;
  let puts: GroupUpdateInput[];
  let stale: boolean;

  beforeEach(() => {
    stored = readingClub([member(MEMBER_ID, "What do parks do?")]);
    puts = [];
    stale = false;
    server.use(
      http.get(`${BASE}/teacher/tests/:id`, () =>
        contractJson("/teacher/tests/{id}", "get", 200, {
          ...test,
          sections: [
            {
              ...test.sections[0]!,
              units: [
                { kind: "question", id: QUESTION_ID },
                { kind: "group", id: GROUP_ID },
              ],
            },
          ],
        }),
      ),
      http.get(`${BASE}/teacher/question-groups/:id`, () =>
        contractJson("/teacher/question-groups/{id}", "get", 200, stored),
      ),
      http.put(`${BASE}/teacher/question-groups/:id`, async ({ request }) => {
        const body = (await request.json()) as GroupUpdateInput;
        puts.push(body);
        if (stale || body.expectedRevision !== stored.revision)
          return HttpResponse.json(
            { error: { code: "STALE_WRITE", message: "Đã có thay đổi ở nơi khác." } },
            { status: 409 },
          );
        stored = {
          ...stored,
          bundle: storedGroupBundle(body.bundle),
          revision: stored.revision + 1,
          testUpdatedAt: "2026-01-04T00:00:00Z",
        };
        return contractJson("/teacher/question-groups/{id}", "put", 200, stored);
      }),
      http.post(`${BASE}/teacher/questions`, () => {
        events.push("create question");
        return contractJson("/teacher/questions", "post", 201, created);
      }),
    );
    generateAnswers([
      {
        type: "short_answer",
        prompt: "What do parks do in summer?",
        answerKey: ["cool the streets"],
        explanation: "",
      },
      {
        type: "true_false",
        prompt: "Cities plant fewer trees.",
        answerKey: ["false"],
        explanation: "",
      },
    ]);
  });

  type User = ReturnType<typeof userEvent.setup>;

  async function press(
    user: User,
    signal: AbortSignal,
    target: HTMLElement | Promise<HTMLElement>,
  ) {
    const element = await target;
    signal.throwIfAborted();
    await user.click(element);
  }

  async function runFromThePassage(user: User, signal: AbortSignal) {
    await press(user, signal, screen.findByRole("button", { name: ai.entry.generate }));
    const dialog = await screen.findByRole("dialog");
    expect(
      await within(dialog).findByRole("button", {
        name: "Bài đọc 1 · Parks",
        pressed: true,
      }),
    ).toBeVisible();
    await press(
      user,
      signal,
      within(dialog).getByRole("button", { name: ai.generate.run }),
    );
    const add = await within(dialog).findByRole("button", { name: GROUP_BUTTON });
    return { dialog, add };
  }

  async function generateFromThePassage(user: User, signal: AbortSignal) {
    const { dialog, add } = await runFromThePassage(user, signal);
    await press(user, signal, add);
    return dialog;
  }

  it("appends what it made from the group's passage to that group and opens the first", async ({
    signal,
  }) => {
    aiOn();
    const { user } = await renderBuilder();
    await generateFromThePassage(user, signal);

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(events).toEqual([
      `generate ${JSON.stringify({
        types: ["single_choice", "true_false"],
        count: 5,
        level: "b2",
        explanations: true,
        language: "vi",
        testId: TEST_ID,
        stimulusId: STIMULUS_ID,
      })}`,
    ]);
    expect(puts).toHaveLength(1);
    const sent = puts[0]!;
    expect(sent.expectedRevision).toBe(1);
    expect(sent.expectedTestUpdatedAt).toBe(test.updatedAt);
    expect(sent.bundle.group.members.map((item) => item.questionId)).toEqual(
      sent.bundle.questions.map((item) => item.id),
    );
    expect(sent.bundle.group.members[0]).toEqual({
      questionId: MEMBER_ID,
      optionOrder: "shuffle",
    });
    expect(sent.bundle.questions.slice(1).map((item) => item.input)).toMatchObject([
      {
        type: "short_answer",
        prompt: "What do parks do in summer?",
        level: "b2",
        sampleAnswer: "cool the streets",
        tags: [ai.draftTag],
      },
      { type: "true_false", prompt: "Cities plant fewer trees.", tags: [ai.draftTag] },
    ]);
    expect(sent.bundle.group.stimuli.map((item) => item.id)).toEqual([STIMULUS_ID]);
    expect(await screen.findByDisplayValue("cool the streets")).toBeVisible();
  });

  it("reloads the group's open editor on the first new member", async ({ signal }) => {
    aiOn();
    const { user } = await renderBuilder();
    await press(
      user,
      signal,
      screen.findByRole("button", { name: /What do parks do\?/ }, { timeout: 5000 }),
    );
    expect(await screen.findByDisplayValue("Reading club")).toBeVisible();
    await generateFromThePassage(user, signal);

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(puts).toHaveLength(1);
    expect(await screen.findByDisplayValue("cool the streets")).toBeVisible();
    expect(screen.getByDisplayValue("Reading club")).toBeVisible();
  });

  it("refuses before writing when the cached group has no room for them", async ({
    signal,
  }) => {
    aiOn();
    const { user, client } = await renderBuilder();
    const { dialog, add } = await runFromThePassage(user, signal);
    stored = readingClub(
      Array.from({ length: 199 }, (_, index) =>
        member(
          `018f0000-0000-7000-8000-${String(index).padStart(12, "0")}`,
          `Question ${index + 1}`,
        ),
      ),
    );
    act(() => {
      client.setQueryData(["teacher-group", GROUP_ID], stored);
    });
    await press(user, signal, add);

    expect(
      await within(dialog).findByText(
        "Một nhóm chứa tối đa 200 câu, và nhóm Reading club chỉ còn chỗ cho 1 câu. Hãy bỏ chọn bớt rồi thử lại.",
      ),
    ).toBeVisible();
    await waitFor(() => expect(add).toBeEnabled());
    expect(puts).toHaveLength(0);
    expect(events).not.toContain("create question");
  });

  it("keeps the panel open with the server's answer when the group changed elsewhere", async ({
    signal,
  }) => {
    aiOn();
    stale = true;
    const { user } = await renderBuilder();
    const { dialog, add } = await runFromThePassage(user, signal);
    await press(user, signal, add);

    expect(await within(dialog).findByText("Đã có thay đổi ở nơi khác.")).toBeVisible();
    await waitFor(() => expect(add).toBeEnabled());
    expect(puts.map((body) => body.expectedRevision)).toEqual([1]);
    expect(events).not.toContain("create question");
  });

  it("still appends what it made from pasted text to the section", async ({
    signal,
  }) => {
    aiOn();
    generateAnswers([
      {
        type: "short_answer",
        prompt: "What do cities rethink?",
        answerKey: ["open land"],
        explanation: "",
      },
    ]);
    const { user, unmount } = await renderBuilder();
    await press(user, signal, screen.findByRole("button", { name: ai.entry.generate }));
    const dialog = await screen.findByRole("dialog");
    const source = within(dialog).getByLabelText(ai.generate.source);
    await user.clear(source);
    await user.type(
      source,
      "Cities around the world are rethinking how they use open land.",
    );
    await press(
      user,
      signal,
      within(dialog).getByRole("button", { name: ai.generate.run }),
    );
    await press(
      user,
      signal,
      within(dialog).findByRole("button", { name: "Thêm 1 câu vào Ngữ pháp" }),
    );

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(events).toContain("create question");
    expect(puts).toHaveLength(0);
    expect(await screen.findByDisplayValue("open land")).toBeVisible();
    unmount();
    await waitFor(() =>
      expect(events).toContain(`patch test Unit 5 ${QUESTION_ID},${CREATED_ID}`),
    );
  });
});
