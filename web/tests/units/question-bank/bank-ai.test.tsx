import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { act, screen, waitFor, within } from "@testing-library/react";
import { http } from "msw";
import vi_ from "@/lib/i18n/locales/vi.json";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import {
  BANK_API,
  bankPage,
  bankQuestion,
  renderBank,
} from "@tests/support/questionBank";
import { server } from "@tests/support/server";
import { aiOn, month } from "../ai/support";

const ai = vi_.ai;
const Q1 = "018f0000-0000-7000-8000-0000000000b1";
const Q2 = "018f0000-0000-7000-8000-0000000000b2";
const Q3 = "018f0000-0000-7000-8000-0000000000b3";

let lists = 0;

beforeEach(() => {
  lists = 0;
  server.use(
    http.get(`${BANK_API}/teacher/questions`, () => {
      lists += 1;
      return contractJson("/teacher/questions", "get", 200, bankPage([bankQuestion()]));
    }),
  );
});

afterEach(() => {
  useAuthStore.setState({ accessToken: null, user: null });
});

function header() {
  return screen.getByRole("link", { name: vi_.bank.newQuestion }).parentElement!;
}

describe("the bank's AI entry points", () => {
  it("sit before New question, and are not drawn without the assistant", async () => {
    aiOn();
    renderBank();
    await screen.findByRole("button", { name: ai.entry.cleanup });
    expect(
      within(header())
        .getAllByRole("button")
        .concat(within(header()).getAllByRole("link"))
        .map((control) => control.textContent),
    ).toEqual([ai.entry.cleanup, ai.entry.generate, vi_.bank.newQuestion]);
  });

  it("are hidden while the server has no assistant", async () => {
    aiOn({ ai: false });
    renderBank();
    await screen.findByRole("link", { name: vi_.bank.newQuestion });
    expect(screen.queryByRole("button", { name: ai.entry.cleanup })).toBeNull();
    expect(screen.queryByRole("button", { name: ai.entry.generate })).toBeNull();
  });

  it("are disabled with the pool's sentence when the school's pool is used up", async () => {
    aiOn({ usage: month({ used: 1200, allowance: 20000, left: 0 }) });
    renderBank();
    await screen.findByRole("button", { name: ai.entry.cleanup });
    await waitFor(() =>
      expect(screen.getByRole("button", { name: ai.entry.cleanup })).toHaveAttribute(
        "aria-disabled",
        "true",
      ),
    );
    expect(screen.getByRole("button", { name: ai.entry.generate })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
    act(() => screen.getByRole("button", { name: ai.entry.cleanup }).focus());
    expect(await screen.findByRole("tooltip")).toHaveTextContent(ai.blocked.pool);
  });

  it("tag the kept suggestions through the bulk tag write, one call for each set of tags", async () => {
    aiOn();
    const tagged: unknown[] = [];
    server.use(
      http.post(`${BANK_API}/teacher/ai/bank/cleanup`, () =>
        contractJson("/teacher/ai/bank/cleanup", "post", 200, {
          scanned: 3,
          truncated: false,
          duplicates: [],
          tags: [
            { questionId: Q1, prompt: "One?", tags: ["maps", "directions"] },
            { questionId: Q2, prompt: "Two?", tags: ["directions", "maps"] },
            { questionId: Q3, prompt: "Three?", tags: ["grammar"] },
          ],
          usage: { credits: 900, left: 1000 },
        }),
      ),
      http.post(`${BANK_API}/teacher/questions/tags`, async ({ request }) => {
        tagged.push(await request.json());
        return contractJson("/teacher/questions/tags", "post", 200, { updated: 1 });
      }),
    );
    const { user } = renderBank();
    await user.click(await screen.findByRole("button", { name: ai.entry.cleanup }));
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: ai.cleanup.run }));
    await user.click(
      await within(dialog).findByRole("button", { name: "Thêm thẻ cho 3 câu" }),
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(tagged).toEqual([
      { questionIds: [Q1, Q2], tags: ["directions", "maps"] },
      { questionIds: [Q3], tags: ["grammar"] },
    ]);
    await waitFor(() => expect(lists).toBeGreaterThan(1));
  });

  it("save generated questions to the bank, tagged", async () => {
    aiOn();
    const created: { tags: string[]; type: string }[] = [];
    server.use(
      http.post(`${BANK_API}/teacher/ai/questions/generate`, () =>
        contractJson("/teacher/ai/questions/generate", "post", 200, {
          items: [
            {
              type: "true_false",
              prompt: "Parks are decoration.",
              options: ["True", "False"],
              answerKey: ["False"],
              explanation: "",
            },
          ],
          usage: { credits: 600, left: 1000 },
        }),
      ),
      http.post(`${BANK_API}/teacher/questions`, async ({ request }) => {
        const body = (await request.json()) as { tags: string[]; type: string };
        created.push(body);
        return contractJson("/teacher/questions", "post", 201, {
          ...bankQuestion({ id: Q3, type: "true_false", tags: body.tags }),
          usedInTests: undefined,
          options: [
            {
              id: "018f0000-0000-7000-8000-0000000000d1",
              ordinal: 0,
              text: "True",
              isCorrect: false,
            },
            {
              id: "018f0000-0000-7000-8000-0000000000d2",
              ordinal: 1,
              text: "False",
              isCorrect: true,
            },
          ],
        });
      }),
    );
    const { user } = renderBank();
    await user.click(await screen.findByRole("button", { name: ai.entry.generate }));
    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).queryByRole("button", { name: ai.generate.startEmpty }),
    ).toBeNull();
    await user.type(
      within(dialog).getByLabelText(ai.generate.source),
      "Cities around the world are rethinking how they use open land.",
    );
    await user.click(within(dialog).getByRole("button", { name: ai.generate.run }));
    await user.click(
      await within(dialog).findByRole("button", { name: "Lưu 1 câu vào ngân hàng" }),
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(created).toMatchObject([{ type: "true_false", tags: [ai.draftTag] }]);
  });
});
