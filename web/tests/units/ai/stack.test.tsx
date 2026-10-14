import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { http } from "msw";
import { useState } from "react";
import { AiPanelStack } from "@/features/ai/components/AiPanelStack";
import type { AiFinding, AiGeneratedQuestion } from "@/features/ai/api";
import vi_ from "@/lib/i18n/locales/vi.json";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import { aiOn, BASE, renderWithApp } from "./support";

const ai = vi_.ai;
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const QUESTION_ID = "018f0000-0000-7000-8000-0000000000b1";
const SOURCE =
  "Cities around the world are rethinking how they use open land, and parks matter.";

type Kind = "check" | "generate";

const finding: AiFinding = {
  severity: "worth_a_look",
  questionId: QUESTION_ID,
  number: 2,
  type: "single_choice",
  title: "What do cities rethink?",
  issue: "Two options fit the passage.",
  fix: { kind: "none", description: "Reword option B." },
};

const drafted: AiGeneratedQuestion = {
  type: "short_answer",
  prompt: "What do cities rethink?",
  answerKey: ["open land"],
  explanation: "",
};

let checks = 0;
let generations = 0;
const save = vi.fn(() => Promise.resolve());

function Page() {
  const [open, setOpen] = useState<Kind | null>(null);
  return (
    <>
      {(["check", "generate"] as const).map((kind) => (
        <button key={kind} type="button" onClick={() => setOpen(kind)}>
          show {kind}
        </button>
      ))}
      <AiPanelStack
        open={open}
        onClose={() => setOpen(null)}
        requestFor={(kind) =>
          kind === "check"
            ? {
                kind,
                context: {
                  testId: TEST_ID,
                  title: "Unit 5",
                  numberOf: () => 2,
                  fix: () => Promise.resolve(),
                  open: () => undefined,
                },
              }
            : {
                kind,
                context: {
                  testId: TEST_ID,
                  passages: [],
                  destination: { kind: "section", title: "Phần 1" },
                  save,
                },
              }
        }
      />
    </>
  );
}

function show(kind: Kind) {
  fireEvent.click(screen.getByText(`show ${kind}`));
}

beforeEach(() => {
  checks = 0;
  generations = 0;
  save.mockClear();
  aiOn();
  server.use(
    http.post(`${BASE}/teacher/ai/tests/:id/check`, () => {
      checks += 1;
      return contractJson("/teacher/ai/tests/{id}/check", "post", 200, {
        findings: [finding],
        usage: { credits: 400, left: 1000 },
      });
    }),
    http.post(`${BASE}/teacher/ai/questions/generate`, () => {
      generations += 1;
      return contractJson("/teacher/ai/questions/generate", "post", 200, {
        items: [drafted],
        usage: { credits: 500, left: 1000 },
      });
    }),
  );
});

afterEach(() => {
  useAuthStore.setState({ accessToken: null, user: null });
});

describe("AiPanelStack", () => {
  it("draws nothing until a panel is opened", () => {
    renderWithApp(<Page />);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(checks + generations).toBe(0);
  });

  it("gives a panel back with its findings after the drawer was closed", async () => {
    const { user } = renderWithApp(<Page />);
    show("check");
    let dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: ai.check.run }));
    await within(dialog).findByText("Two options fit the passage.");
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());

    show("check");
    dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Two options fit the passage.")).toBeVisible();
    expect(within(dialog).queryByRole("button", { name: ai.check.run })).toBeNull();
    expect(checks).toBe(1);
  });

  it("keeps each panel's own session while another is showing", async () => {
    const { user } = renderWithApp(<Page />);
    show("generate");
    let dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText(ai.generate.source), SOURCE);
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());

    show("check");
    dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByRole("heading", { name: ai.check.title })).toBeVisible();
    await user.click(within(dialog).getByRole("button", { name: ai.check.run }));
    await within(dialog).findByText("Two options fit the passage.");
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());

    show("generate");
    dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByLabelText(ai.generate.source)).toHaveValue(SOURCE);
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    show("check");
    expect(
      await within(await screen.findByRole("dialog")).findByText(
        "Two options fit the passage.",
      ),
    ).toBeVisible();
  });

  it("leaves a panel ready for another run once its questions were kept", async () => {
    const { user } = renderWithApp(<Page />);
    for (const round of [1, 2]) {
      show("generate");
      const dialog = await screen.findByRole("dialog");
      if (round === 1)
        await user.type(within(dialog).getByLabelText(ai.generate.source), SOURCE);
      await user.click(within(dialog).getByRole("button", { name: ai.generate.run }));
      await user.click(
        await within(dialog).findByRole("button", { name: "Thêm 1 câu vào Phần 1" }),
      );
      await waitFor(() => expect(save).toHaveBeenCalledTimes(round));
      await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    }
    expect(generations).toBe(2);
  });
});
