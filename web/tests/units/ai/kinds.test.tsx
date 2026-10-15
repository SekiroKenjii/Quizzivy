import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { http } from "msw";
import { useState } from "react";
import AiPanel, { type AiPanelRequest } from "@/features/ai/components/AiPanel";
import type { AiCleanupResult, AiVariantChange } from "@/features/ai/api";
import type { CleanupContext } from "@/features/ai/kinds/cleanup";
import vi_ from "@/lib/i18n/locales/vi.json";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import { aiOn, BASE, renderWithApp } from "./support";

const ai = vi_.ai;
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const COPY_ID = "018f0000-0000-7000-8000-0000000000a2";
const Q1 = "018f0000-0000-7000-8000-0000000000b1";
const Q2 = "018f0000-0000-7000-8000-0000000000b2";
const Q3 = "018f0000-0000-7000-8000-0000000000b3";

const changes: AiVariantChange[] = [
  {
    questionId: Q1,
    number: 1,
    kind: "reword",
    title: "Main purpose of the passage",
    before: "What is the main purpose of the passage?",
    after: "What is the writer mainly trying to do in this passage?",
    payload: { prompt: "What is the writer mainly trying to do in this passage?" },
  },
  {
    questionId: Q2,
    number: 3,
    kind: "reorder",
    title: "Parks in paragraph B",
    before: "A · B (correct) · C",
    after: "C · A · B (correct)",
    payload: {
      optionOrder: [
        "018f0000-0000-7000-8000-0000000000d3",
        "018f0000-0000-7000-8000-0000000000d1",
        "018f0000-0000-7000-8000-0000000000d2",
      ],
    },
  },
];

function cleanup(patch: Partial<AiCleanupResult> = {}): AiCleanupResult {
  return {
    scanned: 2000,
    truncated: false,
    duplicates: [
      {
        a: { questionId: Q1, prompt: "She ___ in Hanoi since 2019.", usedInTests: 4 },
        b: { questionId: Q2, prompt: "He ___ in Da Nang since 2020.", usedInTests: 1 },
        why: "Same grammar point and structure.",
      },
    ],
    tags: [
      {
        questionId: Q3,
        prompt: "Label the map.",
        level: "b1",
        tags: ["directions", "maps"],
      },
    ],
    usage: { credits: 2900, left: 15000 },
    ...patch,
  };
}

function Closable({ request }: Readonly<{ request: AiPanelRequest }>) {
  const [open, setOpen] = useState(true);
  return open ? <AiPanel request={request} onClose={() => setOpen(false)} /> : null;
}

beforeEach(() => {
  aiOn();
});

afterEach(() => {
  useAuthStore.setState({ accessToken: null, user: null });
});

describe("the Variant panel", () => {
  it("asks for the changes it was given, and saves the kept ones as a new draft test", async () => {
    let body: unknown;
    server.use(
      http.post(`${BASE}/teacher/ai/tests/:id/variant`, async ({ request }) => {
        body = await request.json();
        return contractJson("/teacher/ai/tests/{id}/variant", "post", 200, {
          changes,
          usage: { credits: 2600, left: 15000 },
        });
      }),
    );
    const save = vi.fn(() => Promise.resolve(COPY_ID));
    const { user, router } = renderWithApp(
      <Closable
        request={{
          kind: "variant",
          context: { testId: TEST_ID, title: "Unit 5", save },
        }}
      />,
    );
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText(ai.variant.info)).toBeVisible();
    await user.click(
      within(dialog).getByRole("button", { name: ai.variant.difficulties.harder }),
    );
    await user.click(within(dialog).getByRole("button", { name: ai.variant.run }));

    const before = await within(dialog).findByText(
      "What is the main purpose of the passage?",
    );
    expect(before).toHaveClass("line-through");
    expect(body).toEqual({ changes: ["reword", "reorder"], difficulty: "harder" });
    expect(within(dialog).getByText(ai.variant.kinds.reorder)).toBeVisible();
    expect(within(dialog).getByText("Câu 3")).toBeVisible();

    await user.click(within(dialog).getAllByRole("checkbox", { name: ai.keep })[1]!);
    await user.click(
      within(dialog).getByRole("button", { name: "Lưu 1 thay đổi thành bản nháp" }),
    );
    expect(save).toHaveBeenCalledWith([changes[0]]);
    expect(await screen.findByText(ai.variant.saved)).toBeVisible();
    await user.click(screen.getByRole("button", { name: ai.variant.open }));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/teacher/tests/${COPY_ID}`),
    );
  });
});

describe("the Clean up panel", () => {
  function context(patch: Partial<CleanupContext> = {}): CleanupContext {
    return {
      href: (id) => `/teacher/question-bank/${id}`,
      tag: vi.fn(() => Promise.resolve(1)),
      ...patch,
    };
  }

  it("lists a duplicate with Open for each and Keep both, and tags only the kept suggestions", async () => {
    let body: unknown;
    server.use(
      http.post(`${BASE}/teacher/ai/bank/cleanup`, async ({ request }) => {
        body = await request.json();
        return contractJson("/teacher/ai/bank/cleanup", "post", 200, cleanup());
      }),
    );
    const ctx = context();
    const { user } = renderWithApp(
      <AiPanel request={{ kind: "cleanup", context: ctx }} onClose={() => undefined} />,
    );
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText(ai.cleanup.info)).toBeVisible();
    await user.click(within(dialog).getByRole("button", { name: ai.cleanup.run }));

    const pair = (
      await within(dialog).findByText("Same grammar point and structure.")
    ).closest("[data-slot=ai-item]") as HTMLElement;
    expect(body).toEqual({ checks: ["duplicates", "tags"], language: "vi" });
    expect(within(pair).queryByRole("checkbox")).toBeNull();
    expect(within(pair).getByText("Dùng trong 4 đề và 1 đề")).toBeVisible();
    expect(
      within(pair).getByRole("link", { name: ai.cleanup.openOne }),
    ).toHaveAttribute("href", `/teacher/question-bank/${Q1}`);
    expect(
      within(pair).getByRole("link", { name: ai.cleanup.openOther }),
    ).toHaveAttribute("href", `/teacher/question-bank/${Q2}`);
    expect(within(dialog).queryByText(/Gộp|Merge/)).toBeNull();
    expect(within(dialog).getByText("directions")).toBeVisible();
    expect(
      within(dialog).getByText("1 cặp có thể trùng · 1 câu thiếu thẻ"),
    ).toBeVisible();

    await user.click(within(pair).getByRole("button", { name: ai.cleanup.keepBoth }));
    expect(within(dialog).queryByText("Same grammar point and structure.")).toBeNull();
    await user.click(
      within(dialog).getByRole("button", { name: "Thêm thẻ cho 1 câu" }),
    );
    expect(ctx.tag).toHaveBeenCalledWith([cleanup().tags[0]]);
    expect(await screen.findByText("Đã thêm thẻ cho 1 câu")).toBeVisible();
  });

  it("says to apply and run again when the scan stopped short, and is Done with nothing to tag", async () => {
    server.use(
      http.post(`${BASE}/teacher/ai/bank/cleanup`, () =>
        contractJson(
          "/teacher/ai/bank/cleanup",
          "post",
          200,
          cleanup({ truncated: true, tags: [] }),
        ),
      ),
    );
    const { user } = renderWithApp(
      <AiPanel
        request={{ kind: "cleanup", context: context() }}
        onClose={() => undefined}
      />,
    );
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: ai.cleanup.run }));
    expect(
      await within(dialog).findByText(
        "Lần quét này đọc 2000 câu, chưa hết ngân hàng. Hãy áp dụng các gợi ý này rồi chạy lại.",
      ),
    ).toBeVisible();
    expect(within(dialog).getByRole("button", { name: ai.done })).toBeVisible();
  });

  it("shows the empty sentence when the bank is tidy", async () => {
    server.use(
      http.post(`${BASE}/teacher/ai/bank/cleanup`, () =>
        contractJson(
          "/teacher/ai/bank/cleanup",
          "post",
          200,
          cleanup({ duplicates: [], tags: [] }),
        ),
      ),
    );
    const { user } = renderWithApp(
      <AiPanel
        request={{ kind: "cleanup", context: context() }}
        onClose={() => undefined}
      />,
    );
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: ai.cleanup.run }));
    expect(await within(dialog).findByText(ai.cleanup.empty)).toBeVisible();
  });
});
