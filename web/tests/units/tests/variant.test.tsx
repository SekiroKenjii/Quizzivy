import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { act, screen, waitFor, within } from "@testing-library/react";
import { http } from "msw";
import type { AiVariantChange } from "@/features/ai/api";
import type { StoredGroup } from "@/features/question-groups/api";
import vi_ from "@/lib/i18n/locales/vi.json";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import { aiOn } from "../ai/support";
import {
  BASE,
  SECTION_ID,
  TEST_ID,
  detail,
  renderDetail,
  serveDetail,
  testFixture,
} from "./detailHarness";

const ai = vi_.ai;
const QUESTION = "018f0000-0000-7000-8000-0000000000b1";
const DUPLICATE = "018f0000-0000-7000-8000-0000000000b9";
const COPY = "018f0000-0000-7000-8000-0000000000a9";
const OPTIONS = [
  "018f0000-0000-7000-8000-0000000000d1",
  "018f0000-0000-7000-8000-0000000000d2",
];
const COPIED = [
  "018f0000-0000-7000-8000-0000000000e1",
  "018f0000-0000-7000-8000-0000000000e2",
];

function question(id: string, optionIds: readonly string[]) {
  return {
    level: null,
    skill: null,
    id,
    type: "single_choice" as const,
    prompt: "They ___ to the museum.",
    media: null,
    audio: null,
    transcript: null,
    options: [
      { id: optionIds[0]!, ordinal: 0, text: "went", isCorrect: true },
      { id: optionIds[1]!, ordinal: 1, text: "have gone", isCorrect: false },
    ],
    blanks: [],
    points: 1,
    explanation: null,
    sampleAnswer: null,
    tags: [],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  };
}

const changes: AiVariantChange[] = [
  {
    questionId: QUESTION,
    number: 1,
    kind: "reword",
    title: "They ___ to the museum.",
    before: "They ___ to the museum.",
    after: "Last weekend they ___ to the museum.",
    payload: { prompt: "Last weekend they ___ to the museum." },
  },
  {
    questionId: QUESTION,
    number: 1,
    kind: "reorder",
    title: "They ___ to the museum.",
    before: "A (correct) · B",
    after: "B · A (correct)",
    payload: { optionOrder: [OPTIONS[1]!, OPTIONS[0]!] },
  },
];

const GROUP = "018f0000-0000-7000-8000-0000000000f1";
const COPY_GROUP = "018f0000-0000-7000-8000-0000000000f9";
const MEMBERS = [
  "018f0000-0000-7000-8000-0000000000c5",
  "018f0000-0000-7000-8000-0000000000c6",
];
const COPY_MEMBERS = [
  "018f0000-0000-7000-8000-0000000000c7",
  "018f0000-0000-7000-8000-0000000000c8",
];
const MEMBER_OPTIONS = [
  "018f0000-0000-7000-8000-0000000000d5",
  "018f0000-0000-7000-8000-0000000000d6",
];
const COPY_OPTIONS = [
  "018f0000-0000-7000-8000-0000000000d7",
  "018f0000-0000-7000-8000-0000000000d8",
];

function storedGroup(
  id: string,
  members: readonly string[],
  optionIds: readonly string[],
): StoredGroup {
  return {
    bundle: {
      group: {
        id,
        title: "Reading club",
        members: members.map((questionId) => ({
          questionId,
          optionOrder: "shuffle" as const,
        })),
        stimuli: [],
        recordings: [],
      },
      questions: members.map((memberId, index) => ({
        id: memberId,
        input: {
          type: "single_choice" as const,
          level: null,
          skill: null,
          prompt: `What do parks do? (${index + 1})`,
          points: 1,
          options: [
            { id: optionIds[0]!, text: "Cool the streets", isCorrect: true },
            { id: optionIds[1]!, text: "Heat the streets", isCorrect: false },
          ],
          blanks: [],
          tags: [],
        },
      })),
    },
    ownerSectionId: SECTION_ID,
    revision: 3,
    testUpdatedAt: "2026-01-02T00:00:00Z",
    archivedAt: null,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    assets: [],
    unavailableAssetIds: [],
  };
}

let writes: string[] = [];
let questionBody: { prompt: string; options: { id: string; text: string }[] } | null;
let outlineBody: { title: string; sections: { questionIds: string[] }[] } | null;

beforeEach(() => {
  serveDetail();
  aiOn();
  writes = [];
  questionBody = null;
  outlineBody = null;
  server.use(
    http.post(`${BASE}/teacher/ai/tests/:id/variant`, () =>
      contractJson("/teacher/ai/tests/{id}/variant", "post", 200, {
        changes,
        usage: { credits: 2600, left: 15000 },
      }),
    ),
    http.get(`${BASE}/teacher/questions/:id`, () =>
      contractJson("/teacher/questions/{id}", "get", 200, question(QUESTION, OPTIONS)),
    ),
    http.post(`${BASE}/teacher/tests/:id/duplicate`, ({ params }) => {
      writes.push(`duplicate test ${String(params["id"])}`);
      return contractJson("/teacher/tests/{id}/duplicate", "post", 201, {
        ...testFixture(),
        id: COPY,
        status: "draft",
        currentVersion: 0,
        nextVersion: 1,
      });
    }),
    http.post(`${BASE}/teacher/questions/:id/duplicate`, ({ params }) => {
      writes.push(`duplicate question ${String(params["id"])}`);
      return contractJson(
        "/teacher/questions/{id}/duplicate",
        "post",
        201,
        question(DUPLICATE, COPIED),
      );
    }),
    http.patch(`${BASE}/teacher/questions/:id`, async ({ params, request }) => {
      writes.push(`update question ${String(params["id"])}`);
      questionBody = (await request.json()) as typeof questionBody;
      return contractJson(
        "/teacher/questions/{id}",
        "patch",
        200,
        question(DUPLICATE, COPIED),
      );
    }),
    http.patch(`${BASE}/teacher/tests/:id`, async ({ params, request }) => {
      writes.push(`save outline ${String(params["id"])}`);
      outlineBody = (await request.json()) as typeof outlineBody;
      return contractJson("/teacher/tests/{id}", "patch", 200, {
        ...testFixture(),
        id: COPY,
        status: "draft",
        currentVersion: 0,
        nextVersion: 1,
      });
    }),
  );
});

afterEach(() => {
  useAuthStore.setState({ accessToken: null, user: null });
});

describe("Make a variant", () => {
  it("sits before Open builder", async () => {
    renderDetail();
    const variant = await screen.findByRole("button", { name: ai.entry.variant });
    const builder = screen.getByRole("link", { name: vi_.tests.detail.openBuilder });
    expect(variant.compareDocumentPosition(builder)).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING,
    );
  });

  it("is not drawn while the server has no assistant", async () => {
    aiOn({ ai: false });
    renderDetail();
    await screen.findByRole("link", { name: vi_.tests.detail.openBuilder });
    expect(screen.queryByRole("button", { name: ai.entry.variant })).toBeNull();
  });

  it("waits for a published version, saying so", async () => {
    detail.test = testFixture({ status: "draft", currentVersion: 0, nextVersion: 1 });
    const { user } = renderDetail();
    const variant = await screen.findByRole("button", { name: ai.entry.variant });
    expect(variant).toHaveAttribute("aria-disabled", "true");
    act(() => variant.focus());
    expect(await screen.findByRole("tooltip")).toHaveTextContent(ai.needsVersion);
    await user.click(variant);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("copies the test and writes the changes to a copy of the question, never to the test's own", async () => {
    const { user, router } = renderDetail();
    await user.click(await screen.findByRole("button", { name: ai.entry.variant }));
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: ai.variant.run }));
    await user.click(
      await within(dialog).findByRole("button", {
        name: "Lưu 2 thay đổi thành bản nháp",
      }),
    );

    expect(await screen.findByText(ai.variant.saved)).toBeVisible();
    expect(writes).toEqual([
      `duplicate test ${TEST_ID}`,
      `duplicate question ${QUESTION}`,
      `update question ${DUPLICATE}`,
      `save outline ${COPY}`,
    ]);
    expect(questionBody!.prompt).toBe("Last weekend they ___ to the museum.");
    expect(questionBody!.options.map((option) => [option.id, option.text])).toEqual([
      [COPIED[1], "have gone"],
      [COPIED[0], "went"],
    ]);
    expect(outlineBody!.title).toBe("Unit 5 · Biến thể");
    expect(outlineBody!.sections).toMatchObject([
      { id: SECTION_ID, questionIds: [DUPLICATE] },
    ]);

    await user.click(screen.getByRole("button", { name: ai.variant.open }));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/teacher/tests/${COPY}`),
    );
  });

  it("writes a member's changes to the copy's own group, never to the test's group", async () => {
    const groupSections = (group: string) => [
      {
        id: SECTION_ID,
        ordinal: 0,
        title: "Đọc hiểu",
        instructions: null,
        questionIds: [],
        units: [{ kind: "group" as const, id: group }],
      },
    ];
    detail.test = testFixture({ sections: groupSections(GROUP) });
    let groupBody: { bundle: StoredGroup["bundle"]; expectedRevision: number } | null =
      null;
    server.use(
      http.post(`${BASE}/teacher/ai/tests/:id/variant`, () =>
        contractJson("/teacher/ai/tests/{id}/variant", "post", 200, {
          changes: [
            {
              questionId: MEMBERS[1]!,
              number: 2,
              kind: "reword",
              title: "What do parks do? (2)",
              before: "What do parks do? (2)",
              after: "What are parks good for?",
              payload: { prompt: "What are parks good for?" },
            },
            {
              questionId: MEMBERS[1]!,
              number: 2,
              kind: "reorder",
              title: "What do parks do? (2)",
              before: "A (correct) · B",
              after: "B · A (correct)",
              payload: { optionOrder: [MEMBER_OPTIONS[1]!, MEMBER_OPTIONS[0]!] },
            },
          ],
          usage: { credits: 2600, left: 15000 },
        }),
      ),
      http.get(`${BASE}/teacher/question-groups/:id`, ({ params }) =>
        contractJson(
          "/teacher/question-groups/{id}",
          "get",
          200,
          params["id"] === COPY_GROUP
            ? storedGroup(COPY_GROUP, COPY_MEMBERS, COPY_OPTIONS)
            : storedGroup(GROUP, MEMBERS, MEMBER_OPTIONS),
        ),
      ),
      http.put(`${BASE}/teacher/question-groups/:id`, async ({ params, request }) => {
        writes.push(`update group ${String(params["id"])}`);
        groupBody = (await request.json()) as typeof groupBody;
        return contractJson(
          "/teacher/question-groups/{id}",
          "put",
          200,
          storedGroup(COPY_GROUP, COPY_MEMBERS, COPY_OPTIONS),
        );
      }),
      http.post(`${BASE}/teacher/tests/:id/duplicate`, ({ params }) => {
        writes.push(`duplicate test ${String(params["id"])}`);
        return contractJson("/teacher/tests/{id}/duplicate", "post", 201, {
          ...testFixture({ sections: groupSections(COPY_GROUP) }),
          id: COPY,
          status: "draft",
          currentVersion: 0,
          nextVersion: 1,
        });
      }),
    );
    const { user } = renderDetail();
    await user.click(await screen.findByRole("button", { name: ai.entry.variant }));
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: ai.variant.run }));
    await user.click(
      await within(dialog).findByRole("button", {
        name: "Lưu 2 thay đổi thành bản nháp",
      }),
    );

    expect(
      await screen.findByText(ai.variant.saved, {}, { timeout: 3000 }),
    ).toBeVisible();
    expect(writes).toEqual([
      `duplicate test ${TEST_ID}`,
      `save outline ${COPY}`,
      `update group ${COPY_GROUP}`,
    ]);
    const [first, second] = groupBody!.bundle.questions;
    expect(groupBody!.expectedRevision).toBe(3);
    expect(first).toMatchObject({
      id: COPY_MEMBERS[0],
      input: { prompt: "What do parks do? (1)" },
    });
    expect(second!.id).toBe(COPY_MEMBERS[1]);
    expect(second!.input.prompt).toBe("What are parks good for?");
    expect(second!.input.options?.map((option) => [option.id, option.text])).toEqual([
      [COPY_OPTIONS[1], "Heat the streets"],
      [COPY_OPTIONS[0], "Cool the streets"],
    ]);
  });

  it("writes nothing when a change cannot be saved", async () => {
    server.use(
      http.post(`${BASE}/teacher/ai/tests/:id/variant`, () =>
        contractJson("/teacher/ai/tests/{id}/variant", "post", 200, {
          changes: [{ ...changes[0]!, payload: { prompt: " " } }],
          usage: { credits: 2600, left: 15000 },
        }),
      ),
    );
    const { user } = renderDetail();
    await user.click(await screen.findByRole("button", { name: ai.entry.variant }));
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: ai.variant.run }));
    await user.click(
      await within(dialog).findByRole("button", {
        name: "Lưu 1 thay đổi thành bản nháp",
      }),
    );
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Thay đổi cho câu 1 chưa lưu được như hiện tại.",
    );
    expect(writes).toEqual([]);
  });
});
