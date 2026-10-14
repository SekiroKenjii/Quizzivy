import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { useState } from "react";
import AiPanel, { type AiPanelRequest } from "@/features/ai/components/AiPanel";
import {
  AI_USAGE_KEY,
  type AiFinding,
  type AiGeneratedQuestion,
} from "@/features/ai/api";
import type { GenerateContext } from "@/features/ai/kinds/generate";
import type { CheckContext } from "@/features/ai/kinds/check";
import vi_ from "@/lib/i18n/locales/vi.json";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import { aiError, aiOn, BASE, month, renderWithApp } from "./support";

const ai = vi_.ai;
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const QUESTION_ID = "018f0000-0000-7000-8000-0000000000b1";
const OPTION_ID = "018f0000-0000-7000-8000-0000000000d1";
const PASSAGE =
  "Cities around the world are rethinking how they use open land, and parks matter.";

const generated: AiGeneratedQuestion[] = [
  {
    type: "single_choice",
    prompt: "What do cities rethink?",
    options: ["Open land", "Roads"],
    answerKey: ["Open land"],
    explanation: "The first sentence says so.",
  },
  {
    type: "fill_blank",
    prompt: "Cities rethink how they use open ___.",
    answerKey: ["land"],
    explanation: "",
  },
];

const finding = (patch: Partial<AiFinding> = {}): AiFinding => ({
  severity: "likely_error",
  questionId: QUESTION_ID,
  number: 4,
  type: "single_choice",
  title: "What do cities rethink?",
  issue: "Two options fit the passage.",
  fix: {
    kind: "option_text",
    description: "Reword option B.",
    label: "Use rewording",
    payload: { optionId: OPTION_ID, text: "Parking" },
  },
  ...patch,
});

function generateContext(patch: Partial<GenerateContext> = {}): GenerateContext {
  return {
    testId: TEST_ID,
    passages: [
      { id: "018f0000-0000-7000-8000-0000000000e1", title: "Parks", text: PASSAGE },
    ],
    destination: { kind: "section", title: "Phần 1" },
    save: vi.fn(() => Promise.resolve()),
    ...patch,
  };
}

const GROUP_ID = "018f0000-0000-7000-8000-0000000000f1";

const groupPassage = {
  id: "018f0000-0000-7000-8000-0000000000e1",
  title: "Parks",
  text: PASSAGE,
  group: { id: GROUP_ID, title: "Reading club" },
};

function answerWith(items: AiGeneratedQuestion[]) {
  server.use(
    http.post(`${BASE}/teacher/ai/questions/generate`, () =>
      contractJson("/teacher/ai/questions/generate", "post", 200, {
        items,
        usage: { credits: 900, left: 17900 },
      }),
    ),
  );
}

function checkContext(patch: Partial<CheckContext> = {}): CheckContext {
  return {
    testId: TEST_ID,
    title: "Unit 5",
    numberOf: () => 7,
    fix: vi.fn(() => Promise.resolve()),
    open: vi.fn(),
    ...patch,
  };
}

function Host({ request }: Readonly<{ request: AiPanelRequest }>) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}>
        open panel
      </button>
      {open && <AiPanel request={request} onClose={() => setOpen(false)} />}
    </>
  );
}

async function openPanel(request: AiPanelRequest) {
  const view = renderWithApp(<Host request={request} />);
  await view.user.click(screen.getByRole("button", { name: "open panel" }));
  return { ...view, dialog: await screen.findByRole("dialog") };
}

beforeEach(() => {
  aiOn();
});

afterEach(() => {
  useAuthStore.setState({ accessToken: null, user: null });
});

describe("the Generate panel", () => {
  it("starts from the test's passage, with the deck's defaults and credit note", async () => {
    const { dialog } = await openPanel({
      kind: "generate",
      context: generateContext(),
    });
    expect(
      within(dialog).getByRole("heading", { name: ai.generate.title }),
    ).toBeVisible();
    expect(within(dialog).getByLabelText(ai.generate.source)).toHaveValue(PASSAGE);
    expect(
      within(dialog).getByRole("button", { name: "Bài đọc 1 · Parks", pressed: true }),
    ).toBeVisible();
    const types = within(dialog).getByRole("group", { name: ai.generate.types });
    expect(
      within(types)
        .getAllByRole("button", { pressed: true })
        .map((b) => b.textContent),
    ).toEqual([
      vi_.questionEditor.type.single_choice,
      vi_.questionEditor.type.true_false,
    ]);
    expect(
      within(dialog).getByRole("group", { name: ai.generate.count }),
    ).toHaveTextContent("358");
    expect(await within(dialog).findByText(/Còn 18\.800 tín dụng/)).toBeVisible();
  });

  it("names what stops the run, and does not start it", async () => {
    const { dialog, user } = await openPanel({
      kind: "generate",
      context: generateContext(),
    });
    await user.click(
      within(dialog).getByRole("button", { name: ai.generate.startEmpty }),
    );
    expect(within(dialog).getByText(ai.generate.needSource)).toBeVisible();
    expect(
      within(dialog).getByRole("button", { name: ai.generate.run }),
    ).toBeDisabled();
  });

  it("asks for the passage by id, then saves only the kept questions with the AI draft tag", async () => {
    let body: unknown;
    server.use(
      http.post(`${BASE}/teacher/ai/questions/generate`, async ({ request }) => {
        body = await request.json();
        return contractJson("/teacher/ai/questions/generate", "post", 200, {
          items: generated,
          usage: { credits: 1450, left: 17350 },
        });
      }),
    );
    const context = generateContext();
    const { dialog, user, client } = await openPanel({ kind: "generate", context });
    await within(dialog).findByText(/Còn 18\.800 tín dụng/);
    await user.click(within(dialog).getByRole("button", { name: ai.generate.run }));
    expect(await within(dialog).findByText("What do cities rethink?")).toBeVisible();
    expect(body).toEqual({
      types: ["single_choice", "true_false"],
      count: 5,
      level: "b2",
      explanations: true,
      language: "vi",
      testId: TEST_ID,
      stimulusId: "018f0000-0000-7000-8000-0000000000e1",
    });
    expect(
      within(dialog).getByText(/Đã dùng 1\.450 tín dụng · còn 17\.350/),
    ).toBeVisible();
    expect(client.getQueryData(AI_USAGE_KEY)).toMatchObject({
      used: 2650,
      left: 17350,
    });
    expect(within(dialog).getAllByText(ai.draftTag)).toHaveLength(2);
    expect(within(dialog).getByText("land")).toBeVisible();

    const keep = within(dialog).getAllByRole("checkbox", { name: ai.keep });
    await user.click(keep[1]!);
    await user.click(
      within(dialog).getByRole("button", { name: "Thêm 1 câu vào Phần 1" }),
    );

    expect(context.save).toHaveBeenCalledWith(
      [{ question: generated[0], level: "b2" }],
      ai.draftTag,
      { kind: "section", title: "Phần 1" },
    );
    expect(
      await screen.findByText(
        "Đã thêm 1 câu hỏi vào Phần 1. Hãy kiểm tra từng câu trước khi xuất bản.",
      ),
    ).toBeVisible();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("adds what it made from a group's passage to that group, named on the button", async () => {
    answerWith(generated);
    const context = generateContext({ passages: [groupPassage] });
    const { dialog, user } = await openPanel({ kind: "generate", context });
    await user.click(within(dialog).getByRole("button", { name: ai.generate.run }));
    await user.click(
      await within(dialog).findByRole("button", {
        name: "Thêm 2 câu vào nhóm Reading club",
      }),
    );

    expect(context.save).toHaveBeenCalledWith(
      [
        { question: generated[0], level: "b2" },
        { question: generated[1], level: "b2" },
      ],
      ai.draftTag,
      { kind: "group", id: GROUP_ID, title: "Reading club" },
    );
    expect(
      await screen.findByText(
        "Đã thêm 2 câu hỏi vào nhóm Reading club. Hãy kiểm tra từng câu trước khi xuất bản.",
      ),
    ).toBeVisible();
  });

  it("keeps the section for pasted text, even where a group's passage is offered", async () => {
    answerWith(generated.slice(0, 1));
    const context = generateContext({ passages: [groupPassage] });
    const { dialog, user } = await openPanel({ kind: "generate", context });
    const source = within(dialog).getByLabelText(ai.generate.source);
    await user.clear(source);
    await user.type(source, `${PASSAGE} A topic.`);
    await user.click(within(dialog).getByRole("button", { name: ai.generate.run }));
    await user.click(
      await within(dialog).findByRole("button", { name: "Thêm 1 câu vào Phần 1" }),
    );

    expect(context.save).toHaveBeenCalledWith(
      [{ question: generated[0], level: "b2" }],
      ai.draftTag,
      { kind: "section", title: "Phần 1" },
    );
  });

  it.each([
    [{ kind: "bank" } as const, "Lưu 1 câu vào ngân hàng"],
    [{ kind: "practice" } as const, ai.generate.saveAsPractice],
  ])(
    "saves to the %o destination it was given, whatever passage it read",
    async (destination, label) => {
      answerWith(generated.slice(0, 1));
      const context = generateContext({ passages: [groupPassage], destination });
      const { dialog, user } = await openPanel({ kind: "generate", context });
      await user.click(within(dialog).getByRole("button", { name: ai.generate.run }));
      await user.click(await within(dialog).findByRole("button", { name: label }));

      expect(context.save).toHaveBeenCalledWith(
        [{ question: generated[0], level: "b2" }],
        ai.draftTag,
        destination,
      );
    },
  );

  it("sends pasted text as the source and leaves the tag off when the preference is", async () => {
    useAuthStore.setState((state) => ({
      user: state.user && {
        ...state.user,
        preferences: { ai: { tagDrafts: false, language: "en" } },
      },
    }));
    let body: Record<string, unknown> = {};
    server.use(
      http.post(`${BASE}/teacher/ai/questions/generate`, async ({ request }) => {
        body = (await request.json()) as Record<string, unknown>;
        return contractJson("/teacher/ai/questions/generate", "post", 200, {
          items: generated.slice(0, 1),
          usage: { credits: 900, left: 100 },
        });
      }),
    );
    const { dialog, user } = await openPanel({
      kind: "generate",
      context: generateContext({ passages: [] }),
    });
    const source = within(dialog).getByLabelText(ai.generate.source);
    await user.type(source, `${PASSAGE} A topic.`);
    await user.click(within(dialog).getByRole("button", { name: ai.generate.run }));
    await within(dialog).findByText("What do cities rethink?");
    expect(body["source"]).toBe(`${PASSAGE} A topic.`);
    expect(body["testId"]).toBeUndefined();
    expect(body["language"]).toBe("en");
    expect(within(dialog).queryByText(ai.draftTag)).toBeNull();
  });

  it("draws an outage as the warning callout and runs again from it, never sample output", async () => {
    let calls = 0;
    server.use(
      http.post(`${BASE}/teacher/ai/questions/generate`, () => {
        calls += 1;
        if (calls === 1) return aiError(503, "AI_UNAVAILABLE");
        return contractJson("/teacher/ai/questions/generate", "post", 200, {
          items: [],
          usage: { credits: 300, left: 18500 },
        });
      }),
    );
    const { dialog, user } = await openPanel({
      kind: "generate",
      context: generateContext(),
    });
    await user.click(within(dialog).getByRole("button", { name: ai.generate.run }));
    const alert = await within(dialog).findByRole("alert");
    expect(alert).toHaveTextContent(ai.unavailable);
    expect(within(dialog).getByLabelText(ai.generate.source)).toHaveValue(PASSAGE);

    await user.click(within(alert).getByRole("button", { name: ai.tryAgain }));
    expect(await within(dialog).findByText(ai.generate.empty)).toBeVisible();
    expect(within(dialog).getByRole("button", { name: ai.done })).toBeVisible();
    expect(calls).toBe(2);
  });

  it("treats an answer that fails the schema as the same outage", async () => {
    server.use(
      http.post(`${BASE}/teacher/ai/questions/generate`, () =>
        HttpResponse.json({ questions: "not the contract" }),
      ),
    );
    const { dialog, user } = await openPanel({
      kind: "generate",
      context: generateContext(),
    });
    await user.click(within(dialog).getByRole("button", { name: ai.generate.run }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(ai.unavailable);
  });

  it.each([
    [402, "AI_CREDITS_EXHAUSTED", { scope: "teacher" }, ai.blocked.teacher],
    [402, "AI_CREDITS_EXHAUSTED", { scope: "pool" }, ai.blocked.pool],
    [403, "AI_PAUSED", undefined, ai.blocked.paused],
  ])(
    "says why a %i %s stops it, in the footer",
    async (status, code, details, sentence) => {
      server.use(
        http.post(`${BASE}/teacher/ai/questions/generate`, () =>
          aiError(status, code, details),
        ),
      );
      const { dialog, user } = await openPanel({
        kind: "generate",
        context: generateContext(),
      });
      await user.click(within(dialog).getByRole("button", { name: ai.generate.run }));
      expect(await within(dialog).findByText(sentence)).toBeVisible();
      expect(
        within(dialog).getByRole("button", { name: ai.generate.run }),
      ).toBeDisabled();
    },
  );

  it("is stopped before it starts when the usage read says paused", async () => {
    aiOn({ usage: month({ paused: true }) });
    const { dialog } = await openPanel({
      kind: "generate",
      context: generateContext(),
    });
    expect(await within(dialog).findByText(ai.blocked.paused)).toBeVisible();
    expect(
      within(dialog).getByRole("button", { name: ai.generate.run }),
    ).toBeDisabled();
  });

  it("walks its steps while the call runs, and goes back to its fields on Stop", async () => {
    server.use(
      http.post(
        `${BASE}/teacher/ai/questions/generate`,
        () => new Promise<never>(() => undefined),
      ),
    );
    const { dialog, user } = await openPanel({
      kind: "generate",
      context: generateContext(),
    });
    await user.click(within(dialog).getByRole("button", { name: ai.generate.run }));
    const steps = await within(dialog).findByRole("status");
    expect(within(steps).getByText(ai.generate.step1)).toHaveAttribute(
      "aria-current",
      "step",
    );
    await waitFor(
      () =>
        expect(within(steps).getByText(ai.generate.step2)).toHaveAttribute(
          "aria-current",
          "step",
        ),
      { timeout: 2000 },
    );
    await user.click(within(dialog).getByRole("button", { name: ai.stop }));
    expect(within(dialog).getByLabelText(ai.generate.source)).toBeVisible();
  });

  it("closes on Esc, keeps focus inside and gives it back to the opener", async () => {
    const { dialog, user } = await openPanel({
      kind: "generate",
      context: generateContext(),
    });
    expect(dialog).toContainElement(document.activeElement as HTMLElement);
    for (let i = 0; i < 20; i += 1) {
      await user.tab();
      expect(dialog).toContainElement(document.activeElement as HTMLElement);
    }
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.getByRole("button", { name: "open panel" })).toHaveFocus();
  });
});

describe("the Check panel", () => {
  function answer(findings: AiFinding[]) {
    server.use(
      http.post(`${BASE}/teacher/ai/tests/:id/check`, () =>
        contractJson("/teacher/ai/tests/{id}/check", "post", 200, {
          findings,
          usage: { credits: 2100, left: 16700 },
        }),
      ),
    );
  }

  it("draws a finding as the deck does and applies a one-click fix through the host", async () => {
    answer([finding()]);
    const context = checkContext();
    const { dialog, user } = await openPanel({ kind: "check", context });
    expect(within(dialog).getByText(/“Unit 5”/)).toBeVisible();
    await user.click(within(dialog).getByRole("button", { name: ai.check.run }));

    const card = (
      await within(dialog).findByText("Two options fit the passage.")
    ).closest("[data-slot=ai-item]") as HTMLElement;
    expect(within(card).getByText(ai.check.severity.likely_error)).toBeVisible();
    expect(within(card).getByText("Câu 7")).toBeVisible();
    expect(within(card).getByText("Reword option B.")).toBeVisible();
    expect(within(dialog).getByText("1 điều nên xem lại")).toBeVisible();

    await user.click(within(card).getByRole("button", { name: "Use rewording" }));
    expect(context.fix).toHaveBeenCalledWith(finding());
    expect(await screen.findByText("Đã cập nhật câu 7")).toBeVisible();
    expect(await within(dialog).findByText(ai.check.empty)).toBeVisible();
  });

  it("offers only Open question and Dismiss for advice", async () => {
    answer([
      finding({ fix: { kind: "none", description: "Add a word limit." } }),
      finding({ questionId: "018f0000-0000-7000-8000-0000000000b2", issue: "Second." }),
    ]);
    const context = checkContext();
    const { dialog, user } = await openPanel({ kind: "check", context });
    await user.click(within(dialog).getByRole("button", { name: ai.check.run }));
    const advice = (
      await within(dialog).findByText("Two options fit the passage.")
    ).closest("[data-slot=ai-item]") as HTMLElement;
    expect(
      within(advice)
        .getAllByRole("button")
        .map((b) => b.textContent),
    ).toEqual([ai.check.openQuestion, ai.check.dismiss]);

    await user.click(within(advice).getByRole("button", { name: ai.check.dismiss }));
    expect(within(dialog).queryByText("Two options fit the passage.")).toBeNull();
    expect(within(dialog).getByText("Second.")).toBeVisible();

    await user.click(
      within(dialog).getByRole("button", { name: ai.check.openQuestion }),
    );
    expect(context.open).toHaveBeenCalledWith("018f0000-0000-7000-8000-0000000000b2");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("keeps a card and says so when the fix cannot be written", async () => {
    answer([finding()]);
    const context = checkContext({
      fix: vi.fn(() => Promise.reject(new Error("boom"))),
    });
    const { dialog, user } = await openPanel({ kind: "check", context });
    await user.click(within(dialog).getByRole("button", { name: ai.check.run }));
    await user.click(
      await within(dialog).findByRole("button", { name: "Use rewording" }),
    );
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(ai.applyFailed);
    expect(within(dialog).getByText("Two options fit the passage.")).toBeVisible();
  });
});
