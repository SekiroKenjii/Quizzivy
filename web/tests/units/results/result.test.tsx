import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import userEvent from "@testing-library/user-event";
import { delay, http, HttpResponse } from "msw";
import ResultPage from "@/features/results/pages/ResultPage";
import type { AttemptResult } from "@/features/results/api";
import type { components } from "@/lib/api/schema";
import i18n from "@/lib/i18n";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { viewport } from "@tests/support/viewport";
import { previewGroup } from "@tests/support/groupPreview";
import {
  ATTEMPT_ID,
  BASE,
  CLOSED,
  OPEN,
  PART_ONE,
  PART_TWO,
  SCORE_ONLY,
  assignmentCard,
  blank,
  choice,
  essay,
  grading,
  grammarCheck,
  justSubmitted,
  paper,
  scored,
  uuid,
} from "./fixtures";

const flags = vi.hoisted(() => ({
  notifications: false,
  messages: false,
  schedule: false,
  grades: false,
  learn: false,
}));
vi.mock("@/app/modules", () => ({ modules: flags }));

type Review = components["schemas"]["ReviewPolicy"];
type Card = components["schemas"]["StudentAssignmentCard"];

const AB = ["was wrote", "was written"];
let listReads = 0;
const LOCKS: Record<string, string | null> = {
  "true true true": null,
  "false true true": "Giáo viên chưa công bố điểm cho bài này.",
  "true false true": "Giáo viên không hiển thị đáp án đúng cho bài này.",
  "true true false": "Giáo viên không hiển thị giải thích cho bài này.",
  "false false true": "Giáo viên không hiển thị điểm và đáp án đúng cho bài này.",
  "false true false": "Giáo viên không hiển thị điểm và giải thích cho bài này.",
  "true false false": "Giáo viên không hiển thị đáp án đúng và giải thích cho bài này.",
  "false false false":
    "Giáo viên không hiển thị điểm, đáp án đúng và giải thích cho bài này.",
};

function passive(review: Review, transcript: boolean): AttemptResult {
  const wrong = choice(1, AB, [0], {
    prompt: "The letter ____ yesterday.",
    ...(review.showScore ? { earned: 0 } : {}),
    ...(review.showCorrectAnswers ? { correctOptionIds: [uuid("b", 11)] } : {}),
    ...(review.showExplanations
      ? { explanation: "Bị động thì quá khứ đơn dùng was/were + phân từ II." }
      : {}),
  });
  const listening = choice(3, ["Đi bộ"], [0], {
    prompt: "Người phụ nữ đề nghị làm gì?",
    media: {
      id: "018f0000-0000-7000-8000-00000000cc01",
      kind: "audio",
      url: "https://media.example/unit4.mp3",
      mimeType: "audio/mpeg",
      bytes: 159711,
      durationMs: 10004,
      originalFilename: "unit4.mp3",
      createdAt: "2026-08-20T00:00:00Z",
    },
    audioPlaysUsed: 2,
    ...(review.showScore ? { earned: 1 } : {}),
    ...(review.showCorrectAnswers ? { correctOptionIds: [uuid("b", 30)] } : {}),
    ...(transcript
      ? { transcript: "A: I'm struggling to keep up with the Tuesday class." }
      : {}),
  });
  const body = paper({ review, maxAttempts: 2, questions: [wrong, listening] });
  return review.showScore
    ? {
        ...body,
        attempt: { ...body.attempt, score: { earned: 1, total: 2, pendingManual: 0 } },
      }
    : body;
}

function serve(body: unknown, cards: Card[] | "fail" = [assignmentCard()]) {
  server.use(
    http.get(`${BASE}/app/attempts/${ATTEMPT_ID}/result`, () =>
      contractJson("/app/attempts/{id}/result", "get", 200, body),
    ),
    http.get(`${BASE}/app/assignments`, () => {
      listReads += 1;
      return cards === "fail"
        ? HttpResponse.json(
            {
              error: {
                code: "INTERNAL",
                message: "Lỗi máy chủ.",
                requestId: "018f0000-0000-7000-8000-00000000dd09",
              },
            },
            { status: 500 },
          )
        : contractJson("/app/assignments", "get", 200, {
            dueNow: [],
            upcoming: [],
            completed: cards,
          });
    }),
  );
}

function refuse(status: number, code: string, message: string) {
  server.use(
    http.get(`${BASE}/app/attempts/${ATTEMPT_ID}/result`, () =>
      HttpResponse.json(
        {
          error: { code, message, requestId: "018f0000-0000-7000-8000-00000000dd01" },
        },
        { status },
      ),
    ),
  );
}

function renderResult(
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } }),
) {
  const router = createMemoryRouter(
    [
      { path: "/app/attempts/:attemptId/result", element: <ResultPage /> },
      { path: "/app", element: <p>home page</p> },
    ],
    { initialEntries: [`/app/attempts/${ATTEMPT_ID}/result`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return client;
}

function ring() {
  return document.querySelector<HTMLElement>("[data-slot=score-ring]")!;
}

function arc() {
  return [
    ring().style.getPropertyValue("--arc"),
    ring().style.getPropertyValue("--turn"),
  ];
}

function item(prompt: string | RegExp) {
  return screen.getByText(prompt).closest("article")!;
}

function tiles() {
  const list = document.querySelector<HTMLElement>("[data-slot=result-tiles]");
  if (list === null) return [];
  return within(list)
    .getAllByRole("term")
    .map((term) => {
      const box = term.parentElement!;
      return [
        term.textContent,
        within(box).getAllByRole("definition")[0]!.firstElementChild!.textContent,
        box.querySelector<HTMLElement>("[data-slot=tile-bar]")!.style.width,
      ];
    });
}

beforeEach(() => {
  viewport("phone");
  flags.notifications = false;
  listReads = 0;
});
afterEach(async () => {
  vi.unstubAllGlobals();
  await i18n.changeLanguage("vi");
});

it("keeps shared material under a result filter and follows gaps to a hidden question", async () => {
  const body = passive(SCORE_ONLY, false);
  const group = {
    ...previewGroup,
    questionIds: body.questions.map((question) => question.id),
    stimuli: [
      {
        ...previewGroup.stimuli[0]!,
        gaps: [
          {
            kind: "question" as const,
            gapId: "material",
            questionId: body.questions[1]!.id,
          },
        ],
      },
    ],
  };
  serve({
    ...body,
    sharedContext: {
      groups: [group],
      transcripts: {},
      audioPlays: { [group.recordings[0]!.id]: 3 },
    },
  });
  renderResult();
  const user = userEvent.setup();
  await screen.findByText(group.title);
  expect(screen.queryByText("Nội dung bài nghe")).not.toBeInTheDocument();
  expect(
    screen.getByText("Đã nghe 3 lượt · Quy định 2 lượt cho cả nhóm"),
  ).toBeVisible();
  await user.click(screen.getByRole("button", { name: "Sai" }));
  expect(screen.getByRole("button", { name: "Sai" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  expect(screen.getByText(group.title)).toBeVisible();
  expect(screen.queryByText(body.questions[1]!.prompt)).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Ô A — chuyển đến câu 2" }));
  expect(screen.getByText(body.questions[1]!.prompt)).toBeVisible();
  expect(screen.getByRole("button", { name: "Tất cả" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  expect(document.activeElement).toHaveAttribute(
    "id",
    `result-question-${body.questions[1]!.id}`,
  );
});

it("shows a released group transcript while score, answer keys and explanations are hidden", async () => {
  const body = passive(CLOSED, false);
  const group = {
    ...previewGroup,
    questionIds: body.questions.map((question) => question.id),
  };
  serve({
    ...body,
    sharedContext: {
      groups: [group],
      transcripts: { [group.recordings[0]!.id]: "A released group transcript." },
      audioPlays: {},
    },
  });
  renderResult();
  await screen.findByText(group.title);
  await userEvent.setup().click(screen.getByText("Nội dung bài nghe"));
  expect(screen.getByText("A released group transcript.")).toBeVisible();
  expect(
    screen.getByText("Bạn đã trả lời 2/2 câu. Giáo viên chưa công bố điểm."),
  ).toBeVisible();
});

it("keeps shared material above the first of its questions that the filter keeps", async () => {
  const right = choice(1, AB, [1], { prompt: "Câu đầu của nhóm", earned: 1 });
  const wrong = choice(2, AB, [0], { prompt: "Câu sau của nhóm", earned: 0 });
  const alone = choice(3, AB, [0], { prompt: "Câu ngoài nhóm", earned: 0 });
  serve({
    ...scored([alone, right, wrong], { review: SCORE_ONLY }),
    sharedContext: {
      groups: [{ ...previewGroup, questionIds: [right.id, wrong.id] }],
      transcripts: {},
      audioPlays: {},
    },
  });
  renderResult();
  const places = () =>
    [
      ...document.querySelectorAll(
        "[data-slot=result-answers] [data-slot=card], [data-slot=result-answers] article",
      ),
    ].map((node) =>
      node.tagName === "ARTICLE"
        ? [...node.querySelectorAll("span")]
            .map((span) => span.textContent.trim())
            .find((text) => /^\d+\.$/.test(text))
        : "material",
    );
  await screen.findByText(previewGroup.title);
  expect(places()).toEqual(["1.", "material", "2.", "3."]);
  await userEvent.setup().click(screen.getByRole("button", { name: "Sai" }));
  expect(places()).toEqual(["1.", "material", "3."]);
  expect(screen.getByText("Ngữ liệu dùng chung · Câu 2–3")).toBeVisible();
});

it("keeps the group's tables and images on the paper surface", async () => {
  const body = passive(SCORE_ONLY, false);
  serve({
    ...body,
    sharedContext: {
      groups: [
        { ...previewGroup, questionIds: body.questions.map((question) => question.id) },
      ],
      transcripts: {},
      audioPlays: {},
    },
  });
  renderResult();
  const table = (await screen.findAllByRole("region", { name: "Bảng nội dung" }))[0]!;
  const surface = table.closest("[data-slot=result-answers]")!;
  expect(surface).toHaveClass(
    "[&_img]:bg-paper",
    "[&_.content-table-scroll]:bg-paper",
    "[&_.content-table-scroll]:text-paper-fg",
  );
  expect(surface.className).toMatch(
    /\[&_\.content-table-scroll_:is\(th,td\)\]:border-\S+!(\s|$)/,
  );
  expect(surface.className).toMatch(/\[&_\.content-table-scroll_th\]:bg-\S+!(\s|$)/);
  expect(table).toHaveClass("content-table-scroll");
  expect(surface.className).not.toMatch(/dark:/);
  expect(surface).toContainElement(item("The letter ____ yesterday."));
});

describe("the result page", () => {
  const combos: [boolean, boolean, boolean][] = [
    [false, false, false],
    [true, false, false],
    [false, true, false],
    [false, false, true],
    [true, true, false],
    [true, false, true],
    [false, true, true],
    [true, true, true],
  ];

  it.each(combos)(
    "showScore=%s showCorrectAnswers=%s showExplanations=%s renders exactly its blocks",
    async (showScore, showCorrectAnswers, showExplanations) => {
      serve(passive({ showScore, showCorrectAnswers, showExplanations }, true));
      renderResult();
      const wrong = (await screen.findByText("The letter ____ yesterday.")).closest(
        "article",
      )!;
      const right = item("Người phụ nữ đề nghị làm gì?");

      expect(ring()).toHaveAttribute("data-ring", showScore ? "score" : "withheld");
      expect(within(ring()).queryByText("trên 2") !== null).toBe(showScore);
      expect(within(ring()).queryByText("Điểm chưa được công bố") !== null).toBe(
        !showScore,
      );
      expect(
        await screen.findByText("IELTS Foundation A · nộp Thứ 4, 26/08 · lượt 1/2"),
      ).toBeInTheDocument();
      const graded = showCorrectAnswers
        ? "Bạn trả lời đúng 1/2 câu. Đáp án đúng hiển thị bên dưới."
        : "Bạn trả lời đúng 1/2 câu.";
      expect(
        screen.getByText(
          showScore ? graded : "Bạn đã trả lời 2/2 câu. Giáo viên chưa công bố điểm.",
        ),
      ).toBeInTheDocument();
      expect(tiles()).toEqual([]);

      expect(screen.queryByRole("button", { name: "Sai" }) !== null).toBe(showScore);
      expect(screen.queryByRole("group", { name: "Lọc câu hỏi" }) !== null).toBe(
        showScore,
      );
      expect(screen.queryByRole("button", { name: "Chờ chấm" })).toBeNull();

      expect(wrong).toHaveAttribute("data-verdict", showScore ? "wrong" : "unknown");
      expect(right).toHaveAttribute("data-verdict", showScore ? "correct" : "unknown");
      expect(within(wrong).queryByText("0 / 1") !== null).toBe(showScore);
      expect(within(right).queryByText("1 / 1") !== null).toBe(showScore);
      expect(within(wrong).queryByText("Sai") !== null).toBe(showScore);
      expect(within(right).queryByText("Đúng") !== null).toBe(showScore);
      const picked = within(wrong).getByText("was wrote").closest("[data-slot=given]")!;
      expect(picked.classList.contains("line-through")).toBe(showScore);
      expect(picked.classList.contains("text-danger-ink")).toBe(showScore);
      expect(right.querySelector("[data-slot=given]")).toHaveTextContent("Đi bộ");
      expect(right.querySelector("[data-slot=given]")).not.toHaveClass("line-through");

      expect(within(wrong).queryByText("Đáp án đúng") !== null).toBe(
        showCorrectAnswers,
      );
      expect(within(wrong).queryByText("was written") !== null).toBe(
        showCorrectAnswers,
      );
      expect(within(right).queryByText("Đáp án đúng") !== null).toBe(
        showCorrectAnswers && !showScore,
      );

      expect(screen.queryByText(/Bị động thì quá khứ đơn/) !== null).toBe(
        showExplanations,
      );

      const lock = LOCKS[`${showScore} ${showCorrectAnswers} ${showExplanations}`]!;
      for (const line of new Set(Object.values(LOCKS))) {
        if (line === null) continue;
        expect(screen.queryByText(line) !== null).toBe(line === lock);
      }

      expect(screen.getAllByText("Bạn trả lời")).toHaveLength(2);
      expect(screen.getByText("Xem lời thoại")).toBeInTheDocument();
    },
  );

  it("draws nothing from a field the policy removed, even when the server sends it", async () => {
    const leaked = choice(1, AB, [0], {
      prompt: "The letter ____ yesterday.",
      earned: 0,
      correctOptionIds: [uuid("b", 11)],
      explanation: "Bị động thì quá khứ đơn dùng was/were + phân từ II.",
    });
    serve(
      scored([leaked], {
        review: CLOSED,
      }),
    );
    renderResult();
    const article = (await screen.findByText("The letter ____ yesterday.")).closest(
      "article",
    )!;
    expect(article).toHaveAttribute("data-verdict", "unknown");
    expect(article.firstElementChild).toHaveClass("bg-muted", "text-muted-fg");
    expect(article.querySelector("svg.lucide-minus")).not.toBeNull();
    expect(screen.queryByText("0 / 1")).toBeNull();
    expect(screen.queryByText("Đáp án đúng")).toBeNull();
    expect(screen.queryByText("was written")).toBeNull();
    expect(screen.queryByText(/Bị động thì quá khứ đơn/)).toBeNull();
    expect(
      within(article).getByText("was wrote").closest("[data-slot=given]"),
    ).not.toHaveClass("line-through");
    expect(ring()).toHaveAttribute("data-ring", "withheld");
    expect(ring()).not.toHaveTextContent("0");
    expect(screen.queryByRole("button", { name: "Sai" })).toBeNull();
  });

  it("says the transcript is withheld in the transcript's own slot", async () => {
    serve(passive(OPEN, false));
    renderResult();
    await screen.findByText("Người phụ nữ đề nghị làm gì?");
    expect(screen.getByText("Lời thoại không được hiển thị.")).toBeInTheDocument();
    expect(screen.queryByText("Xem lời thoại")).not.toBeInTheDocument();
  });

  it("shows the grader's comment on its answer under any policy", async () => {
    serve(
      paper({
        review: CLOSED,
        questions: [
          essay(1, "Viết một câu bị động.", "The letter was written.", {
            pendingManual: false,
            graderComment: "Đúng cấu trúc.\nNhớ thêm trạng từ.",
          }),
          essay(2, "Viết thêm một câu.", "Another one.", {
            pendingManual: false,
            graderComment: "  ",
          }),
        ],
      }),
    );
    renderResult();
    const article = (await screen.findByText("Viết một câu bị động.")).closest(
      "article",
    )!;
    expect(within(article).getByText("Nhận xét của giáo viên")).toBeVisible();
    expect(within(article).getByText(/Đúng cấu trúc\./)).toHaveClass(
      "whitespace-pre-wrap",
    );
    expect(screen.getAllByText("Nhận xét của giáo viên")).toHaveLength(1);
  });

  it("draws each mark's own circle", async () => {
    serve(
      scored([
        choice(1, AB, [1], { prompt: "Câu đúng", earned: 1 }),
        choice(2, AB, [0], { prompt: "Câu sai", earned: 0 }),
        choice(3, AB, [0], { prompt: "Câu đúng một nửa", points: 2, earned: 1 }),
        essay(4, "Câu đang chờ", "Một câu."),
      ]),
    );
    renderResult();
    await screen.findByText("Câu đúng");
    const circle = (prompt: string) => {
      const mark = item(prompt).firstElementChild!;
      return [
        /lucide-[a-z-]+/.exec(mark.querySelector("svg")!.getAttribute("class")!)![0],
        ["success", "brand", "danger", "warning"].find((tone) =>
          mark.classList.contains(`bg-${tone}-soft`),
        ),
      ];
    };
    expect(circle("Câu đúng")).toEqual(["lucide-check", "success"]);
    expect(circle("Câu sai")).toEqual(["lucide-x", "danger"]);
    expect(circle("Câu đúng một nửa")).toEqual(["lucide-percent", "brand"]);
    expect(circle("Câu đang chờ")).toEqual(["lucide-hourglass", "warning"]);
  });

  it("writes the number and the prompt as one run, so a second line starts under the number", async () => {
    serve(scored([choice(1, AB, [1], { prompt: "Câu đúng", earned: 1 })]));
    renderResult();
    const prompt = await screen.findByText("Câu đúng");
    const number = within(item("Câu đúng")).getByText("1.");
    expect(number).toHaveClass("float-left", "whitespace-pre");
    expect(number.textContent).toBe("1. ");
    expect(number).not.toHaveClass("mr-1");
    expect(number.parentElement).not.toHaveClass("flex");
    expect(number.parentElement).toContainElement(prompt);
  });

  it("lets a long word break inside the answer's box, whichever line it is on", async () => {
    serve(
      scored([
        choice(1, AB, [0], {
          prompt: "Câu sai",
          earned: 0,
          explanation: "https://example.com/a-very-long-address-with-no-space-in-it",
        }),
      ]),
    );
    renderResult();
    const prompt = await screen.findByText("Câu sai");
    const column = item("Câu sai").lastElementChild!;
    expect(column).toHaveClass("min-w-0", "break-words");
    expect(column).toContainElement(prompt);
    expect(column).toContainElement(screen.getByText(/a-very-long-address/));
  });

  it("offers to read again when a question's recording has expired", async () => {
    let reads = 0;
    const body = passive(OPEN, true);
    server.use(
      http.get(`${BASE}/app/attempts/${ATTEMPT_ID}/result`, () => {
        reads += 1;
        return contractJson("/app/attempts/{id}/result", "get", 200, body);
      }),
      http.get(`${BASE}/app/assignments`, () =>
        contractJson("/app/assignments", "get", 200, {
          dueNow: [],
          upcoming: [],
          completed: [],
        }),
      ),
    );
    renderResult();
    const article = (await screen.findByText("Người phụ nữ đề nghị làm gì?")).closest(
      "article",
    )!;
    fireEvent.error(article.querySelector("audio")!);
    await userEvent
      .setup()
      .click(await within(article).findByRole("button", { name: "Thử lại" }));
    await waitFor(() => expect(reads).toBe(2));
  });
});

describe("the deck's three variants", () => {
  beforeEach(() => viewport("desktop"));

  it("fully marked: the score, the sentence, a tile per part and every answer", async () => {
    serve(grammarCheck(), [assignmentCard({ testTitle: "Unit 3 · Grammar check" })]);
    renderResult();
    await screen.findByRole("heading", { level: 1, name: "Unit 3 · Grammar check" });
    expect(ring()).toHaveAttribute("data-ring", "score");
    expect(arc()).toEqual(["var(--success)", "288deg"]);
    expect(within(ring()).getByText("8")).toBeVisible();
    expect(within(ring()).getByText("trên 10")).toBeVisible();
    expect(
      screen.getByText("Bạn trả lời đúng 8/10 câu. Đáp án đúng hiển thị bên dưới."),
    ).toBeVisible();
    expect(
      await screen.findByText("IELTS Foundation A · nộp Thứ 6, 19/09"),
    ).toBeVisible();
    expect(tiles()).toEqual([
      ["Present perfect", "4 / 5", "80%"],
      ["Past simple", "4 / 5", "80%"],
    ]);
    expect(document.querySelectorAll("[data-slot=tile-bar].bg-success")).toHaveLength(
      2,
    );
    expect(screen.queryByText(/Giáo viên không hiển thị/)).toBeNull();
    expect(screen.getAllByRole("article")).toHaveLength(10);

    const right = item(/^She\s+in Hanoi since 2019\.$/);
    expect(right).toHaveAttribute("data-verdict", "correct");
    expect(within(right).getByText("1.")).toBeVisible();
    expect(within(right).getByText("1 / 1")).toBeVisible();
    expect(within(right).getByText("has lived")).not.toHaveClass("line-through");
    expect(within(right).queryByText("Đáp án đúng")).toBeNull();
    expect(within(right).getByRole("img", { name: "chỗ trống" })).toHaveTextContent(
      "___",
    );

    const wrong = item(/^I\s+him yesterday at the station\.$/);
    expect(wrong).toHaveAttribute("data-verdict", "wrong");
    expect(within(wrong).getByText("0 / 1")).toBeVisible();
    expect(within(wrong).getByText("have seen")).toHaveClass(
      "line-through",
      "text-danger-ink",
    );
    expect(within(wrong).getByText("saw")).toHaveClass("text-success-ink");
    expect(
      within(wrong).getByText("Yesterday is a finished time, so use the past simple."),
    ).toBeVisible();
  });

  it("just submitted: the points so far, three tiles and the answers that wait", async () => {
    const submittedAt = new Date(Date.now() - 20_000).toISOString();
    serve(justSubmitted(submittedAt), [
      assignmentCard({ className: "IELTS 6.5 Evening" }),
    ]);
    renderResult();
    await screen.findByRole("heading", {
      level: 1,
      name: "Reading · Why cities need green space",
    });
    expect(ring()).toHaveAttribute("data-ring", "soFar");
    expect(arc()).toEqual(["var(--accent-c)", "180deg"]);
    expect(within(ring()).getByText("3")).toBeVisible();
    expect(within(ring()).getByText("trên 6, tạm tính")).toBeVisible();
    expect(
      screen.getByText("6 câu đã được chấm tự động. Giáo viên sẽ chấm 2 câu tự luận."),
    ).toBeVisible();
    expect(await screen.findByText("IELTS 6.5 Evening · vừa nộp")).toBeVisible();
    expect(tiles()).toEqual([
      ["Chấm tự động", "3 / 6", "50%"],
      ["Chờ giáo viên chấm", "2 câu", "0%"],
      ["Thời gian làm bài", "7 phút", "16%"],
    ]);
    expect(
      [...document.querySelectorAll("[data-slot=tile-bar]")].map((bar) =>
        ["bg-brand", "bg-warning", "bg-info"].find((tone) =>
          bar.classList.contains(tone),
        ),
      ),
    ).toEqual(["bg-brand", "bg-warning", "bg-info"]);
    expect(
      screen.getByText(
        "Giáo viên không hiển thị đáp án đúng và giải thích cho bài này.",
      ),
    ).toBeVisible();

    const waiting = item("What should cities prioritise when they plan new districts?");
    expect(waiting).toHaveAttribute("data-verdict", "waiting");
    expect(within(waiting).getByText("Chờ chấm")).toBeVisible();
    const written = waiting.querySelector("[data-slot=given]");
    expect(written).toHaveTextContent("public green space");
    expect(written).not.toHaveClass("line-through");
    const skipped = item("What rule does Copenhagen follow?");
    expect(skipped).toHaveAttribute("data-verdict", "wrong");
    expect(within(skipped).getByText("Chưa trả lời")).not.toHaveClass("line-through");
    expect(within(skipped).getByText("0 / 1")).toBeVisible();

    await userEvent.setup().click(screen.getByRole("button", { name: "Chờ chấm" }));
    expect(screen.getAllByRole("article")).toHaveLength(2);
    expect(screen.queryByText("What rule does Copenhagen follow?")).toBeNull();
  });

  it("pending: nothing marked yet, so no number and no tiles", async () => {
    serve(grading(), [assignmentCard({ className: "IELTS 6.5 Evening" })]);
    renderResult();
    await screen.findByRole("heading", { level: 1, name: "Reading · TFNG Practice A" });
    expect(ring()).toHaveAttribute("data-ring", "grading");
    expect(arc()).toEqual(["var(--warning)", "0deg"]);
    expect(within(ring()).getByText("đang chấm")).toBeVisible();
    expect(within(ring()).getByText("…")).toHaveAttribute("aria-hidden", "true");
    expect(
      screen.getByText(
        "Giáo viên đang chấm các câu tự luận của bạn. Điểm sẽ hiện ở đây khi chấm xong.",
      ),
    ).toBeVisible();
    expect(tiles()).toEqual([]);
    expect(document.querySelector("[data-slot=result-tiles]")).toBeNull();
    expect(screen.getAllByRole("article")).toHaveLength(2);
    expect(screen.getAllByText("Chờ chấm")).toHaveLength(3);
  });

  it("promises a notification only once notifications exist", async () => {
    flags.notifications = true;
    serve(grading());
    renderResult();
    expect(
      await screen.findByText(
        "Giáo viên đang chấm các câu tự luận của bạn. Bạn sẽ nhận được thông báo khi có điểm.",
      ),
    ).toBeVisible();
  });

  it("reads as the deck writes it in English", async () => {
    await i18n.changeLanguage("en");
    serve(grammarCheck());
    renderResult();
    await screen.findByRole("heading", { level: 1, name: "Unit 3 · Grammar check" });
    expect(within(ring()).getByText("of 10")).toBeVisible();
    expect(
      screen.getByText(
        "You answered 8 of 10 correctly. Correct answers are shown below.",
      ),
    ).toBeVisible();
    expect(
      await screen.findByText("IELTS Foundation A · submitted Fri 19 Sep"),
    ).toBeVisible();
    expect(
      screen.getByRole("heading", { level: 2, name: "Your answers" }),
    ).toBeVisible();
    expect(
      within(screen.getByRole("group", { name: "Filter questions" }))
        .getAllByRole("button")
        .map((button) => button.textContent),
    ).toEqual(["All", "Wrong"]);
    const wrong = item(/^I\s+him yesterday at the station\.$/);
    expect(wrong).toHaveTextContent("You answered have seen");
    expect(wrong).toHaveTextContent("Correct answer saw");
    expect(screen.getByRole("link", { name: "Home" })).toHaveAttribute("href", "/app");
  });

  it("writes the just-submitted variant in the deck's English", async () => {
    await i18n.changeLanguage("en");
    serve(justSubmitted(new Date(Date.now() - 20_000).toISOString()), [
      assignmentCard({ className: "IELTS 6.5 Evening" }),
    ]);
    renderResult();
    expect(
      await screen.findByText(
        "6 questions were marked automatically. Your teacher will grade the 2 written answers.",
      ),
    ).toBeVisible();
    expect(within(ring()).getByText("of 6 so far")).toBeVisible();
    expect(
      await screen.findByText("IELTS 6.5 Evening · submitted just now"),
    ).toBeVisible();
    expect(tiles().map(([label, value]) => [label, value])).toEqual([
      ["Marked automatically", "3 / 6"],
      ["Waiting for teacher", "2 answers"],
      ["Time used", "7 min"],
    ]);
    expect(
      within(screen.getByRole("group", { name: "Filter questions" }))
        .getAllByRole("button")
        .map((button) => button.textContent),
    ).toEqual(["All", "Wrong", "Waiting"]);
    expect(screen.getAllByText("No answer")).toHaveLength(3);
    expect(screen.getAllByText("Waiting")).toHaveLength(3);
  });

  describe("in English", () => {
    it.each([
      [false, true, true, "Your teacher has not released the score for this test."],
      [true, false, true, "Your teacher is not showing correct answers for this test."],
      [true, true, false, "Your teacher is not showing explanations for this test."],
      [
        false,
        false,
        true,
        "Your teacher is not showing the score or correct answers for this test.",
      ],
      [
        false,
        true,
        false,
        "Your teacher is not showing the score or explanations for this test.",
      ],
      [
        true,
        false,
        false,
        "Your teacher is not showing correct answers or explanations for this test.",
      ],
      [
        false,
        false,
        false,
        "Your teacher is not showing the score, correct answers or explanations for this test.",
      ],
    ])(
      "names what showScore=%s showCorrectAnswers=%s showExplanations=%s hides",
      async (showScore, showCorrectAnswers, showExplanations, line) => {
        await i18n.changeLanguage("en");
        serve(passive({ showScore, showCorrectAnswers, showExplanations }, false));
        renderResult();
        expect(await screen.findByText(line)).toBeVisible();
      },
    );

    it.each([
      [
        false,
        "Your teacher is grading your written answers. Your score appears here when grading is finished.",
      ],
      [
        true,
        "Your teacher is grading your written answers. You will get a notification when your score is ready.",
      ],
    ])("writes the pending sentence with notifications=%s", async (on, line) => {
      await i18n.changeLanguage("en");
      flags.notifications = on;
      serve(grading());
      renderResult();
      expect(await screen.findByText(line)).toBeVisible();
      expect(within(ring()).getByText("grading")).toBeVisible();
    });

    it("writes the withheld sentence", async () => {
      await i18n.changeLanguage("en");
      serve(passive(CLOSED, false));
      renderResult();
      expect(
        await screen.findByText(
          "You answered 2 of 2 questions. Your teacher has not released the score.",
        ),
      ).toBeVisible();
      expect(within(ring()).getByText("Score not released")).toBeInTheDocument();
    });

    it("writes one marked answer and one waiting answer in the singular", async () => {
      await i18n.changeLanguage("en");
      serve(
        scored(
          [
            choice(1, AB, [0], { earned: 1 }),
            essay(2, "Viết một câu bị động.", "The letter was written."),
          ],
          { review: SCORE_ONLY },
        ),
      );
      renderResult();
      expect(
        await screen.findByText(
          "1 question was marked automatically. Your teacher will grade the 1 written answer.",
        ),
      ).toBeVisible();
      expect(tiles()[1]!.slice(0, 2)).toEqual(["Waiting for teacher", "1 answer"]);
    });
  });
});

describe("the ring's colour", () => {
  it.each([
    [8, "var(--success)", "288deg"],
    [7, "var(--accent-c)", "252deg"],
    [6, "var(--accent-c)", "216deg"],
    [5, "var(--warning)", "180deg"],
    [0, "var(--warning)", "0deg"],
  ])("%s of 10 draws %s over %s", async (earned, colour, turn) => {
    serve(scored([choice(1, AB, [0], { points: 10, earned })]));
    renderResult();
    await screen.findByText("Câu hỏi 1");
    expect(arc()).toEqual([colour, turn]);
  });
});

describe("the tiles' columns", () => {
  const parts = (count: number) =>
    scored(
      Array.from({ length: count }, (_, i) =>
        choice(i + 1, AB, [0], { earned: 1, sectionId: uuid("c", i + 1) }),
      ),
      {
        sections: Array.from({ length: count }, (_, i) => ({
          id: uuid("c", i + 1),
          title: `Phần ${i + 1}`,
          instructions: null,
        })),
      },
    );
  const columns = () =>
    document.querySelector<HTMLElement>("[data-slot=result-tiles]")!.style
      .gridTemplateColumns;

  it("are two below 768, whatever the number of tiles", async () => {
    serve(parts(5));
    renderResult();
    await screen.findByText("Phần 5");
    expect(columns()).toBe("repeat(2, minmax(0, 1fr))");
  });

  it.each([
    [2, 2],
    [3, 3],
    [4, 4],
    [5, 4],
  ])("are %s for %s parts from 768, never more than four", async (count, expected) => {
    viewport("desktop");
    serve(parts(count));
    renderResult();
    await screen.findByText(`Phần ${count}`);
    expect(columns()).toBe(`repeat(${expected}, minmax(0, 1fr))`);
    expect(tiles()).toHaveLength(count);
  });

  it("follow the width when it crosses 768", async () => {
    const screenWidth = viewport("phone");
    serve(parts(3));
    renderResult();
    await screen.findByText("Phần 3");
    expect(columns()).toBe("repeat(2, minmax(0, 1fr))");
    expect(screen.queryByRole("link", { name: "Trang chủ" })).toBeNull();
    screenWidth.resize("desktop");
    await waitFor(() => expect(columns()).toBe("repeat(3, minmax(0, 1fr))"));
    expect(screen.getByRole("link", { name: "Trang chủ" })).toHaveAttribute(
      "href",
      "/app",
    );
  });
});

describe("what counts as answered", () => {
  it("does not count a cleared answer, and says so on its item", async () => {
    serve(
      paper({
        review: CLOSED,
        questions: [
          choice(1, AB, [1], { prompt: "Câu đã chọn" }),
          choice(2, AB, [], { prompt: "Câu đã bỏ chọn" }),
          essay(3, "Câu đã xoá chữ", "   ", { pendingManual: false }),
          choice(4, AB, null, { prompt: "Câu chưa mở" }),
        ],
      }),
    );
    renderResult();
    expect(
      await screen.findByText("Bạn đã trả lời 1/4 câu. Giáo viên chưa công bố điểm."),
    ).toBeVisible();
    expect(within(item("Câu đã chọn")).getByText("was written")).toBeVisible();
    for (const prompt of ["Câu đã bỏ chọn", "Câu đã xoá chữ", "Câu chưa mở"])
      expect(within(item(prompt)).getByText("Chưa trả lời")).toBeVisible();
  });

  it("lists what was typed into a fill-in that is only partly filled", async () => {
    const first = uuid("e", 1);
    const second = uuid("e", 2);
    serve(
      scored([
        blank(1, "I {{1}} him, then {{2}} home.", null, {
          points: 2,
          earned: 1,
          blanks: [
            { id: first, ordinal: 1, caseSensitive: false },
            { id: second, ordinal: 2, caseSensitive: false },
          ],
          answer: { type: "fill_blank", values: { [first]: "saw", [second]: " " } },
          correctAnswers: [
            { blankId: first, answer: "saw" },
            { blankId: second, answer: "went" },
          ],
        }),
      ]),
    );
    renderResult();
    const article = (await screen.findByText(/him, then/)).closest("article")!;
    expect(article).toHaveAttribute("data-verdict", "partial");
    expect(within(article).getByText("Đúng một phần")).toBeInTheDocument();
    expect(within(article).getByText("1 / 2")).toBeVisible();
    expect(within(article).getByText("saw · —")).not.toHaveClass("line-through");
    expect(within(article).getByText("saw · went")).toHaveClass("text-success-ink");
    expect(within(article).getAllByRole("img", { name: "chỗ trống" })).toHaveLength(2);
  });

  it("writes a true/false value in words and keeps the lines of a written answer", async () => {
    serve(
      paper({
        review: CLOSED,
        questions: [
          choice(1, ["Đúng", "Sai"], null, {
            type: "true_false",
            prompt: "Trái đất phẳng.",
            answer: { type: "true_false", value: false },
          }),
          essay(2, "Viết hai dòng.", "Dòng một\nDòng hai", { pendingManual: false }),
        ],
      }),
    );
    renderResult();
    const flat = (await screen.findByText("Trái đất phẳng.")).closest("article")!;
    expect(within(flat).getByText("Sai")).toBeVisible();
    expect(within(item("Viết hai dòng.")).getByText(/Dòng một/)).toHaveClass(
      "whitespace-pre-wrap",
    );
  });

  it("separates the options picked with commas and writes decimals the Vietnamese way", async () => {
    serve(
      scored([
        choice(1, ["một", "hai", "ba"], [2, 0], {
          type: "multiple_choice",
          prompt: "Chọn hai",
          points: 1.5,
          earned: 0.5,
        }),
      ]),
    );
    renderResult();
    const article = (await screen.findByText("Chọn hai")).closest("article")!;
    expect(article.querySelector("[data-slot=given]")).toHaveTextContent("một, ba");
    expect(within(article).getByText("0,5 / 1,5")).toBeVisible();
  });

  it("names the answers section after its heading", async () => {
    serve(scored([choice(1, AB, [0], { earned: 1 })]));
    renderResult();
    await screen.findByText("Câu hỏi 1");
    expect(screen.getByRole("region", { name: "Câu trả lời của bạn" })).toHaveAttribute(
      "data-slot",
      "result-answers",
    );
  });

  it("has no empty-filter note on a paper with no questions", async () => {
    serve(scored([]));
    renderResult();
    await screen.findByRole("heading", { level: 1 });
    expect(screen.queryByText("Không có câu sai trong bài này.")).toBeNull();
  });

  it("turns a part's bar amber and writes a true answer in words", async () => {
    serve(
      scored(
        [
          choice(1, ["Đúng", "Sai"], null, {
            type: "true_false",
            prompt: "Nước sôi ở 100 độ.",
            earned: 0,
            answer: { type: "true_false", value: true },
          }),
          choice(2, AB, [1], { earned: 1, sectionId: PART_TWO }),
        ],
        {
          sections: [
            { id: PART_ONE, title: "Nghe", instructions: null },
            { id: PART_TWO, title: "Đọc", instructions: null },
          ],
        },
      ),
    );
    renderResult();
    const article = (await screen.findByText("Nước sôi ở 100 độ.")).closest("article")!;
    expect(article.querySelector("[data-slot=given]")).toHaveTextContent("Đúng");
    expect(
      [...document.querySelectorAll("[data-slot=tile-bar]")].map((bar) =>
        ["bg-success", "bg-warning"].find((tone) => bar.classList.contains(tone)),
      ),
    ).toEqual(["bg-warning", "bg-success"]);
  });
});

describe("content written in the rich editor", () => {
  const written = (text: string) => ({
    format: "semantic_v1" as const,
    blocks: [
      {
        type: "paragraph" as const,
        content: [{ type: "text" as const, text, marks: [] }],
      },
    ],
  });

  it("draws a rich fill-in's gap as a blank", async () => {
    serve(
      scored([
        blank(1, "Điền [1]", "went", {
          earned: 1,
          promptContent: {
            format: "semantic_v1",
            blocks: [
              {
                type: "paragraph",
                content: [
                  { type: "text", text: "Điền ", marks: [] },
                  { type: "gap", id: "a", label: "1" },
                ],
              },
            ],
          },
          blanks: [{ id: uuid("e", 1), gapId: "a", ordinal: 1, caseSensitive: false }],
        }),
      ]),
    );
    renderResult();
    const article = (await screen.findByText(/Điền/)).closest("article")!;
    expect(within(article).getByRole("img", { name: "chỗ trống" })).toHaveTextContent(
      "___",
    );
    expect(article).not.toHaveTextContent("[1]");
  });

  it("draws a rich prompt and explanation as written, never as Markdown", async () => {
    serve(
      scored([
        choice(1, AB, [0], {
          prompt: "Chọn *một* đáp án.",
          promptContent: written("Chọn *một* đáp án."),
          earned: 0,
          explanation: "Dùng _was_ với phân từ II.",
          explanationContent: written("Dùng _was_ với phân từ II."),
        }),
      ]),
    );
    renderResult();
    const article = (await screen.findByText("Chọn *một* đáp án.")).closest("article")!;
    expect(within(article).getByText("Dùng _was_ với phân từ II.")).toBeVisible();
  });
});

describe("the line above the title", () => {
  const body = () => scored([choice(1, AB, [0], { earned: 1 })]);

  it("reads the class from the lists the shell already holds, and asks for nothing", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    client.setQueryData(["my-assignments"], {
      dueNow: [assignmentCard({ className: "Lớp trong bộ nhớ" })],
      upcoming: [],
      completed: [],
    });
    serve(body());
    renderResult(client);
    expect(
      await screen.findByText("Lớp trong bộ nhớ · nộp Thứ 4, 26/08"),
    ).toBeVisible();
    await expect(
      screen.findByText(/IELTS Foundation A/, {}, { timeout: 300 }),
    ).rejects.toThrow();
    expect(listReads).toBe(0);
  });

  it("reads the lists itself when nothing holds them", async () => {
    serve(body());
    renderResult();
    expect(
      await screen.findByText("IELTS Foundation A · nộp Thứ 4, 26/08"),
    ).toBeVisible();
    expect(listReads).toBe(1);
  });

  it("has only the day when the paper's class is not known", async () => {
    serve(body(), []);
    renderResult();
    expect(await screen.findByText("Nộp Thứ 4, 26/08")).toBeVisible();
  });

  it("writes the day in the app's time zone", async () => {
    serve(
      scored([choice(1, AB, [0], { earned: 1 })], {
        attempt: { ...paper().attempt, submittedAt: "2026-08-26T18:30:00Z" },
      }),
      [],
    );
    renderResult();
    expect(await screen.findByText("Nộp Thứ 5, 27/08")).toBeVisible();
  });

  it("still shows the result when the assignment lists cannot be read", async () => {
    serve(body(), "fail");
    renderResult();
    expect(await screen.findByText("Nộp Thứ 4, 26/08")).toBeVisible();
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Unit 4");
  });

  it("names the attempt only when the paper allows more than one", async () => {
    serve({ ...body(), maxAttempts: 3 });
    renderResult();
    expect(
      await screen.findByText("IELTS Foundation A · nộp Thứ 4, 26/08 · lượt 1/3"),
    ).toBeVisible();
  });

  it("says just now for a minute, with or without the class", async () => {
    const fresh = scored([choice(1, AB, [0], { earned: 1 })], {
      attempt: {
        ...paper().attempt,
        submittedAt: new Date(Date.now() - 5_000).toISOString(),
      },
    });
    serve(fresh, []);
    renderResult();
    expect(await screen.findByText("Vừa nộp")).toBeVisible();
  });

  it("has the class alone for an attempt with no submission time", async () => {
    serve(
      scored([choice(1, AB, [0], { earned: 1 })], {
        attempt: { ...paper().attempt, status: "timed_out", submittedAt: null },
      }),
    );
    renderResult();
    expect(await screen.findByText("IELTS Foundation A")).toBeVisible();
    expect(screen.queryByText(/nộp/i)).toBeNull();
  });
});

describe("the minute after submitting", () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.setSystemTime(new Date("2026-08-26T13:14:05Z"));
  });
  afterEach(() => vi.useRealTimers());

  it("repaints the line above the title alone, then writes the day", async () => {
    serve(scored([blank(1, "She {{1}} in Hanoi.", "lives", { earned: 1 })]));
    renderResult();
    expect(await screen.findByText("IELTS Foundation A · vừa nộp")).toBeVisible();
    const gap = screen.getByRole("img", { name: "chỗ trống" });
    await act(() => vi.advanceTimersByTimeAsync(3_000));
    expect(gap).toBeInTheDocument();
    expect(screen.getByText("IELTS Foundation A · vừa nộp")).toBeVisible();
    await act(() => vi.advanceTimersByTimeAsync(60_000));
    expect(screen.getByText("IELTS Foundation A · nộp Thứ 4, 26/08")).toBeVisible();
    expect(gap).toBeInTheDocument();
  });
});

describe("the filter", () => {
  it("keeps a partly right answer under Wrong and leaves the right ones out", async () => {
    serve(
      scored([
        choice(1, AB, [1], { prompt: "Câu đúng", earned: 1 }),
        choice(2, AB, [0], { prompt: "Câu sai", earned: 0 }),
        choice(3, AB, [0], { prompt: "Câu đúng một nửa", points: 2, earned: 1 }),
      ]),
    );
    renderResult();
    const user = userEvent.setup();
    await user.click(await screen.findByRole("button", { name: "Sai" }));
    expect(screen.getAllByRole("article").map((a) => a.dataset["verdict"])).toEqual([
      "wrong",
      "partial",
    ]);
    expect(screen.queryByText("Câu đúng")).toBeNull();
    expect(within(item("Câu đúng một nửa")).getByText("3.")).toBeVisible();
  });

  it("explains an empty Wrong filter and lets the student return to all questions", async () => {
    const user = userEvent.setup();
    serve(
      scored([choice(1, AB, [1], { prompt: "The letter ____ yesterday.", earned: 1 })]),
    );
    renderResult();
    await user.click(await screen.findByRole("button", { name: "Sai" }));
    expect(screen.getByText("Không có câu sai trong bài này.")).toBeInTheDocument();
    expect(screen.queryByText("The letter ____ yesterday.")).toBeNull();
    await user.click(screen.getByRole("button", { name: "Xem tất cả câu" }));
    expect(screen.getByText("The letter ____ yesterday.")).toBeInTheDocument();
    expect(screen.queryByText("Không có câu sai trong bài này.")).toBeNull();
  });

  it("offers Waiting without Wrong when the score is hidden and an answer waits", async () => {
    serve(
      paper({
        review: CLOSED,
        questions: [
          choice(1, AB, [1], { prompt: "Câu trắc nghiệm" }),
          essay(2, "Câu tự luận", "Một câu."),
        ],
      }),
    );
    renderResult();
    await screen.findByText("Câu tự luận");
    expect(
      within(screen.getByRole("group", { name: "Lọc câu hỏi" }))
        .getAllByRole("button")
        .map((button) => button.textContent),
    ).toEqual(["Tất cả", "Chờ chấm"]);
    await userEvent.setup().click(screen.getByRole("button", { name: "Chờ chấm" }));
    expect(screen.queryByText("Câu trắc nghiệm")).toBeNull();
    expect(item("Câu tự luận")).toHaveAttribute("data-verdict", "waiting");
  });
  it("returns to All when the choice it was on is no longer offered", async () => {
    const waiting = [
      choice(1, AB, [1], { prompt: "Câu trắc nghiệm", earned: 1 }),
      essay(2, "Câu tự luận", "Một câu."),
    ];
    serve(scored(waiting));
    const client = renderResult();
    await userEvent
      .setup()
      .click(await screen.findByRole("button", { name: "Chờ chấm" }));
    expect(screen.queryByText("Câu trắc nghiệm")).toBeNull();
    serve(
      scored([
        waiting[0]!,
        essay(2, "Câu tự luận", "Một câu.", { pendingManual: false, earned: 1 }),
      ]),
    );
    await client.refetchQueries({ queryKey: ["attempt-result"] });
    expect(await screen.findByText("Câu trắc nghiệm")).toBeVisible();
    expect(screen.queryByRole("button", { name: "Chờ chấm" })).toBeNull();
    expect(screen.getByRole("button", { name: "Tất cả" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });
});

describe("before and instead of a result", () => {
  it("shows the page's shape while it loads", async () => {
    server.use(
      http.get(`${BASE}/app/attempts/${ATTEMPT_ID}/result`, async () => {
        await delay("infinite");
        return new HttpResponse(null, { status: 204 });
      }),
    );
    renderResult();
    expect(await screen.findByRole("status", { name: "Đang tải…" })).toBeVisible();
    expect(screen.queryByRole("heading", { level: 1 })).toBeNull();
  });

  it("shows the not-ready card on a 409 with one way home", async () => {
    refuse(409, "ATTEMPT_IN_PROGRESS", "Bài chưa được nộp.");
    renderResult();
    expect(await screen.findByText("Kết quả chưa sẵn sàng.")).toBeInTheDocument();
    expect(screen.getByText("Bài chưa được nộp.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Về trang chủ" })).toHaveAttribute(
      "href",
      "/app",
    );
    expect(screen.queryByRole("button", { name: "Thử lại" })).toBeNull();
  });

  it.each([
    [403, "FORBIDDEN"],
    [404, "NOT_FOUND"],
  ])(
    "says a result that is not the student's was not found (%s)",
    async (status, code) => {
      refuse(status, code, "Không có quyền.");
      renderResult();
      expect(
        await screen.findByText(
          "Không tìm thấy kết quả này, hoặc đây không phải bài của bạn.",
        ),
      ).toBeVisible();
      expect(screen.queryByText("Không có quyền.")).toBeNull();
      expect(screen.getByRole("link", { name: "Về trang chủ" })).toHaveAttribute(
        "href",
        "/app",
      );
      expect(screen.queryByRole("button", { name: "Thử lại" })).toBeNull();
    },
  );

  it("says the paper is safe on any other failure, and reads again on retry", async () => {
    refuse(500, "INTERNAL", "Lỗi máy chủ.");
    renderResult();
    const alert = await screen.findByRole("alert");
    expect(within(alert).getByText("Không tải được kết quả")).toBeVisible();
    expect(
      within(alert).getByText(
        "Bài làm của bạn đã nộp và vẫn được lưu. Kiểm tra kết nối rồi thử lại.",
      ),
    ).toBeVisible();
    expect(
      within(alert).getByText("018f0000-0000-7000-8000-00000000dd01"),
    ).toBeVisible();
    serve(scored([choice(1, AB, [0], { earned: 1 })]));
    await userEvent.setup().click(screen.getByRole("button", { name: "Thử lại" }));
    expect(await screen.findByRole("heading", { level: 1 })).toHaveTextContent(
      "Unit 4",
    );
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("lets the request id drop under its label and Copy, and keeps the label whole", async () => {
    refuse(500, "INTERNAL", "Lỗi máy chủ.");
    renderResult();
    const alert = await screen.findByRole("alert");
    const id = within(alert).getByText("018f0000-0000-7000-8000-00000000dd01");
    expect(id).toHaveClass(
      "order-last",
      "basis-full",
      "min-w-0",
      "break-all",
      "@md/load-error:order-none",
      "@md/load-error:basis-auto",
    );
    expect(id.parentElement).toHaveClass("flex-wrap", "min-w-0", "max-w-full");
    expect(id.parentElement!.parentElement).toHaveClass("@container/load-error");
    expect(within(alert).getByText("Mã lỗi")).toHaveClass(
      "whitespace-nowrap",
      "shrink-0",
    );
    expect(within(alert).getByRole("button", { name: "Sao chép" })).toHaveClass(
      "shrink-0",
    );
    expect([...id.parentElement!.children].map((child) => child.textContent)).toEqual([
      "Mã lỗi",
      "018f0000-0000-7000-8000-00000000dd01",
      "Sao chép",
    ]);
  });

  it("reads again by itself when the network drops, twice at most", async () => {
    const body = scored([choice(1, AB, [0], { earned: 1 })]);
    let reads = 0;
    serve(body);
    server.use(
      http.get(`${BASE}/app/attempts/${ATTEMPT_ID}/result`, () => {
        reads += 1;
        return reads < 3
          ? HttpResponse.error()
          : contractJson("/app/attempts/{id}/result", "get", 200, body);
      }),
    );
    renderResult(new QueryClient({ defaultOptions: { queries: { retryDelay: 0 } } }));
    expect(await screen.findByRole("heading", { level: 1 })).toHaveTextContent(
      "Unit 4",
    );
    expect(reads).toBe(3);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("stops after the second retry and says the paper is safe", async () => {
    let reads = 0;
    server.use(
      http.get(`${BASE}/app/attempts/${ATTEMPT_ID}/result`, () => {
        reads += 1;
        return HttpResponse.error();
      }),
    );
    renderResult(new QueryClient({ defaultOptions: { queries: { retryDelay: 0 } } }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Không tải được kết quả",
    );
    expect(reads).toBe(3);
    expect(screen.queryByText("Mã lỗi")).toBeNull();
  });

  it("keeps a result that has loaded when a later read fails", async () => {
    serve(scored([choice(1, AB, [0], { earned: 1 })]));
    const client = renderResult();
    await screen.findByRole("heading", { level: 1 });
    refuse(500, "INTERNAL", "Lỗi máy chủ.");
    await act(() => client.refetchQueries({ queryKey: ["attempt-result"] }));
    expect(client.getQueryState(["attempt-result", ATTEMPT_ID])?.status).toBe("error");
    await expect(screen.findByRole("alert", {}, { timeout: 300 })).rejects.toThrow();
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Unit 4");
  });
});

describe("on a wide screen", () => {
  beforeEach(() => viewport("desktop"));

  it("draws its own way back above the summary, and no side panel", async () => {
    serve(passive(SCORE_ONLY, true));
    renderResult();
    await screen.findByText("The letter ____ yesterday.");
    expect(screen.queryByRole("complementary")).toBeNull();
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
      "Unit 4 — Passive voice",
    );
    expect(screen.getByRole("link", { name: "Trang chủ" })).toHaveAttribute(
      "href",
      "/app",
    );
    expect(screen.getByRole("heading", { level: 2 })).toHaveTextContent(
      "Câu trả lời của bạn",
    );
  });

  it("keeps the way back while the result loads and when it cannot be read", async () => {
    refuse(500, "INTERNAL", "Lỗi máy chủ.");
    renderResult();
    await screen.findByRole("alert");
    expect(screen.getByRole("link", { name: "Trang chủ" })).toHaveAttribute(
      "href",
      "/app",
    );
  });
});

describe("a paper in parts", () => {
  it("sums each part from its own questions, decimals included", async () => {
    viewport("desktop");
    serve(
      scored(
        [
          choice(1, AB, [1], { points: 1.5, earned: 1.5 }),
          choice(2, AB, [0], { points: 1.5, earned: 0 }),
          choice(3, AB, [1], { points: 2, earned: 2, sectionId: PART_TWO }),
        ],
        {
          sections: [
            { id: PART_ONE, title: "Nghe", instructions: null },
            { id: PART_TWO, title: "Đọc", instructions: null },
          ],
        },
      ),
    );
    renderResult();
    await screen.findByText("Nghe");
    expect(tiles()).toEqual([
      ["Nghe", "1,5 / 3", "50%"],
      ["Đọc", "2 / 2", "100%"],
    ]);
    expect(
      [...document.querySelectorAll("[data-slot=tile-bar]")].map((bar) =>
        bar.classList.contains("bg-success"),
      ),
    ).toEqual([false, true]);
    expect(within(ring()).getByText("3,5")).toBeVisible();
    expect(within(ring()).getByText("trên 5")).toBeVisible();
  });

  it("has no tiles while an answer waits in a paper of two parts", async () => {
    serve(
      scored(
        [
          choice(1, AB, [1], { earned: 1 }),
          essay(2, "Viết.", "Xong.", { sectionId: PART_TWO }),
        ],
        {
          sections: [
            { id: PART_ONE, title: "Nghe", instructions: null },
            { id: PART_TWO, title: "Viết", instructions: null },
          ],
        },
      ),
    );
    renderResult();
    await screen.findByText("Chờ giáo viên chấm");
    expect(tiles().map(([label]) => label)).toEqual([
      "Chấm tự động",
      "Chờ giáo viên chấm",
      "Thời gian làm bài",
    ]);
  });
});

it("branches on the shell's 768px and on no other width", async () => {
  const stubbed = window.matchMedia;
  const asked = new Set<string>();
  vi.stubGlobal("matchMedia", (query: string) => {
    asked.add(query);
    return stubbed(query);
  });
  serve(scored([choice(1, AB, [0], { earned: 1 })]));
  renderResult();
  await screen.findByText("Câu hỏi 1");
  expect([...asked]).toEqual(["(min-width: 768px)"]);
});

it("keeps the deck's 820px column and 112px ring", async () => {
  serve(scored([choice(1, AB, [0], { earned: 1 })]));
  renderResult();
  const card = (await screen.findByRole("heading", { level: 1 })).closest("section")!;
  expect(card.parentElement).toHaveClass("max-w-205", "mx-auto");
  expect(ring()).toHaveClass("size-28");
});
