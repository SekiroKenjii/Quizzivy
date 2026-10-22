import { describe, expect, it } from "vitest";
import { useState } from "react";
import { render, renderHook, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import "@/lib/i18n";
import { t } from "i18next";
import { GroupComposer } from "@/features/question-groups/components/GroupComposer";
import type { GroupBundle } from "@/features/question-groups/api";
import { emptyGroup, newGroupQuestion } from "@/features/question-groups/model";
import {
  criterionKey,
  type QuestionValues,
} from "@/features/question-bank/questionSchema";
import { adoptIds } from "@/features/tests/adoptIds";
import { draftUnkeyed } from "@/features/tests/publishProblem";
import { useReportUnkeyed } from "@/features/tests/useReportUnkeyed";

const ID = (n: number) =>
  `0190a000-0000-7000-8000-0000000000${String(n).padStart(2, "0")}`;

function values(over: Partial<QuestionValues>): QuestionValues {
  return {
    type: "matching",
    level: null,
    skill: null,
    prompt: "p",
    points: 1,
    options: [
      { id: null, text: "cat", isCorrect: false },
      { id: null, text: "dog", isCorrect: false },
    ],
    matchAllowReuse: false,
    matchItems: [
      { id: null, text: "meow", correctOption: 0 },
      { id: null, text: "woof", correctOption: 1 },
    ],
    ...over,
  } as QuestionValues;
}

describe("rubric labels compared as the server does (QC-R6AC-4)", () => {
  it("folds a dotted capital I to i, and a final sigma to sigma", () => {
    expect(criterionKey(" İdea ")).toBe(criterionKey("idea"));
    expect(criterionKey("ΟΔΟΣ")).toBe(criterionKey("οδοσ"));
    expect(criterionKey("Ý")).toBe(criterionKey("ý"));
  });
});

describe("the builder adopts the ids of rows it created (QC-R6AC-8)", () => {
  const saved = values({
    options: [
      { id: ID(1), text: "cat", isCorrect: false },
      { id: ID(2), text: "dog", isCorrect: false },
    ],
    matchItems: [
      { id: ID(3), text: "meow", correctOption: 0 },
      { id: ID(4), text: "woof", correctOption: 1 },
    ],
  });

  it("sends the saved ids for new rows on the next save", () => {
    const sent = values({});
    const next = values({
      matchItems: [
        { id: null, text: "meow!", correctOption: 0 },
        { id: null, text: "woof", correctOption: 1 },
      ],
    });
    const body = adoptIds(next, sent, saved);
    expect(body.options.map((o) => o.id)).toEqual([ID(1), ID(2)]);
    expect(body.matchItems!.map((i) => [i.id, i.text])).toEqual([
      [ID(3), "meow!"],
      [ID(4), "woof"],
    ]);
  });

  it("leaves a list alone while a row has been added or removed since", () => {
    const next = values({
      options: [
        { id: null, text: "cat", isCorrect: false },
        { id: null, text: "dog", isCorrect: false },
        { id: null, text: "bird", isCorrect: false },
      ],
    });
    const body = adoptIds(next, values({}), saved);
    expect(body.options.map((o) => o.id)).toEqual([null, null, null]);
    expect(body.matchItems!.map((i) => i.id)).toEqual([ID(3), ID(4)]);
  });

  it("does nothing before a first save or across a type change", () => {
    const next = values({});
    expect(adoptIds(next, null, null)).toBe(next);
    expect(adoptIds(values({ type: "essay" }), values({}), saved)).toEqual(
      values({ type: "essay" }),
    );
  });
});

describe("the live outline warning", () => {
  it("clears when the editor unmounts (QC-R6B-10)", () => {
    const reports: boolean[] = [];
    const { rerender, unmount } = renderHook(
      ({ unkeyed }) => useReportUnkeyed(unkeyed, (next) => reports.push(next)),
      { initialProps: { unkeyed: true } },
    );
    rerender({ unkeyed: true });
    expect(reports).toEqual([true]);
    unmount();
    expect(reports).toEqual([true, false]);
  });

  it("reads a group member's input as well as a form value (QC-R6B-11)", () => {
    expect(
      draftUnkeyed({
        type: "matching",
        options: [{ isCorrect: false }, { isCorrect: false }],
        matchItems: [{ correctOption: 0 }, { correctOption: 0 }],
      }),
    ).toBe(true);
    expect(
      draftUnkeyed({
        type: "multiple_choice",
        selectCount: 2,
        options: [{ isCorrect: true }, { isCorrect: true }, { isCorrect: true }],
      }),
    ).toBe(true);
  });
});

describe("a group member's option order across a type switch (QC-R6B-10, N9)", () => {
  it("starts from shuffle when a member comes back from matching to a choice type", async () => {
    const member = newGroupQuestion(t);
    const initial: GroupBundle = { ...emptyGroup("Reading"), questions: [member] };
    initial.group.members = [{ questionId: member.id, optionOrder: "fixed" }];
    const latest = { current: initial };
    function Host() {
      const [bundle, setBundle] = useState(initial);
      return (
        <GroupComposer
          bundle={bundle}
          assets={[]}
          onChange={(next) => {
            latest.current = next;
            setBundle(next);
          }}
          onAsset={() => undefined}
          onRefresh={() => undefined}
        />
      );
    }
    render(
      <QueryClientProvider client={new QueryClient()}>
        <Host />
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    await user.click(await screen.findByRole("button", { name: /^1Câu hỏi mới/ }));
    await user.click(await screen.findByRole("tab", { name: "Nối" }));
    await waitFor(() =>
      expect(latest.current.group.members[0]!.optionOrder).toBe("fixed"),
    );
    await user.click(screen.getByRole("tab", { name: "Một đáp án" }));
    await waitFor(() =>
      expect(latest.current.group.members[0]!.optionOrder).toBe("shuffle"),
    );
  }, 15_000);
});
