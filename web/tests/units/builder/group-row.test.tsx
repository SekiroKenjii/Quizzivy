import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import TestBuilderPage from "@/features/tests/pages/teacher/TestBuilderPage";
import { OutlineTree } from "@/features/tests/components/OutlineTree";
import type { OutlineSection } from "@/features/tests/outline";
import { sharedContentLine } from "@/features/tests/sharedContentLine";
import type { GroupBundle, StoredGroup } from "@/features/question-groups/api";
import type { GroupMaterial } from "@/features/question-groups/model";
import type { components } from "@/lib/api/schema";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { viewport } from "@tests/support/viewport";
import i18n from "@/lib/i18n";

globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

const BASE = "http://localhost:8080";
const TEST_ID = "018f0000-0000-7000-8000-0000000002a1";
const SECTION_ID = "018f0000-0000-7000-8000-0000000002c1";
const QUESTION_ID = "018f0000-0000-7000-8000-0000000002b1";
const GROUP_ID = "018f0000-0000-7000-8000-0000000002d1";
const MEMBER_ID = "018f0000-0000-7000-8000-0000000002e1";
const AUDIO_ID = "018f0000-0000-7000-8000-0000000002f1";
const SECOND_AUDIO_ID = "018f0000-0000-7000-8000-0000000002f2";
const STAMP = "2026-01-01T00:00:00Z";
const TITLE = "Why cities need green space";

function passage(title: string, gaps = 0): GroupMaterial {
  return {
    id: crypto.randomUUID(),
    title,
    content: {
      format: "semantic_v1",
      blocks: [
        {
          type: "paragraph",
          content: [
            { type: "text", text: "Parks cool the streets.", marks: [] },
            ...Array.from({ length: gaps }, (_, index) => ({
              type: "gap" as const,
              id: `g${index + 1}`,
              label: String(index + 1),
            })),
          ],
        },
      ],
    },
    gaps: [],
  };
}

function recording(assetIds: string[], text = false): GroupMaterial {
  return {
    id: crypto.randomUUID(),
    title: "Listening",
    content: {
      format: "semantic_v1",
      blocks: [
        {
          type: "paragraph",
          content: text ? [{ type: "text", text: "Listen twice.", marks: [] }] : [],
        },
        ...assetIds.map((assetId) => ({
          type: "audio" as const,
          assetId,
          label: "Track",
        })),
      ],
    },
    gaps: [],
  };
}

function bundle(
  stimuli: GroupMaterial[],
  maxPlays: (number | null)[] = [],
  title = "Reading club",
): GroupBundle {
  const assetIds = stimuli.flatMap((material) =>
    material.content.format === "semantic_v1"
      ? material.content.blocks.flatMap((block) =>
          block.type === "audio" ? [block.assetId] : [],
        )
      : [],
  );
  return {
    group: {
      id: GROUP_ID,
      title,
      members: [{ questionId: MEMBER_ID, optionOrder: "shuffle" }],
      stimuli,
      recordings: assetIds.map((assetId, index) => ({
        id: crypto.randomUUID(),
        assetId,
        policy: {
          maxPlays: maxPlays[index] ?? null,
          allowSeek: false,
          showTranscriptAfterSubmit: false,
          allowPause: true,
        },
        transcript: null,
      })),
    },
    questions: [
      {
        id: MEMBER_ID,
        input: {
          type: "single_choice",
          level: null,
          skill: null,
          prompt: "What do parks do?",
          points: 1,
          options: [
            { text: "Cool the streets", isCorrect: true },
            { text: "Heat the streets", isCorrect: false },
          ],
          blanks: [],
          tags: [],
        },
      },
    ],
  };
}

const files = new Map([
  [AUDIO_ID, "lecture.mp3"],
  [SECOND_AUDIO_ID, "interview.mp3"],
]);

function line(value: GroupBundle, language = "en") {
  return sharedContentLine(value, files, i18n.getFixedT(language));
}

describe("the shared-content line's text", () => {
  it("names one passage, and adds its gaps only when it has some", () => {
    expect(line(bundle([passage(TITLE)]))).toEqual({
      kind: "passage",
      text: `Passage · ${TITLE}`,
    });
    expect(line(bundle([passage(TITLE, 1)]))?.text).toBe(`Passage · ${TITLE} · 1 gap`);
    expect(line(bundle([passage(TITLE, 3)]))?.text).toBe(`Passage · ${TITLE} · 3 gaps`);
    expect(line(bundle([passage("  ")]))?.text).toBe("Passage · Untitled");
    expect(line(bundle([passage(TITLE, 2)]), "vi")?.text).toBe(
      `Bài đọc · ${TITLE} · 2 chỗ trống`,
    );
  });

  it("names one recording's file and its play limit, in the singular for one", () => {
    expect(line(bundle([recording([AUDIO_ID])], [2]))).toEqual({
      kind: "audio",
      text: "Audio · lecture.mp3 · 2 plays",
    });
    expect(line(bundle([recording([AUDIO_ID])], [1]))?.text).toBe(
      "Audio · lecture.mp3 · 1 play",
    );
    expect(line(bundle([recording([AUDIO_ID])], [null]))?.text).toBe(
      "Audio · lecture.mp3 · unlimited plays",
    );
    expect(line(bundle([recording([AUDIO_ID])], [null]), "vi")?.text).toBe(
      "Bài nghe · lecture.mp3 · không giới hạn lượt nghe",
    );
  });

  it("leaves out a file name it does not know", () => {
    const unknown = "018f0000-0000-7000-8000-0000000002f9";
    expect(line(bundle([recording([unknown])], [3]))?.text).toBe("Audio · 3 plays");
  });

  it("counts materials and recordings when there is more than one, or both kinds", () => {
    expect(line(bundle([passage(TITLE), recording([AUDIO_ID])], [2]))).toEqual({
      kind: "materials",
      text: "2 materials · 1 recording",
    });
    expect(line(bundle([recording([AUDIO_ID], true)], [2]))?.text).toBe(
      "1 material · 1 recording",
    );
    expect(line(bundle([recording([AUDIO_ID, SECOND_AUDIO_ID])], [2, 2]))?.text).toBe(
      "1 material · 2 recordings",
    );
    expect(line(bundle([passage(TITLE), passage("Maps")]))?.text).toBe(
      "2 materials · 0 recordings",
    );
    expect(line(bundle([passage(TITLE), passage("Maps")]), "vi")?.text).toBe(
      "2 ngữ liệu · 0 bản ghi âm",
    );
  });

  it("has no line for a group with no shared content", () => {
    expect(line(bundle([]))).toBeNull();
  });
});

function sections(): OutlineSection[] {
  return [
    {
      id: "s1",
      title: "Đọc hiểu",
      instructions: null,
      questionIds: [],
      units: [{ kind: "group", id: GROUP_ID }],
    },
    { id: "s2", title: "Viết", instructions: null, questionIds: [] },
  ];
}

function renderTree(value: GroupBundle, initial: OutlineSection[] = sections()) {
  const handlers = {
    onChange: vi.fn(),
    onSelectGroup: vi.fn(),
    onRenameGroup: vi.fn(),
    onRemoveGroup: vi.fn(),
  };
  function Harness() {
    const [outline, setOutline] = useState(initial);
    return (
      <OutlineTree
        sections={outline}
        questions={new Map()}
        groups={new Map([[GROUP_ID, value]])}
        groupFiles={files}
        selectedId={null}
        creating={false}
        onCreateQuestion={vi.fn()}
        onPickFromBank={vi.fn()}
        onAddSection={vi.fn()}
        onSelect={vi.fn()}
        onSelectGroup={handlers.onSelectGroup}
        onRenameGroup={handlers.onRenameGroup}
        onRemoveGroup={handlers.onRemoveGroup}
        onChange={(next) => {
          handlers.onChange(next);
          setOutline(next);
        }}
      />
    );
  }
  render(<Harness />);
  return { user: userEvent.setup(), ...handlers };
}

const groupMenu = () =>
  screen.getByRole("button", { name: "Thao tác với nhóm Reading club" });

describe("the group's row in the outline", () => {
  it("shows the line under the open group in the instructions line's geometry", async () => {
    const { user, onSelectGroup } = renderTree(bundle([passage(TITLE)]));

    const shared = screen.getByRole("button", { name: `Bài đọc · ${TITLE}` });
    expect(shared).toHaveClass(
      "mb-0.5",
      "ml-6.5",
      "rounded-[6px]",
      "px-1.5",
      "py-1",
      "text-xs",
      "text-muted-fg",
    );
    expect(shared.querySelector("svg")).toHaveClass("size-3.25", "lucide-file-text");
    expect(within(shared).getByText(`Bài đọc · ${TITLE}`)).toHaveClass("truncate");

    await user.click(shared);
    expect(onSelectGroup).toHaveBeenCalledWith(GROUP_ID);

    await user.click(
      screen.getByRole("button", { name: "Reading club", expanded: true }),
    );
    expect(screen.queryByRole("button", { name: `Bài đọc · ${TITLE}` })).toBeNull();
  });

  it("draws the audio icon for a recording, and no line without shared content", () => {
    renderTree(bundle([recording([AUDIO_ID])], [2]));
    expect(
      screen
        .getByRole("button", { name: "Bài nghe · lecture.mp3 · 2 lượt nghe" })
        .querySelector("svg"),
    ).toHaveClass("lucide-audio-lines");
  });

  it("has no line when the group holds no material", () => {
    renderTree(bundle([]));
    expect(screen.queryByRole("button", { name: /^Bài đọc|^Bài nghe/ })).toBeNull();
  });

  it("puts the row's actions in a menu, in the export's order, ends disabled", async () => {
    const { user } = renderTree(bundle([passage(TITLE)]));
    const trigger = groupMenu();

    await user.click(trigger);
    const menu = await screen.findByRole("menu");
    const items = within(menu).getAllByRole("menuitem");
    expect(items.map((item) => item.textContent)).toEqual([
      "Đổi tên",
      "Nội dung dùng chung",
      "Di chuyển lên",
      "Di chuyển xuống",
      "Xoá nhóm",
    ]);
    expect(items[1]!.querySelector("svg")).toHaveClass("lucide-panel-left");
    expect(trigger.closest(".overflow-y-auto")?.contains(menu)).toBe(false);
    expect(items[2]).toHaveAttribute("aria-disabled", "true");
    expect(items[3]).not.toHaveAttribute("aria-disabled");
    expect(screen.queryByRole("button", { name: /Đưa nhóm|Xoá nhóm/ })).toBeNull();
  });

  it("disables Move down on the outline's last unit", async () => {
    const { user } = renderTree(bundle([passage(TITLE)]), [
      { id: "s0", title: "Nghe", instructions: null, questionIds: [] },
      ...sections().slice(0, 1),
    ]);

    await user.click(groupMenu());
    const menu = await screen.findByRole("menu");
    expect(
      within(menu).getByRole("menuitem", { name: "Di chuyển lên" }),
    ).not.toHaveAttribute("aria-disabled");
    expect(
      within(menu).getByRole("menuitem", { name: "Di chuyển xuống" }),
    ).toHaveAttribute("aria-disabled", "true");
  });

  it("moves, removes, renames and opens the group from its items", async () => {
    const { user, onChange, onRemoveGroup, onRenameGroup, onSelectGroup } = renderTree(
      bundle([passage(TITLE)]),
    );

    await user.click(groupMenu());
    await user.click(
      await screen.findByRole("menuitem", { name: "Nội dung dùng chung" }),
    );
    expect(onSelectGroup).toHaveBeenCalledWith(GROUP_ID);

    await user.click(groupMenu());
    await user.click(await screen.findByRole("menuitem", { name: "Đổi tên" }));
    expect(onRenameGroup).toHaveBeenCalledWith(GROUP_ID);
    expect(onChange).not.toHaveBeenCalled();

    await user.click(groupMenu());
    await user.click(await screen.findByRole("menuitem", { name: "Di chuyển xuống" }));
    expect(onChange).toHaveBeenLastCalledWith([
      expect.objectContaining({ id: "s1", units: [] }),
      expect.objectContaining({ id: "s2", units: [{ kind: "group", id: GROUP_ID }] }),
    ]);

    await user.click(groupMenu());
    await user.click(await screen.findByRole("menuitem", { name: "Xoá nhóm" }));
    expect(onRemoveGroup).toHaveBeenCalledWith(GROUP_ID, groupMenu());
  });

  it("opens and runs the menu from the keyboard, and returns focus to its trigger", async () => {
    const { user, onSelectGroup } = renderTree(bundle([passage(TITLE)]));

    groupMenu().focus();
    await user.keyboard("{Enter}");
    const menu = await screen.findByRole("menu");
    expect(within(menu).getByRole("menuitem", { name: "Đổi tên" })).toHaveFocus();
    await user.keyboard("{ArrowDown}{Enter}");
    expect(onSelectGroup).toHaveBeenCalledWith(GROUP_ID);
    await waitFor(() => expect(groupMenu()).toHaveFocus());
  });
});

const standalone: components["schemas"]["AdminQuestion"] = {
  level: null,
  skill: null,
  id: QUESTION_ID,
  type: "short_answer",
  prompt: "Viết 2–3 câu về công viên gần nhà.",
  media: null,
  audio: null,
  transcript: null,
  options: [],
  blanks: [],
  points: 5,
  explanation: null,
  sampleAnswer: null,
  tags: [],
  createdAt: STAMP,
  updatedAt: STAMP,
};

let writes: string[];
let patches: { id: string; points: number }[];

beforeEach(() => {
  viewport(1280);
  writes = [];
  patches = [];
  const stored: StoredGroup = {
    bundle: bundle([passage(TITLE)]),
    ownerSectionId: SECTION_ID,
    revision: 1,
    testUpdatedAt: STAMP,
    archivedAt: null,
    createdAt: STAMP,
    updatedAt: STAMP,
    assets: [],
    unavailableAssetIds: [],
  };
  server.events.removeAllListeners();
  server.events.on("request:start", ({ request }) => {
    if (request.method !== "GET") writes.push(`${request.method} ${request.url}`);
  });
  server.use(
    http.get(`${BASE}/teacher/tests/${TEST_ID}`, () =>
      contractJson("/teacher/tests/{id}", "get", 200, {
        skills: [],
        assignments: { live: 0, scheduled: 0, closed: 0 },
        unpublishedChanges: null,
        id: TEST_ID,
        title: "Unit 6",
        description: null,
        status: "draft",
        currentVersion: 0,
        nextVersion: 1,
        totalPoints: 6,
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
      } satisfies components["schemas"]["Test"]),
    ),
    http.get(`${BASE}/teacher/questions/:id`, () =>
      contractJson("/teacher/questions/{id}", "get", 200, standalone),
    ),
    http.get(`${BASE}/teacher/question-groups/:id`, () =>
      contractJson("/teacher/question-groups/{id}", "get", 200, stored),
    ),
    http.patch(`${BASE}/teacher/questions/:id`, async ({ params, request }) => {
      const body = (await request.json()) as { points: number };
      patches.push({ id: String(params.id), points: body.points });
      return contractJson("/teacher/questions/{id}", "patch", 200, {
        ...standalone,
        points: body.points,
      });
    }),
  );
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
  return userEvent.setup();
}

describe("the group's row in the builder", () => {
  it("opens the group, focuses its title and sends no request on Rename", async () => {
    const user = renderBuilder();
    await screen.findByLabelText("Điểm", undefined, { timeout: 5000 });
    await screen.findByRole("button", { name: `Bài đọc · ${TITLE}` });

    await user.click(groupMenu());
    await user.click(await screen.findByRole("menuitem", { name: "Đổi tên" }));

    const field = await screen.findByLabelText("Tên nhóm câu hỏi");
    await waitFor(() => expect(field).toHaveFocus());
    expect(field).toHaveValue("Reading club");
    expect([
      (field as HTMLInputElement).selectionStart,
      (field as HTMLInputElement).selectionEnd,
    ]).toEqual([0, "Reading club".length]);
    expect(writes).toEqual([]);
  });

  it("keeps focus in the title when Rename asks again while the group is open", async () => {
    const user = renderBuilder();
    await user.click(
      await screen.findByRole(
        "button",
        { name: `Bài đọc · ${TITLE}` },
        { timeout: 5000 },
      ),
    );
    const field = await screen.findByLabelText("Tên nhóm câu hỏi");
    expect(field).not.toHaveFocus();

    await user.click(groupMenu());
    await user.click(await screen.findByRole("menuitem", { name: "Đổi tên" }));

    await waitFor(() => expect(field).toHaveFocus());
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(field).toHaveFocus();
    expect(writes).toEqual([]);
  });

  it("returns focus to the group's menu when its removal is cancelled", async () => {
    const user = renderBuilder();
    await screen.findByRole(
      "button",
      { name: `Bài đọc · ${TITLE}` },
      { timeout: 5000 },
    );
    const trigger = groupMenu();

    await user.click(trigger);
    await user.click(await screen.findByRole("menuitem", { name: "Xoá nhóm" }));
    expect(await screen.findByRole("dialog")).toBeVisible();
    await user.keyboard("{Escape}");

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() => expect(trigger).toHaveFocus());
    expect(writes).toEqual([]);
  });

  it("saves a pending edit in the open question before the line opens the group", async () => {
    const user = renderBuilder();
    const points = await screen.findByLabelText("Điểm", undefined, { timeout: 5000 });
    await user.clear(points);
    await user.type(points, "7");
    expect(patches).toEqual([]);

    await user.click(await screen.findByRole("button", { name: `Bài đọc · ${TITLE}` }));

    expect(await screen.findByLabelText("Tên nhóm câu hỏi")).toBeVisible();
    expect(patches).toEqual([{ id: QUESTION_ID, points: 7 }]);
  });
});
