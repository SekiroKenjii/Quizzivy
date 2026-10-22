import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, screen, waitFor, within } from "@testing-library/react";
import { http } from "msw";
import { useEffect, useState } from "react";
import AiPanel from "@/features/ai/components/AiPanel";
import type { AiGeneratedQuestion } from "@/features/ai/api";
import { AiWriteError } from "@/features/ai/kind";
import type { AiSection, GenerateContext } from "@/features/ai/kinds/generate";
import vi_ from "@/lib/i18n/locales/vi.json";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import { aiError, aiOn, BASE, month, renderWithApp } from "./support";

const ai = vi_.ai;
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const GRAMMAR = "018f0000-0000-7000-8000-0000000000c1";
const READING = "018f0000-0000-7000-8000-0000000000c2";
const BLANK = "018f0000-0000-7000-8000-0000000000c3";
const GROUP_ID = "018f0000-0000-7000-8000-0000000000f1";
const STIMULUS_ID = "018f0000-0000-7000-8000-0000000000e1";
const PASSAGE =
  "Cities around the world are rethinking how they use open land, and parks matter.";

const grammar: AiSection = {
  id: GRAMMAR,
  title: "Ngữ pháp",
  questions: 3,
  types: ["single_choice"],
};
const reading: AiSection = { id: READING, title: "Đọc hiểu", questions: 4 };
const blank: AiSection = { id: BLANK, title: "", questions: 0 };

const standaloneQuestion: AiGeneratedQuestion = {
  type: "single_choice",
  prompt: "She ___ to school every day.",
  options: ["goes", "go"],
  answerKey: ["goes"],
  explanation: "",
};
const passageQuestion: AiGeneratedQuestion = {
  type: "true_false",
  prompt: "Cities rethink open land.",
  answerKey: ["true"],
  explanation: "",
};

function context(patch: Partial<GenerateContext> = {}): GenerateContext {
  return {
    testId: TEST_ID,
    passages: [
      {
        id: STIMULUS_ID,
        title: "Parks",
        text: PASSAGE,
        group: { id: GROUP_ID, title: "Reading club" },
        sectionId: READING,
      },
    ],
    section: grammar,
    sections: [grammar, reading, blank],
    destination: { kind: "section", title: "Ngữ pháp" },
    save: vi.fn(() => Promise.resolve()),
    ...patch,
  };
}

function Host({ start }: Readonly<{ start: GenerateContext }>) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}>
        open panel
      </button>
      {open && (
        <AiPanel
          request={{ kind: "generate", context: start }}
          onClose={() => setOpen(false)}
        />
      )}
    </>
  );
}

async function openPanel(start: GenerateContext) {
  const view = renderWithApp(<Host start={start} />);
  await view.user.click(screen.getByRole("button", { name: "open panel" }));
  return { ...view, dialog: await screen.findByRole("dialog") };
}

function answerBoth(
  bodies: Record<string, unknown>[],
  standalone: () => Response | Promise<Response>,
) {
  server.use(
    http.post(`${BASE}/teacher/ai/questions/generate`, async ({ request }) => {
      const body = (await request.json()) as Record<string, unknown>;
      bodies.push(body);
      if (body["standalone"]) return standalone();
      return contractJson("/teacher/ai/questions/generate", "post", 200, {
        items: [passageQuestion],
        usage: { credits: 150, left: 18650 },
      });
    }),
  );
}

function standaloneAnswer() {
  return contractJson("/teacher/ai/questions/generate", "post", 200, {
    items: [standaloneQuestion],
    usage: { credits: 120, left: 18530 },
  });
}

async function planTwoSections(
  dialog: HTMLElement,
  user: ReturnType<typeof renderWithApp>["user"],
) {
  await user.click(
    within(dialog).getByRole("button", { name: ai.generate.scopes.test }),
  );
  for (const title of ["Ngữ pháp", "Đọc hiểu"]) {
    const count = within(dialog).getByRole("textbox", {
      name: `${ai.generate.planCount} · ${title}`,
    });
    await user.clear(count);
    await user.type(count, "1");
  }
}

beforeEach(() => {
  aiOn();
});

afterEach(() => {
  useAuthStore.setState({ accessToken: null, user: null });
});

describe("Generate for one section", () => {
  it("moves between the two styles with the arrow keys", async () => {
    const { dialog, user } = await openPanel(context({ section: reading }));
    const passage = within(dialog).getByRole("radio", { name: /^Theo bài đọc/ });
    expect(passage).toBeChecked();
    passage.focus();
    await user.keyboard("{ArrowRight}");
    expect(within(dialog).getByRole("radio", { name: /^Câu độc lập/ })).toBeChecked();
    await user.keyboard("{ArrowLeft}");
    expect(within(dialog).getByRole("radio", { name: /^Theo bài đọc/ })).toBeChecked();
  });

  it("starts standalone in a section with no passage, with its types, and keeps the count from 1 to 8", async () => {
    const { dialog, user } = await openPanel(context());
    expect(within(dialog).getByRole("radio", { name: /^Câu độc lập/ })).toBeChecked();
    const types = within(dialog).getByRole("group", { name: ai.generate.types });
    expect(
      within(types)
        .getAllByRole("button", { pressed: true })
        .map((b) => b.textContent),
    ).toEqual([vi_.questionEditor.type.single_choice]);
    const count = within(dialog).getByRole("textbox", { name: ai.generate.count });
    await user.clear(count);
    expect(within(dialog).getByText(ai.generate.needCount)).toBeVisible();
    expect(
      within(dialog).getByRole("button", { name: ai.generate.run }),
    ).toBeDisabled();
    await user.type(count, "9");
    expect(count).toHaveValue("8");
    expect(
      within(dialog).getByRole("button", { name: ai.generate.more }),
    ).toBeDisabled();
  });

  it("names the ceiling, and refuses to start when the teacher has less left", async () => {
    aiOn({ usage: month({ left: 150 }) });
    const { dialog } = await openPanel(context());
    expect(
      await within(dialog).findByText(
        "Lượt này cần tối đa 200 tín dụng, bạn còn 150. Hãy xin thêm trong Cài đặt.",
      ),
    ).toBeVisible();
    expect(
      within(dialog).getByRole("button", { name: ai.generate.run }),
    ).toBeDisabled();
  });

  it("asks for another section when the one it wrote for is deleted before the questions are kept", async () => {
    server.use(
      http.post(`${BASE}/teacher/ai/questions/generate`, () => standaloneAnswer()),
    );
    const save = vi.fn(() => Promise.resolve());
    const page: { drop: (() => void) | null } = { drop: null };
    function Shrinking() {
      const [sections, setSections] = useState([grammar, reading]);
      useEffect(() => {
        page.drop = () => setSections([reading]);
      }, []);
      return (
        <AiPanel
          request={{
            kind: "generate",
            context: context({ passages: [], sections, save }),
          }}
          onClose={() => undefined}
        />
      );
    }
    const { user } = renderWithApp(<Shrinking />);
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: ai.generate.run }));
    await within(dialog).findByRole("button", { name: "Thêm 1 câu vào Ngữ pháp" });

    act(() => page.drop?.());
    expect(
      within(dialog).getByText(
        "«Ngữ pháp» đã bị xoá nên chưa thêm được câu nào. Hãy chọn một phần khác.",
      ),
    ).toBeVisible();
    expect(
      within(dialog).getByRole("button", { name: "Thêm 1 câu vào Ngữ pháp" }),
    ).toBeDisabled();
    await user.click(
      within(dialog).getByRole("combobox", { name: ai.generate.chooseSection }),
    );
    await user.click(await screen.findByRole("option", { name: "Đọc hiểu" }));
    await user.click(
      within(dialog).getByRole("button", { name: "Thêm 1 câu vào Đọc hiểu" }),
    );
    expect(save).toHaveBeenCalledWith(
      [{ question: standaloneQuestion, level: "b2" }],
      ai.draftTag,
      { kind: "section", id: READING, title: "Đọc hiểu" },
    );
  });
});

describe("Generate for the whole test", () => {
  it("plans each section, says which cannot be written for, and names the total and the ceiling", async () => {
    const { dialog, user } = await openPanel(context());
    await user.click(
      within(dialog).getByRole("button", { name: ai.generate.scopes.test }),
    );
    const rows = within(dialog).getAllByRole("listitem");
    expect(rows.map((row) => row.getAttribute("aria-label"))).toEqual([
      "Ngữ pháp",
      "Đọc hiểu",
      ai.generate.untitled,
    ]);
    expect(within(rows[2]!).getByText(ai.generate.planNeedsContext)).toBeVisible();
    expect(within(dialog).getByText(ai.generate.planEmpty)).toBeVisible();

    const reading = within(rows[1]!).getByRole("textbox", {
      name: `${ai.generate.planCount} · Đọc hiểu`,
    });
    await user.clear(reading);
    await user.type(reading, "2");
    expect(
      within(rows[1]!).getByText("1 bài đọc × 2 câu = 2 câu, mỗi bài vào nhóm của nó"),
    ).toBeVisible();
    const grammarCount = within(rows[0]!).getByRole("textbox", {
      name: `${ai.generate.planCount} · Ngữ pháp`,
    });
    await user.clear(grammarCount);
    await user.type(grammarCount, "3");
    expect(within(dialog).getByText("Tổng 5 câu · 2 phần")).toBeVisible();
    expect(within(dialog).getByRole("button", { name: "Tạo 5 câu" })).toBeEnabled();
    expect(
      await within(dialog).findByText(
        "Tối đa 400 tín dụng · còn 18.800 trong tháng này",
      ),
    ).toBeVisible();
  });

  it("refuses a plan of more than five calls, naming why", async () => {
    const passages = Array.from({ length: 6 }, (_, index) => ({
      id: `018f0000-0000-7000-8000-00000000010${index}`,
      title: `Bài ${index + 1}`,
      text: PASSAGE,
      group: { id: `018f0000-0000-7000-8000-00000000020${index}`, title: "Nhóm" },
      sectionId: READING,
    }));
    const { dialog, user } = await openPanel(context({ passages }));
    await user.click(
      within(dialog).getByRole("button", { name: ai.generate.scopes.test }),
    );
    const count = within(dialog).getByRole("textbox", {
      name: `${ai.generate.planCount} · Đọc hiểu`,
    });
    await user.clear(count);
    await user.type(count, "1");
    expect(within(dialog).getByRole("alert")).toHaveTextContent(/kế hoạch này có 6/);
    expect(within(dialog).getByText(ai.generate.overLimit)).toBeVisible();
    expect(within(dialog).getByRole("button", { name: "Tạo 6 câu" })).toBeDisabled();
  });

  it("refuses a plan the teacher's credits cannot cover", async () => {
    aiOn({ usage: month({ left: 300 }) });
    const { dialog, user } = await openPanel(context());
    await planTwoSections(dialog, user);
    expect(
      await within(dialog).findByText(
        "Lượt này cần tối đa 400 tín dụng, bạn còn 300. Hãy bớt kế hoạch, hoặc xin thêm trong Cài đặt.",
      ),
    ).toBeVisible();
    expect(within(dialog).getByRole("button", { name: "Tạo 2 câu" })).toBeDisabled();
  });

  it("calls once per section, keeps what one returned when another fails, and tries the failed one again alone", async () => {
    const bodies: Record<string, unknown>[] = [];
    let standaloneCalls = 0;
    answerBoth(bodies, () => {
      standaloneCalls += 1;
      return standaloneCalls === 1
        ? aiError(503, "AI_UNAVAILABLE")
        : standaloneAnswer();
    });
    const start = context();
    const { dialog, user } = await openPanel(start);
    await planTwoSections(dialog, user);
    await user.click(within(dialog).getByRole("button", { name: "Tạo 2 câu" }));

    const failed = await within(dialog).findByRole("region", { name: "Ngữ pháp" });
    expect(within(failed).getByText(ai.generate.sectionFailed)).toBeVisible();
    expect(within(failed).getByText("Thử lại dùng tối đa 200 tín dụng.")).toBeVisible();
    const done = within(dialog).getByRole("region", { name: "Đọc hiểu" });
    expect(within(done).getByText("Cities rethink open land.")).toBeVisible();
    expect(bodies).toHaveLength(2);
    expect(bodies).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          standalone: true,
          testId: TEST_ID,
          sectionId: GRAMMAR,
          count: 1,
          types: ["single_choice"],
        }),
        expect.objectContaining({ testId: TEST_ID, stimulusId: STIMULUS_ID, count: 1 }),
      ]),
    );

    await user.click(within(failed).getByRole("button", { name: ai.tryAgain }));
    expect(
      await within(failed).findByText("She ___ to school every day."),
    ).toBeVisible();
    expect(within(failed).queryByText(ai.generate.sectionFailed)).toBeNull();
    expect(bodies).toHaveLength(3);

    await user.click(
      within(dialog).getByRole("button", { name: "Thêm 2 câu vào 2 phần" }),
    );
    expect(start.save).toHaveBeenNthCalledWith(
      1,
      [{ question: standaloneQuestion, level: "b2" }],
      ai.draftTag,
      { kind: "section", id: GRAMMAR, title: "Ngữ pháp" },
    );
    expect(start.save).toHaveBeenNthCalledWith(
      2,
      [{ question: passageQuestion, level: "b2" }],
      ai.draftTag,
      { kind: "group", id: GROUP_ID, title: "Reading club" },
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("keeps only what was not saved when a later section refuses the write", async () => {
    const bodies: Record<string, unknown>[] = [];
    answerBoth(bodies, standaloneAnswer);
    const save = vi
      .fn<GenerateContext["save"]>()
      .mockResolvedValueOnce(undefined)
      .mockRejectedValueOnce(new AiWriteError("Nhóm đã bị xoá."));
    const { dialog, user } = await openPanel(context({ save }));
    await planTwoSections(dialog, user);
    await user.click(within(dialog).getByRole("button", { name: "Tạo 2 câu" }));
    await user.click(
      await within(dialog).findByRole("button", { name: "Thêm 2 câu vào 2 phần" }),
    );

    expect(
      await within(dialog).findByText(
        "Chưa thêm được vào Đọc hiểu: Nhóm đã bị xoá. Các phần khác đã được thêm.",
      ),
    ).toBeVisible();
    expect(within(dialog).queryByText("She ___ to school every day.")).toBeNull();
    expect(within(dialog).getByText("Cities rethink open land.")).toBeVisible();
    expect(
      within(dialog).getByRole("button", { name: "Thêm 1 câu vào Đọc hiểu" }),
    ).toBeEnabled();
  });
  it("saves the later sections when an earlier one refuses the write", async () => {
    const bodies: Record<string, unknown>[] = [];
    answerBoth(bodies, standaloneAnswer);
    const save = vi
      .fn<GenerateContext["save"]>()
      .mockRejectedValueOnce(new AiWriteError("Phần đã bị khoá."))
      .mockResolvedValueOnce(undefined);
    const { dialog, user } = await openPanel(context({ save }));
    await planTwoSections(dialog, user);
    await user.click(within(dialog).getByRole("button", { name: "Tạo 2 câu" }));
    await user.click(
      await within(dialog).findByRole("button", { name: "Thêm 2 câu vào 2 phần" }),
    );

    expect(
      await within(dialog).findByText(
        "Chưa thêm được vào Ngữ pháp: Phần đã bị khoá. Các phần khác đã được thêm.",
      ),
    ).toBeVisible();
    expect(save).toHaveBeenCalledTimes(2);
    expect(within(dialog).queryByText("Cities rethink open land.")).toBeNull();
    expect(within(dialog).getByText("She ___ to school every day.")).toBeVisible();
  });

  it("asks where a removed section's questions go, and saves every section once one is chosen", async () => {
    const bodies: Record<string, unknown>[] = [];
    answerBoth(bodies, standaloneAnswer);
    const save = vi.fn<GenerateContext["save"]>(() => Promise.resolve());
    const page: { drop: (() => void) | null } = { drop: null };
    function Shrinking() {
      const [sections, setSections] = useState([grammar, reading, blank]);
      useEffect(() => {
        page.drop = () => setSections([reading, blank]);
      }, []);
      return (
        <AiPanel
          request={{ kind: "generate", context: context({ sections, save }) }}
          onClose={() => undefined}
        />
      );
    }
    const { user } = renderWithApp(<Shrinking />);
    const dialog = await screen.findByRole("dialog");
    await planTwoSections(dialog, user);
    await user.click(within(dialog).getByRole("button", { name: "Tạo 2 câu" }));
    const add = await within(dialog).findByRole("button", {
      name: "Thêm 2 câu vào 2 phần",
    });

    act(() => page.drop?.());
    expect(
      within(dialog).getByText(
        "«Ngữ pháp» đã bị xoá nên chưa thêm được câu nào. Hãy chọn một phần khác.",
      ),
    ).toBeVisible();
    expect(add).toBeDisabled();
    await user.click(
      within(dialog).getByRole("combobox", { name: ai.generate.chooseSection }),
    );
    await user.click(await screen.findByRole("option", { name: "Đọc hiểu" }));
    await user.click(add);
    expect(save).toHaveBeenNthCalledWith(
      1,
      [{ question: standaloneQuestion, level: "b2" }],
      ai.draftTag,
      { kind: "section", id: READING, title: "Đọc hiểu" },
    );
    expect(save).toHaveBeenNthCalledWith(
      2,
      [{ question: passageQuestion, level: "b2" }],
      ai.draftTag,
      { kind: "group", id: GROUP_ID, title: "Reading club" },
    );
  });
});

describe("True / False / Not given", () => {
  const tfngQuestion: AiGeneratedQuestion = {
    type: "tfng",
    prompt: "Parks are a kind of road.",
    options: ["True", "False", "Not given"],
    answerKey: ["Not given"],
    explanation: "",
  };
  const tfngChip = vi_.questionEditor.type.tfng;

  function chips(scope: HTMLElement) {
    return within(within(scope).getByRole("group", { name: ai.generate.types }))
      .getAllByRole("button")
      .map((button) => button.textContent);
  }

  it("is offered from a passage, names its options in the reader's language, and is never sent standalone", async () => {
    const bodies: Record<string, unknown>[] = [];
    server.use(
      http.post(`${BASE}/teacher/ai/questions/generate`, async ({ request }) => {
        bodies.push((await request.json()) as Record<string, unknown>);
        return contractJson("/teacher/ai/questions/generate", "post", 200, {
          items: [tfngQuestion],
          usage: { credits: 150, left: 18650 },
        });
      }),
    );
    const { dialog, user } = await openPanel(context({ section: reading }));
    expect(chips(dialog)).toContain(tfngChip);
    await user.click(within(dialog).getByRole("button", { name: tfngChip }));
    await user.click(within(dialog).getByRole("radio", { name: /^Câu độc lập/ }));
    expect(chips(dialog)).not.toContain(tfngChip);
    await user.click(within(dialog).getByRole("radio", { name: /^Theo bài đọc/ }));
    expect(
      within(dialog).getByRole("button", { name: tfngChip, pressed: true }),
    ).toBeVisible();
    await user.click(within(dialog).getByRole("button", { name: ai.generate.run }));
    expect(await within(dialog).findByText("Parks are a kind of road.")).toBeVisible();
    for (const label of ["Đúng", "Sai", "Không có thông tin"])
      expect(within(dialog).getByText(label)).toBeVisible();
    expect(bodies[0]).toEqual(
      expect.objectContaining({
        stimulusId: STIMULUS_ID,
        types: expect.arrayContaining(["tfng"]),
      }),
    );
  });

  it("is dropped from a standalone run's request even when it was chosen from a passage", async () => {
    const bodies: Record<string, unknown>[] = [];
    answerBoth(bodies, standaloneAnswer);
    const { dialog, user } = await openPanel(context({ section: reading }));
    await user.click(within(dialog).getByRole("button", { name: tfngChip }));
    await user.click(within(dialog).getByRole("radio", { name: /^Câu độc lập/ }));
    await user.click(within(dialog).getByRole("button", { name: ai.generate.run }));
    await waitFor(() => expect(bodies).toHaveLength(1));
    expect(bodies[0]).toEqual(expect.objectContaining({ standalone: true }));
    expect(bodies[0]!["types"]).not.toContain("tfng");
  });

  it("keeps True / False / Not given chosen while other chips change in a standalone run", async () => {
    const { dialog, user } = await openPanel(context({ section: reading }));
    await user.click(within(dialog).getByRole("button", { name: tfngChip }));
    await user.click(within(dialog).getByRole("radio", { name: /^Câu độc lập/ }));
    const fill = vi_.questionEditor.type.fill_blank;
    await user.click(within(dialog).getByRole("button", { name: fill }));
    await user.click(within(dialog).getByRole("button", { name: fill }));
    await user.click(within(dialog).getByRole("radio", { name: /^Theo bài đọc/ }));
    expect(
      within(dialog).getByRole("button", { name: tfngChip, pressed: true }),
    ).toBeVisible();
    expect(
      within(dialog).getByRole("button", { name: fill, pressed: false }),
    ).toBeVisible();
  });

  it("asks for another type when a standalone run has only True / False / Not given", async () => {
    const tfngOnly: AiSection = { ...grammar, types: ["tfng"] };
    const { dialog } = await openPanel(
      context({ section: tfngOnly, sections: [tfngOnly, reading, blank] }),
    );
    expect(within(dialog).getByRole("radio", { name: /^Câu độc lập/ })).toBeChecked();
    expect(chips(dialog)).not.toContain(tfngChip);
    expect(within(dialog).getByText(ai.generate.needType)).toBeVisible();
  });

  it("offers it in the plan only to a section written per passage, and drops it from a standalone row", async () => {
    const bodies: Record<string, unknown>[] = [];
    answerBoth(bodies, standaloneAnswer);
    const mixed: AiSection = { ...grammar, types: ["single_choice", "tfng"] };
    const withTfng: AiSection = { ...reading, types: ["tfng"] };
    const { dialog, user } = await openPanel(
      context({ section: mixed, sections: [mixed, withTfng, blank] }),
    );
    await planTwoSections(dialog, user);
    const rows = within(dialog).getAllByRole("listitem");
    expect(chips(rows[0]!)).not.toContain(tfngChip);
    expect(chips(rows[1]!)).toContain(tfngChip);
    await user.click(within(dialog).getByRole("button", { name: "Tạo 2 câu" }));
    await waitFor(() => expect(bodies).toHaveLength(2));
    expect(bodies).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ standalone: true, types: ["single_choice"] }),
        expect.objectContaining({ stimulusId: STIMULUS_ID, types: ["tfng"] }),
      ]),
    );
  });
});
