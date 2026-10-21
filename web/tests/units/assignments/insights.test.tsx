import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { http } from "msw";
import type { components } from "@/lib/api/schema";
import { api } from "@/lib/api/client";
import vi_ from "@/lib/i18n/locales/vi.json";
import { useAuthStore } from "@/stores/auth";
import {
  DETAIL_API,
  DETAIL_ID,
  DETAIL_TEST_ID,
  detailAssignment,
  detailRow,
  renderDetail,
  serveDetail,
} from "@tests/support/assignmentDetail";
import { contractJson } from "@tests/support/contractResponse";
import { teacherUser } from "@tests/support/fixtures";
import { bankQuestion } from "@tests/support/questionBank";
import { server } from "@tests/support/server";
import { aiError, aiOn, month } from "../ai/support";

vi.mock("@/lib/api/client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api/client")>();
  return { ...actual, api: vi.fn(actual.api) };
});

globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

type InsightsResult = components["schemas"]["AiInsightsResult"];

const copy = vi_.assignmentDetail.insights;
const STUDENT = "Lê Văn An";
const STUDENT_ID = "018f0000-0000-7000-8000-0000000000e1";
const ATTEMPT_ID = "018f0000-0000-7000-8000-0000000000e9";
const PASSAGE =
  "Cities around the world are rethinking how they use open land. Fifty years ago, planners saw parks as decoration.";
const INSIGHTS_PATH = `${DETAIL_API}/teacher/ai/assignments/${DETAIL_ID}/insights`;

const ANSWERED: InsightsResult = {
  basis: 18,
  items: [
    {
      kind: "key",
      title: "Q27: most wrong answers are nearly right",
      body: "11 of 14 wrong answers were “playing street” or “play street”.",
      action: "review",
      questionNumbers: [27],
      types: ["short_answer"],
    },
    {
      kind: "problem",
      title: "Q31: True and False are getting mixed up",
      body: "9 of 18 chose False where the answer is True.",
      action: "practice",
      questionNumbers: [31],
      types: ["true_false", "short_answer"],
    },
    {
      kind: "strength",
      title: "Main-idea questions are solid",
      body: "83% got Q3 right.",
      action: "none",
      questionNumbers: [3],
      types: ["single_choice"],
    },
  ],
  usage: { credits: 60, left: 18740 },
};

const NEW_TEST = "018f0000-0000-7000-8000-00000000cc01";
const NEW_QUESTION = "018f0000-0000-7000-8000-00000000cc11";

function draftTest(title: string) {
  return {
    skills: [],
    assignments: { live: 0, scheduled: 0, closed: 0 },
    unpublishedChanges: null,
    id: NEW_TEST,
    title,
    description: null,
    status: "draft",
    currentVersion: 0,
    nextVersion: 1,
    totalPoints: 0,
    questionCount: 0,
    audioCount: 0,
    sections: [],
    createdAt: "2026-10-13T00:00:00Z",
    updatedAt: "2026-10-13T00:00:00Z",
  };
}

function reached() {
  return vi.mocked(api).mock.calls.map(([method, path]) => `${method} ${path}`);
}

function serve({
  handedIn = 18,
  answers = [
    () => contractJson("/teacher/ai/assignments/{id}/insights", "post", 200, ANSWERED),
  ],
  previewVersion = 3,
  previewed = [],
}: {
  handedIn?: number;
  answers?: (() => Response | Promise<Response>)[];
  previewVersion?: number;
  previewed?: (string | null)[];
} = {}) {
  serveDetail(detailAssignment(), {
    rows: [
      detailRow(STUDENT_ID, STUDENT, {
        state: "submitted",
        attemptId: ATTEMPT_ID,
      }),
    ],
    analysis: {
      handedIn,
      items: [
        {
          questionId: "018f0000-0000-7000-8000-000000000101",
          number: 27,
          type: "short_answer",
          promptExcerpt: "Term for streets closed to cars",
          answered: handedIn,
          correctRate: 0.22,
        },
      ],
    },
  });
  const bodies: (string | null)[] = [];
  let call = 0;
  server.use(
    http.get(`${DETAIL_API}/teacher/tests/${DETAIL_TEST_ID}/preview`, ({ request }) => {
      previewed.push(new URL(request.url).searchParams.get("version"));
      return contractJson("/teacher/tests/{id}/preview", "get", 200, {
        version: previewVersion,
        questions: [],
        groups: [
          {
            id: "018f0000-0000-7000-8000-00000000aa10",
            sectionId: "018f0000-0000-7000-8000-00000000aa04",
            title: "Reading",
            instructions: null,
            questionIds: [],
            stimuli: [
              {
                id: "018f0000-0000-7000-8000-00000000aa11",
                title: "Urban green space",
                content: {
                  format: "semantic_v1",
                  blocks: [
                    {
                      type: "paragraph",
                      content: [{ type: "text", text: PASSAGE, marks: [] }],
                    },
                  ],
                },
                gaps: [],
              },
            ],
            recordings: [],
            assets: [],
          },
        ],
      });
    }),
    http.post(INSIGHTS_PATH, async ({ request }) => {
      bodies.push(await request.text());
      const answer = answers[Math.min(call, answers.length - 1)]!;
      call += 1;
      return answer();
    }),
  );
  return bodies;
}

beforeEach(() => {
  vi.mocked(api).mockClear();
  aiOn();
});

afterEach(() => {
  useAuthStore.setState({ accessToken: null, user: null });
});

async function explainButton() {
  return screen.findByRole("button", { name: copy.explain });
}

describe("Explain results with AI", () => {
  it("is not drawn while the assistant is off", async () => {
    aiOn({ ai: false });
    serve();
    renderDetail("questions");
    expect(await screen.findByText("C27")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: copy.explain })).toBeNull();
    expect(reached()).not.toContain("post /teacher/ai/assignments/{id}/insights");
    expect(reached()).not.toContain("get /teacher/tests/{id}/preview");
  });

  it("reads the results, shows what it read and the cards, and links Review answers to the assignment's grading", async () => {
    let release: () => void = () => undefined;
    const held = new Promise<void>((resolve) => (release = resolve));
    const bodies = serve({
      answers: [
        async () => {
          await held;
          return contractJson(
            "/teacher/ai/assignments/{id}/insights",
            "post",
            200,
            ANSWERED,
          );
        },
      ],
    });
    const user = renderDetail("questions");
    await user.click(await explainButton());
    const running = await screen.findByRole("button", { name: copy.reading });
    expect(running).toHaveAttribute("aria-busy", "true");
    release();
    const panel = await screen.findByRole("region", { name: copy.title });
    expect(within(panel).getByText("Dựa trên 18 bài làm")).toBeInTheDocument();
    expect(within(panel).getAllByRole("listitem")).toHaveLength(3);
    expect(
      within(panel).getByText("Q27: most wrong answers are nearly right"),
    ).toBeInTheDocument();
    expect(within(panel).getByRole("link", { name: copy.review })).toHaveAttribute(
      "href",
      `/teacher/grading?assignment=${DETAIL_ID}`,
    );
    expect(within(panel).getAllByRole("button", { name: copy.practice })).toHaveLength(
      1,
    );
    expect(screen.getByRole("button", { name: copy.refresh })).toBeInTheDocument();
    expect(bodies).toEqual([""]);
  });

  it("writes nothing and never names a student, through a run, a refresh and Hide insights", async () => {
    serve();
    const user = renderDetail("questions");
    await user.click(await explainButton());
    const panel = await screen.findByRole("region", { name: copy.title });
    expect(within(panel).queryByText(new RegExp(STUDENT))).toBeNull();
    await user.click(screen.getByRole("button", { name: copy.refresh }));
    await screen.findByRole("region", { name: copy.title });
    await user.click(screen.getByRole("button", { name: copy.hide }));
    expect(screen.queryByRole("region", { name: copy.title })).toBeNull();
    const again = screen.getByRole("button", { name: copy.explain });
    expect(again).toHaveFocus();
    const calls = reached();
    expect(
      calls.filter((call) => call === "post /teacher/ai/assignments/{id}/insights"),
    ).toHaveLength(2);
    expect(
      calls.filter(
        (call) =>
          !call.startsWith("get ") &&
          call !== "post /teacher/ai/assignments/{id}/insights",
      ),
    ).toEqual([]);
  });

  it("keeps the insights for the visit when the teacher moves to another tab and back", async () => {
    serve();
    const user = renderDetail("questions");
    await user.click(await explainButton());
    await screen.findByRole("region", { name: copy.title });
    await user.click(screen.getByRole("tab", { name: "Học viên" }));
    await user.click(screen.getByRole("tab", { name: "Câu hỏi" }));
    expect(await screen.findByRole("region", { name: copy.title })).toBeInTheDocument();
  });

  it("says why and links to Settings › AI assistant while reading student answers is off", async () => {
    useAuthStore.setState({
      accessToken: "token",
      user: { ...teacherUser, preferences: { ai: { readStudentAnswers: false } } },
    });
    serve();
    renderDetail("questions");
    const link = await screen.findByRole("link", { name: "Cài đặt › Trợ lý AI" });
    expect(link).toHaveAttribute("href", "/teacher/settings/ai");
    expect(link.closest("p")).toHaveTextContent(
      "Để AI giải thích kết quả, hãy bật “Cho AI đọc bài làm của học viên” trong Cài đặt › Trợ lý AI.",
    );
    expect(screen.queryByRole("button", { name: copy.explain })).toBeNull();
  });

  it("is disabled with the reason while the teacher is paused, and calls nothing", async () => {
    aiOn({ usage: month({ paused: true }) });
    serve();
    const user = renderDetail("questions");
    await waitFor(async () =>
      expect(await explainButton()).toHaveAttribute("aria-disabled", "true"),
    );
    const button = await explainButton();
    await user.hover(button);
    expect((await screen.findAllByText(vi_.ai.blocked.paused)).length).toBeGreaterThan(
      0,
    );
    await user.click(button);
    expect(reached()).not.toContain("post /teacher/ai/assignments/{id}/insights");
  });

  it("is disabled while nothing is handed in", async () => {
    serve({ handedIn: 0 });
    renderDetail("questions");
    await waitFor(async () =>
      expect(await explainButton()).toHaveAttribute("aria-disabled", "true"),
    );
  });

  it("offers Try again when the assistant does not answer, and shows the insights after it", async () => {
    serve({
      answers: [
        () => aiError(503, "AI_UNAVAILABLE"),
        () =>
          contractJson("/teacher/ai/assignments/{id}/insights", "post", 200, ANSWERED),
      ],
    });
    const user = renderDetail("questions");
    await user.click(await explainButton());
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(vi_.ai.unavailable);
    await user.click(within(alert).getByRole("button", { name: vi_.ai.tryAgain }));
    expect(await screen.findByRole("region", { name: copy.title })).toBeInTheDocument();
    expect(screen.queryByText(vi_.ai.unavailable)).toBeNull();
  });

  it("gives the allowance sentence on a 402", async () => {
    serve({
      answers: [() => aiError(402, "AI_CREDITS_EXHAUSTED", { scope: "teacher" })],
    });
    const user = renderDetail("questions");
    await user.click(await explainButton());
    expect(await screen.findByRole("alert")).toHaveTextContent(vi_.ai.blocked.teacher);
  });

  it("says when AI found nothing to point out", async () => {
    serve({
      answers: [
        () =>
          contractJson("/teacher/ai/assignments/{id}/insights", "post", 200, {
            basis: 1,
            items: [],
            usage: { credits: 60, left: 100 },
          }),
      ],
    });
    const user = renderDetail("questions");
    await user.click(await explainButton());
    const panel = await screen.findByRole("region", { name: copy.title });
    expect(within(panel).getByText(copy.empty)).toBeInTheDocument();
    expect(within(panel).getByText("Dựa trên 1 bài làm")).toBeInTheDocument();
  });
});

describe("Make a practice set", () => {
  it("opens Generate with the insight's types and saves a new draft test through the existing writes", async () => {
    serve();
    const created: unknown[] = [];
    const tests: unknown[] = [];
    const outlines: unknown[] = [];
    server.use(
      http.post(`${DETAIL_API}/teacher/ai/questions/generate`, () =>
        contractJson("/teacher/ai/questions/generate", "post", 200, {
          items: [
            {
              type: "short_answer",
              prompt: "What do cities rethink?",
              answerKey: ["open land"],
              explanation: "",
            },
          ],
          usage: { credits: 200, left: 18540 },
        }),
      ),
      http.post(`${DETAIL_API}/teacher/questions`, async ({ request }) => {
        const body = (await request.json()) as { tags: string[] };
        created.push(body);
        return contractJson("/teacher/questions", "post", 201, {
          ...bankQuestion({ id: NEW_QUESTION, type: "short_answer", tags: body.tags }),
          usedInTests: undefined,
        });
      }),
      http.post(`${DETAIL_API}/teacher/tests`, async ({ request }) => {
        const body = (await request.json()) as { title: string };
        tests.push(body);
        return contractJson("/teacher/tests", "post", 201, draftTest(body.title));
      }),
      http.patch(`${DETAIL_API}/teacher/tests/${NEW_TEST}`, async ({ request }) => {
        const body = (await request.json()) as { title: string };
        outlines.push(body);
        return contractJson("/teacher/tests/{id}", "patch", 200, draftTest(body.title));
      }),
    );
    const user = renderDetail("questions");
    await user.click(await explainButton());
    const panel = await screen.findByRole("region", { name: copy.title });
    await user.click(within(panel).getByRole("button", { name: copy.practice }));
    const dialog = await screen.findByRole("dialog", { name: vi_.ai.generate.title });
    const types = within(dialog).getByRole("group", { name: vi_.ai.generate.types });
    const pressed = within(types)
      .getAllByRole("button")
      .filter((chip) => chip.getAttribute("aria-pressed") === "true")
      .map((chip) => chip.textContent);
    expect(pressed).toEqual(["Đúng/Sai", "Tự luận"]);
    expect(within(dialog).getByLabelText(vi_.ai.generate.source)).toHaveValue(PASSAGE);
    await user.click(within(dialog).getByRole("button", { name: vi_.ai.generate.run }));
    await user.click(
      await within(dialog).findByRole("button", {
        name: vi_.ai.generate.saveAsPractice,
      }),
    );
    expect(
      await screen.findByText(
        "Đã lưu đề luyện tập gồm 1 câu thành bản nháp trong Đề thi",
      ),
    ).toBeInTheDocument();
    expect(created).toHaveLength(1);
    expect(created[0]).toMatchObject({ type: "short_answer", tags: ["Bản nháp AI"] });
    expect(tests).toEqual([
      { title: "Unit 5 — Present perfect & listening · Luyện tập" },
    ]);
    expect(outlines).toEqual([
      {
        expectedUpdatedAt: "2026-10-13T00:00:00Z",
        title: "Unit 5 — Present perfect & listening · Luyện tập",
        outlineFormat: "group_v1",
        sections: [
          {
            id: null,
            title: "Phần 1",
            instructions: null,
            units: [{ kind: "question", id: NEW_QUESTION }],
            questionIds: [NEW_QUESTION],
          },
        ],
      },
    ]);
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("reads the passages of the pinned version only while the assistant is offered", async () => {
    const previewed: (string | null)[] = [];
    serve({ previewed });
    renderDetail("questions");
    await explainButton();
    await waitFor(() => expect(previewed).toEqual(["3"]));
  });

  it.each([2, 4])(
    "never substitutes the passages of response version %i for the pinned version",
    async (previewVersion) => {
      const previewed: (string | null)[] = [];
      serve({ previewVersion, previewed });
      const user = renderDetail("questions");
      await user.click(await explainButton());
      await waitFor(() => expect(previewed).toEqual(["3"]));
      const panel = await screen.findByRole("region", { name: copy.title });
      await user.click(within(panel).getByRole("button", { name: copy.practice }));
      const dialog = await screen.findByRole("dialog", { name: vi_.ai.generate.title });
      expect(within(dialog).queryByDisplayValue(PASSAGE)).toBeNull();
      expect(within(dialog).queryByText(PASSAGE)).toBeNull();
    },
  );

  it("brings back a practice set that finished behind a closed drawer from the toast", async () => {
    serve();
    let release: () => void = () => undefined;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    server.use(
      http.post(`${DETAIL_API}/teacher/ai/questions/generate`, async () => {
        await gate;
        return contractJson("/teacher/ai/questions/generate", "post", 200, {
          items: [
            {
              type: "short_answer",
              prompt: "What do cities rethink?",
              answerKey: ["open land"],
              explanation: "",
            },
          ],
          usage: { credits: 200, left: 18540 },
        });
      }),
    );
    const user = renderDetail("questions");
    await user.click(await explainButton());
    const panel = await screen.findByRole("region", { name: copy.title });
    await user.click(within(panel).getByRole("button", { name: copy.practice }));
    let dialog = await screen.findByRole("dialog", { name: vi_.ai.generate.title });
    await user.click(within(dialog).getByRole("button", { name: vi_.ai.generate.run }));
    await within(dialog).findByRole("status");
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    release();

    await user.click(await screen.findByRole("button", { name: vi_.ai.finished.open }));
    dialog = await screen.findByRole("dialog", { name: vi_.ai.generate.title });
    expect(await within(dialog).findByText("What do cities rethink?")).toBeVisible();
  });
});
