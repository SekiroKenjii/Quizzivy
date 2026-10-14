import { useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import TestBuilderPage from "@/features/tests/pages/teacher/TestBuilderPage";
import {
  OutlineTree,
  type OutlineQuestion,
} from "@/features/tests/components/OutlineTree";
import type { OutlineSection } from "@/features/tests/outline";
import type { QuestionMove } from "@/features/tests/outlineUnits";
import type { GroupBundle, StoredGroup } from "@/features/question-groups/api";
import type { components } from "@/lib/api/schema";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import {
  moveFailure,
  moveQuestionHandler,
  type GroupMoveDraft,
} from "@tests/support/groupMove";
import { storedGroupBundle } from "@tests/support/storedGroupBundle";
import { viewport } from "@tests/support/viewport";
import "@/lib/i18n";

globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

const LINE = 40;

function rect(top: number, height: number): DOMRect {
  return {
    width: 300,
    height,
    top,
    left: 0,
    right: 300,
    bottom: top + height,
    x: 0,
    y: top,
    toJSON: () => ({}),
  };
}

/**
 * stackLines lays the outline out as the browser would, one 32px line every
 * 40px in reading order: a row, a section's header or a group's header. A
 * container measures the lines it holds, and a control the line it is in.
 */
function stackLines() {
  vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (
    this: Element,
  ) {
    const lines = [
      ...document.querySelectorAll(
        '[data-outline-row], [aria-label^="Kéo nhóm"], [aria-label^="Kéo để đổi vị trí phần"]',
      ),
    ].map((node) =>
      node.hasAttribute("data-outline-row") ? node : node.parentElement!,
    );
    const own = lines.findIndex((line) => line === this || line.contains(this));
    const held =
      own === -1
        ? lines.flatMap((line, index) => (this.contains(line) ? [index] : []))
        : [own];
    if (held.length === 0) return rect(0, 0);
    const top = Math.min(...held) * LINE;
    return rect(top, Math.max(...held) * LINE + 32 - top);
  });
}

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
        { text: "Yes", isCorrect: true },
        { text: "No", isCorrect: false },
      ],
      blanks: [],
      tags: [],
    },
  };
}

function bundle(
  id: string,
  title: string,
  members: GroupBundle["questions"],
  bound: string[] = [],
): GroupBundle {
  return {
    group: {
      id,
      title,
      members: members.map((item) => ({ questionId: item.id, optionOrder: "shuffle" })),
      stimuli: [
        {
          id: crypto.randomUUID(),
          title: "Parks",
          content: {
            format: "semantic_v1",
            blocks: [
              {
                type: "paragraph",
                content: [
                  { type: "text", text: "Parks cool the streets.", marks: [] },
                  ...bound.map((_, index) => ({
                    type: "gap" as const,
                    id: `g${index + 1}`,
                    label: String(index + 1),
                  })),
                ],
              },
            ],
          },
          gaps: bound.map((questionId, index) => ({
            kind: "question" as const,
            gapId: `g${index + 1}`,
            questionId,
          })),
        },
      ],
      recordings: [],
    },
    questions: members,
  };
}

afterEach(() => vi.restoreAllMocks());

describe("dragging a question across a group's boundary in the outline", () => {
  const questions = new Map<string, OutlineQuestion>([
    [
      "q1",
      { id: "q1", prompt: "Standalone one", points: 1, hasAudio: false, problem: null },
    ],
    [
      "q2",
      { id: "q2", prompt: "Standalone two", points: 1, hasAudio: false, problem: null },
    ],
  ]);

  function setup(
    initial: OutlineSection[],
    groups: GroupBundle[],
    movingQuestionId: string | null = null,
  ) {
    const moved = vi.fn<(move: QuestionMove) => void>();
    const changed = vi.fn();
    function Harness() {
      const [sections, setSections] = useState(initial);
      return (
        <OutlineTree
          sections={sections}
          questions={questions}
          groups={new Map(groups.map((item) => [item.group.id, item]))}
          selectedId={null}
          creating={false}
          onSelect={vi.fn()}
          onCreateQuestion={vi.fn()}
          onPickFromBank={vi.fn()}
          onAddSection={vi.fn()}
          onChange={(next) => {
            changed(next);
            setSections(next);
          }}
          onMove={moved}
          movingQuestionId={movingQuestionId}
        />
      );
    }
    const view = render(<Harness />);
    stackLines();
    return { ...view, user: userEvent.setup(), moved, changed };
  }

  type Units = NonNullable<OutlineSection["units"]>;
  const reading = (units: Units): OutlineSection => ({
    id: "s1",
    clientId: "s1",
    title: "Reading",
    instructions: null,
    questionIds: units
      .filter((unit) => unit.kind === "question")
      .map((unit) => unit.id),
    units,
  });
  const listening = (units: Units): OutlineSection => ({
    id: "s2",
    clientId: "s2",
    title: "Listening",
    instructions: null,
    questionIds: units
      .filter((unit) => unit.kind === "question")
      .map((unit) => unit.id),
    units,
  });

  async function pickUp(user: ReturnType<typeof userEvent.setup>, grip: HTMLElement) {
    grip.focus();
    await user.keyboard(" ");
    await waitFor(() => expect(grip).toHaveAttribute("aria-pressed", "true"));
  }

  it("drops a standalone question on a group's header as the group's first member, tinting the header while over it", async () => {
    const club = bundle("g1", "Reading club", [member("m1", "First member")]);
    const { user, moved, changed, container } = setup(
      [
        reading([
          { kind: "question", id: "q1" },
          { kind: "group", id: "g1" },
        ]),
      ],
      [club],
    );

    await pickUp(user, screen.getByRole("button", { name: "Kéo để đổi vị trí câu 1" }));
    await user.keyboard("{ArrowDown}");
    await waitFor(() => expect(screen.getByText("Thả vào nhóm")).toBeInTheDocument());
    const header = container.querySelector(
      "[data-outline-group] [data-outline-drop-into]",
    );
    expect(header).toHaveClass("bg-brand-soft");
    expect(header).toHaveTextContent("Reading club");
    await user.keyboard(" ");

    await waitFor(() => expect(moved).toHaveBeenCalledOnce());
    expect(moved).toHaveBeenCalledWith({
      questionId: "q1",
      to: { sectionIndex: 0, groupId: "g1", index: 0 },
      groupIds: ["g1"],
    });
    expect(changed).not.toHaveBeenCalled();
    expect(container.querySelector("[data-outline-drop-into]")).toBeNull();
  });

  it("drops a standalone question between two members at that member's place", async () => {
    const club = bundle("g1", "Reading club", [
      member("m1", "First member"),
      member("m2", "Second member"),
    ]);
    const { user, moved } = setup(
      [
        reading([
          { kind: "question", id: "q1" },
          { kind: "group", id: "g1" },
        ]),
      ],
      [club],
    );

    await pickUp(user, screen.getByRole("button", { name: "Kéo để đổi vị trí câu 1" }));
    await user.keyboard("{ArrowDown}{ArrowDown}{ArrowDown}");
    await waitFor(() =>
      expect(
        screen
          .getByRole("button", { name: /Second member/ })
          .closest("[data-outline-row]")
          ?.querySelector("[data-outline-drop]"),
      ).toBeInTheDocument(),
    );
    await user.keyboard(" ");

    await waitFor(() =>
      expect(moved).toHaveBeenCalledWith({
        questionId: "q1",
        to: { sectionIndex: 0, groupId: "g1", index: 1 },
        groupIds: ["g1"],
      }),
    );
  });

  it("drops a member on the next section's header as that section's first question", async () => {
    const club = bundle("g1", "Reading club", [member("m1", "First member")]);
    const { user, moved } = setup(
      [
        reading([{ kind: "group", id: "g1" }]),
        listening([{ kind: "question", id: "q2" }]),
      ],
      [club],
    );

    await pickUp(user, screen.getByRole("button", { name: "Kéo câu 1" }));
    await user.keyboard("{ArrowDown}");
    await waitFor(() =>
      expect(screen.getByText("Đã đến vị trí thả.")).toBeInTheDocument(),
    );
    await user.keyboard(" ");

    await waitFor(() =>
      expect(moved).toHaveBeenCalledWith({
        questionId: "m1",
        to: { sectionIndex: 1, groupId: null, index: 0 },
        groupIds: ["g1"],
      }),
    );
  });

  it("drops a member before a unit of another section, and into another group, touching both groups", async () => {
    const club = bundle("g1", "Reading club", [member("m1", "First member")]);
    const second = bundle("g2", "Listening club", [member("n1", "Other member")]);
    const { user, moved } = setup(
      [
        reading([{ kind: "group", id: "g1" }]),
        listening([
          { kind: "question", id: "q2" },
          { kind: "group", id: "g2" },
        ]),
      ],
      [club, second],
    );

    const grip = screen.getByRole("button", { name: "Kéo câu 1" });
    await pickUp(user, grip);
    await user.keyboard("{ArrowDown}{ArrowDown}");
    await user.keyboard(" ");
    await waitFor(() =>
      expect(moved).toHaveBeenLastCalledWith({
        questionId: "m1",
        to: { sectionIndex: 1, groupId: null, index: 0 },
        groupIds: ["g1"],
      }),
    );

    await pickUp(user, grip);
    await user.keyboard("{ArrowDown}{ArrowDown}{ArrowDown}");
    await waitFor(() => expect(screen.getByText("Thả vào nhóm")).toBeInTheDocument());
    await user.keyboard(" ");
    await waitFor(() =>
      expect(moved).toHaveBeenLastCalledWith({
        questionId: "m1",
        to: { sectionIndex: 1, groupId: "g2", index: 0 },
        groupIds: ["g1", "g2"],
      }),
    );
  });

  it("opens a closed group it drops into", async () => {
    const club = bundle("g1", "Reading club", [member("m1", "First member")]);
    const { user, moved } = setup(
      [
        reading([
          { kind: "question", id: "q1" },
          { kind: "group", id: "g1" },
        ]),
      ],
      [club],
    );
    const disclosure = screen.getByRole("button", {
      name: "Reading club",
      expanded: true,
    });
    await user.click(disclosure);
    expect(screen.queryByRole("button", { name: /First member/ })).toBeNull();

    await pickUp(user, screen.getByRole("button", { name: "Kéo để đổi vị trí câu 1" }));
    await user.keyboard("{ArrowDown}");
    await waitFor(() => expect(screen.getByText("Thả vào nhóm")).toBeInTheDocument());
    await user.keyboard(" ");

    await waitFor(() => expect(moved).toHaveBeenCalledOnce());
    expect(disclosure).toHaveAttribute("aria-expanded", "true");
  });

  it("keeps a member that a material gap is bound to in its group: its grip is disabled with the reason", async () => {
    const club = bundle(
      "g1",
      "Reading club",
      [member("m1", "Bound member"), member("m2", "Free member")],
      ["m1"],
    );
    const { user, moved } = setup([reading([{ kind: "group", id: "g1" }])], [club]);

    const bound = screen.getByRole("button", { name: "Kéo câu 1" });
    expect(bound).toHaveAttribute("aria-disabled", "true");
    expect(bound).toHaveAccessibleDescription(
      "Đang gắn với chỗ trống trong ngữ liệu chung",
    );
    expect(bound).not.toHaveAttribute("aria-roledescription");
    bound.focus();
    await user.keyboard(" ");
    await user.keyboard("{ArrowDown} ");
    expect(bound).not.toHaveAttribute("aria-pressed", "true");
    expect(moved).not.toHaveBeenCalled();

    const free = screen.getByRole("button", { name: "Kéo câu 2" });
    expect(free).toHaveAttribute("aria-disabled", "false");
    expect(free).toHaveAttribute("aria-roledescription", "sortable");
  });

  it("draws the question being moved as busy and refuses another drag meanwhile", async () => {
    const club = bundle("g1", "Reading club", [member("m1", "First member")]);
    const { user, moved } = setup(
      [
        reading([
          { kind: "question", id: "q1" },
          { kind: "group", id: "g1" },
        ]),
      ],
      [club],
      "m1",
    );

    const row = screen
      .getByRole("button", { name: /First member/ })
      .closest("[data-outline-row]");
    expect(row).toHaveAttribute("aria-busy", "true");
    expect(row).toHaveClass("opacity-40");
    expect(screen.getByText("Đang chuyển…")).toHaveAttribute("role", "status");

    const grip = screen.getByRole("button", { name: "Kéo để đổi vị trí câu 1" });
    grip.focus();
    await user.keyboard(" ");
    expect(grip).not.toHaveAttribute("aria-pressed", "true");
    expect(moved).not.toHaveBeenCalled();
  });

  it("keeps a whole group out of another group", async () => {
    const club = bundle("g1", "Reading club", [member("m1", "First member")]);
    const second = bundle("g2", "Listening club", [member("n1", "Other member")]);
    const { user, moved, changed } = setup(
      [
        reading([
          { kind: "group", id: "g1" },
          { kind: "group", id: "g2" },
        ]),
      ],
      [club, second],
    );

    await pickUp(user, screen.getByRole("button", { name: "Kéo nhóm Reading club" }));
    await user.keyboard("{ArrowDown}");
    await waitFor(() =>
      expect(screen.getByText("Đã đến vị trí thả.")).toBeInTheDocument(),
    );
    expect(document.querySelector("[data-outline-drop-into]")).toBeNull();
    await user.keyboard(" ");

    await waitFor(() => expect(changed).toHaveBeenCalledOnce());
    expect(moved).not.toHaveBeenCalled();
  });
});

const BASE = "http://localhost:8080";
const TEST_ID = "018f0000-0000-7000-8000-0000000004a1";
const SECTION_ID = "018f0000-0000-7000-8000-0000000004c1";
const QUESTION_ID = "018f0000-0000-7000-8000-0000000004b1";
const GROUP_ID = "018f0000-0000-7000-8000-0000000004d1";
const MEMBER_ID = "018f0000-0000-7000-8000-0000000004e1";
const COPY_ID = "018f0000-0000-7000-8000-0000000004e2";
const STAMP = "2026-01-01T00:00:00.000Z";
const MOVED_STAMP = "2026-01-01T00:00:01.000Z";

type UpdateInput = components["schemas"]["GroupUpdateInput"];

const standalone: components["schemas"]["AdminQuestion"] = {
  level: null,
  skill: null,
  id: QUESTION_ID,
  type: "short_answer",
  prompt: "Standalone question",
  media: null,
  audio: null,
  transcript: null,
  options: [],
  blanks: [],
  points: 1,
  explanation: null,
  sampleAnswer: null,
  tags: [],
  createdAt: STAMP,
  updatedAt: STAMP,
};

let draft: GroupMoveDraft;
let writes: string[];
let puts: UpdateInput[];

function storedGroup(value: GroupBundle): StoredGroup {
  return {
    bundle: value,
    ownerSectionId: SECTION_ID,
    revision: 1,
    testUpdatedAt: STAMP,
    archivedAt: null,
    createdAt: STAMP,
    updatedAt: STAMP,
    assets: [],
    unavailableAssetIds: [],
  };
}

beforeEach(() => {
  viewport(1280);
  writes = [];
  puts = [];
  draft = {
    test: {
      skills: [],
      assignments: { live: 0, scheduled: 0, closed: 0 },
      unpublishedChanges: null,
      id: TEST_ID,
      title: "Unit 7",
      description: null,
      status: "draft",
      currentVersion: 0,
      nextVersion: 1,
      totalPoints: 2,
      questionCount: 2,
      audioCount: 0,
      sections: [
        {
          id: SECTION_ID,
          ordinal: 0,
          title: "Đọc hiểu",
          instructions: null,
          questionIds: [QUESTION_ID],
          units: [
            { kind: "question", id: QUESTION_ID },
            { kind: "group", id: GROUP_ID },
          ],
        },
      ],
      createdAt: STAMP,
      updatedAt: STAMP,
    },
    groups: new Map([
      [
        GROUP_ID,
        storedGroup(
          bundle(GROUP_ID, "Reading club", [member(MEMBER_ID, "Group member")]),
        ),
      ],
    ]),
    copy: () => ({ ...member(COPY_ID, standalone.prompt), id: COPY_ID }),
    requests: [],
  };
  server.events.removeAllListeners();
  server.events.on("request:start", ({ request }) => {
    if (request.method !== "GET")
      writes.push(`${request.method} ${new URL(request.url).pathname}`);
  });
  server.use(
    http.get(`${BASE}/teacher/tests/${TEST_ID}`, () =>
      contractJson("/teacher/tests/{id}", "get", 200, draft.test),
    ),
    http.get(`${BASE}/teacher/questions/:id`, () =>
      contractJson("/teacher/questions/{id}", "get", 200, standalone),
    ),
    http.patch(`${BASE}/teacher/questions/:id`, async ({ request }) => {
      const body = (await request.json()) as { points: number };
      return contractJson("/teacher/questions/{id}", "patch", 200, {
        ...standalone,
        points: body.points,
      });
    }),
    http.get(`${BASE}/teacher/question-groups/:id`, ({ params }) =>
      contractJson(
        "/teacher/question-groups/{id}",
        "get",
        200,
        draft.groups.get(String(params.id))!,
      ),
    ),
    http.put(`${BASE}/teacher/question-groups/:id`, async ({ params, request }) => {
      const body = (await request.json()) as UpdateInput;
      puts.push(body);
      const current = draft.groups.get(String(params.id))!;
      const saved = {
        ...current,
        bundle: storedGroupBundle(body.bundle),
        revision: current.revision + 1,
      };
      draft.groups.set(String(params.id), saved);
      return contractJson("/teacher/question-groups/{id}", "put", 200, saved);
    }),
    moveQuestionHandler(draft),
  );
});

afterEach(() => {
  vi.useRealTimers();
});

function renderBuilder() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/teacher/tests/:id/edit", element: <TestBuilderPage /> }],
    { initialEntries: [`/teacher/tests/${TEST_ID}/edit`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
}

async function dragOntoGroup(user: ReturnType<typeof userEvent.setup>) {
  const grip = await screen.findByRole(
    "button",
    { name: "Kéo để đổi vị trí câu 1" },
    { timeout: 5000 },
  );
  await screen.findByRole("button", { name: /Group member/ }, { timeout: 5000 });
  stackLines();
  grip.focus();
  await user.keyboard(" ");
  await waitFor(() => expect(grip).toHaveAttribute("aria-pressed", "true"));
  await user.keyboard("{ArrowDown}");
  await waitFor(() => expect(screen.getByText("Thả vào nhóm")).toBeInTheDocument());
  await user.keyboard(" ");
  vi.restoreAllMocks();
}

describe("moving a question in the builder", () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });

  it("saves the open question first, then copies it into the group from the server's answer and opens the copy on the new revision", async () => {
    const user = renderBuilder();
    const points = await screen.findByLabelText("Điểm", undefined, { timeout: 5000 });
    await user.clear(points);
    await user.type(points, "3");

    await dragOntoGroup(user);

    await waitFor(() => expect(draft.requests).toHaveLength(1));
    expect(writes.slice(0, 2)).toEqual([
      `PATCH /teacher/questions/${QUESTION_ID}`,
      `POST /teacher/tests/${TEST_ID}/questions/move`,
    ]);
    expect(draft.requests[0]).toEqual({
      expectedUpdatedAt: STAMP,
      questionId: QUESTION_ID,
      to: { sectionId: SECTION_ID, groupId: GROUP_ID, index: 0 },
      expectedGroupRevisions: [{ groupId: GROUP_ID, revision: 1 }],
    });

    const title = await screen.findByLabelText("Tên nhóm câu hỏi");
    expect(
      screen.queryByRole("button", { name: "Kéo để đổi vị trí câu 1" }),
    ).toBeNull();
    const group = document.querySelector<HTMLElement>(
      `[data-outline-group="${GROUP_ID}"]`,
    )!;
    expect(
      within(group)
        .getAllByRole("button", { name: /member|question/ })
        .map((button) => button.textContent),
    ).toEqual([
      expect.stringMatching(/^1Standalone question/),
      expect.stringMatching(/^2Group member/),
    ]);
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Kéo câu 1" })).toHaveFocus(),
    );
    expect(
      screen.getByRole("button", { name: "Kéo câu 1" }).closest("[data-outline-row]"),
    ).toHaveClass("bg-secondary");

    await user.type(title, " B");
    await vi.advanceTimersByTimeAsync(1600);
    await waitFor(() => expect(puts).toHaveLength(1));
    expect(puts[0]?.expectedRevision).toBe(2);
    expect(puts[0]?.expectedTestUpdatedAt).toBe(MOVED_STAMP);
    expect(puts[0]?.bundle.group.members.map((item) => item.questionId)).toEqual([
      COPY_ID,
      MEMBER_ID,
    ]);
  });

  it("leaves the outline as it was when the move is refused, and returns focus to the dragged row", async () => {
    server.use(
      http.post(`${BASE}/teacher/tests/:id/questions/move`, () =>
        moveFailure(409, "STALE_WRITE", "Đề đã được sửa ở nơi khác."),
      ),
    );
    const user = renderBuilder();

    await dragOntoGroup(user);

    expect(await screen.findByText("Đề đã được sửa ở nơi khác.")).toBeVisible();
    const grip = screen.getByRole("button", { name: "Kéo để đổi vị trí câu 1" });
    await waitFor(() => expect(grip).toHaveFocus());
    const group = document.querySelector<HTMLElement>(
      `[data-outline-group="${GROUP_ID}"]`,
    )!;
    expect(within(group).getAllByRole("button", { name: /Group member/ })).toHaveLength(
      1,
    );
    expect(
      within(group).queryByRole("button", { name: /Standalone question/ }),
    ).toBeNull();
    expect(screen.getByLabelText("Điểm")).toBeVisible();
  });
});
