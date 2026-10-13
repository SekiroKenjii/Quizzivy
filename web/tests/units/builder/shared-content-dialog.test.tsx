import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
import TestBuilderPage from "@/features/tests/pages/teacher/TestBuilderPage";
import type { GroupBundle, StoredGroup } from "@/features/question-groups/api";
import type { GroupMaterial } from "@/features/question-groups/model";
import { groupPreview } from "@/features/question-groups/preview";
import type { LibraryAsset, MediaAsset } from "@/features/media/api";
import type { components } from "@/lib/api/schema";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { installIntersectionObserver } from "@tests/support/intersection";
import { storedGroupBundle } from "@tests/support/storedGroupBundle";
import { viewport } from "@tests/support/viewport";
import "@/lib/i18n";

globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

const BASE = "http://localhost:8080";
const TEST_ID = "018f0000-0000-7000-8000-0000000003a1";
const SECTION_ID = "018f0000-0000-7000-8000-0000000003c1";
const QUESTION_ID = "018f0000-0000-7000-8000-0000000003b1";
const GROUP_ID = "018f0000-0000-7000-8000-0000000003d1";
const MEMBER_ID = "018f0000-0000-7000-8000-0000000003e1";
const AUDIO_ID = "018f0000-0000-7000-8000-0000000003f1";
const STAMP = "2026-01-01T00:00:00Z";
const TITLE = "Why cities need green space";
const DIALOG = "Nội dung dùng chung của “Reading club”";

type UpdateInput = components["schemas"]["GroupUpdateInput"];

function passage(title: string, gap?: { bound: boolean }): GroupMaterial {
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
            ...(gap ? [{ type: "gap" as const, id: "g1", label: "1" }] : []),
          ],
        },
      ],
    },
    gaps: gap?.bound ? [{ kind: "question", gapId: "g1", questionId: MEMBER_ID }] : [],
  };
}

function bundle(stimuli: GroupMaterial[]): GroupBundle {
  return {
    group: {
      id: GROUP_ID,
      title: "Reading club",
      members: [{ questionId: MEMBER_ID, optionOrder: "shuffle" }],
      stimuli,
      recordings: [],
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

const lecture: LibraryAsset = {
  id: AUDIO_ID,
  kind: "audio",
  url: "https://example.test/lecture.mp3",
  mimeType: "audio/mpeg",
  bytes: 2_400_000,
  durationMs: 110_000,
  originalFilename: "lecture.mp3",
  createdAt: STAMP,
  displayName: "lecture.mp3",
  defaultMaxPlays: null,
  width: null,
  height: null,
  questionCount: 0,
};

const lectureFile: MediaAsset = {
  id: lecture.id,
  kind: lecture.kind,
  url: lecture.url,
  mimeType: lecture.mimeType,
  bytes: lecture.bytes,
  durationMs: lecture.durationMs ?? null,
  originalFilename: lecture.originalFilename,
  createdAt: lecture.createdAt,
};

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

beforeAll(async () => {
  await import("@/features/tests/components/OutlineTree");
});

let stored: StoredGroup;
let puts: UpdateInput[];
let answer: "ok" | "fail" | "stale";

function storeGroup(value: GroupBundle) {
  stored = {
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
  vi.useFakeTimers({ shouldAdvanceTime: true });
  viewport(1280);
  installIntersectionObserver();
  puts = [];
  answer = "ok";
  storeGroup(bundle([passage(TITLE)]));
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
    http.put(`${BASE}/teacher/question-groups/:id`, async ({ request }) => {
      const body = (await request.json()) as UpdateInput;
      puts.push(body);
      if (answer === "fail")
        return HttpResponse.json(
          { error: { code: "UNKNOWN", message: "Máy chủ đang bận." } },
          { status: 503 },
        );
      if (answer === "stale" || body.expectedRevision !== stored.revision)
        return HttpResponse.json(
          { error: { code: "STALE_WRITE", message: "Đã có thay đổi ở nơi khác." } },
          { status: 409 },
        );
      stored = {
        ...stored,
        bundle: storedGroupBundle(body.bundle),
        revision: stored.revision + 1,
        assets: body.bundle.group.recordings.length > 0 ? [lectureFile] : [],
      };
      return contractJson("/teacher/question-groups/{id}", "put", 200, stored);
    }),
    http.get(`${BASE}/teacher/media`, () =>
      contractJson("/teacher/media", "get", 200, {
        items: [lecture],
        page: 1,
        pageSize: 24,
        total: 1,
        totalBytes: 0,
        facets: { all: 1, audio: 1, image: 0, unused: 1 },
        usage: { audioBytes: 0, imageBytes: 0, quotaBytes: 5_368_709_120 },
      }),
    ),
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
  return userEvent.setup({
    applyAccept: false,
    advanceTimers: vi.advanceTimersByTime,
  });
}

async function line(name: string) {
  return screen.findByRole("button", { name }, { timeout: 5000 });
}

async function openFromLine(user: ReturnType<typeof userEvent.setup>, name: string) {
  await user.click(await line(name));
  return screen.findByRole("dialog", { name: DIALOG });
}

async function openFromMenu(user: ReturnType<typeof userEvent.setup>) {
  await screen.findByLabelText("Điểm", undefined, { timeout: 5000 });
  await user.click(
    await screen.findByRole(
      "button",
      { name: "Thao tác với nhóm Reading club" },
      { timeout: 5000 },
    ),
  );
  await user.click(
    await screen.findByRole("menuitem", { name: "Nội dung dùng chung" }),
  );
  return screen.findByRole("dialog", { name: DIALOG });
}

const titleField = (dialog: HTMLElement) =>
  within(dialog).getByRole("textbox", { name: "Tiêu đề" });

const save = (dialog: HTMLElement) =>
  within(dialog).getByRole("button", { name: "Lưu" });

describe("the Shared content dialog", () => {
  it("opens from the line over the group's editor, on the stored passage, with focus on its kind", async () => {
    const user = renderBuilder();
    const dialog = await openFromLine(user, `Bài đọc · ${TITLE}`);

    expect(
      within(dialog).getByRole("button", { name: "Bài đọc", pressed: true }),
    ).toHaveFocus();
    expect(
      within(dialog).getByRole("button", { name: "Không có", pressed: false }),
    ).toBeVisible();
    expect(
      within(dialog).getByRole("button", { name: "Bài nghe", pressed: false }),
    ).toBeVisible();
    expect(titleField(dialog)).toHaveValue(TITLE);
    expect(
      within(dialog).getByText(
        "Bài đọc hoặc bản ghi âm hiện bên cạnh mọi câu hỏi trong nhóm này. Học viên xem nội dung này bên cạnh các câu hỏi khi làm bài.",
      ),
    ).toBeVisible();
    expect(
      within(dialog).getAllByRole("button", { name: "Thêm chỗ trống" }),
    ).toHaveLength(1);
    expect(screen.getByLabelText("Tên nhóm câu hỏi")).toHaveValue("Reading club");
    expect(puts).toEqual([]);
  });

  it("saves a passage through the group's editor in exactly one request, keeps the members, and returns focus to the line", async () => {
    const user = renderBuilder();
    const dialog = await openFromLine(user, `Bài đọc · ${TITLE}`);
    const title = titleField(dialog);
    await user.clear(title);
    await user.type(title, "Parks and heat");
    await user.click(save(dialog));

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(puts).toHaveLength(1);
    const [sent] = puts;
    expect(sent?.expectedRevision).toBe(1);
    expect(sent?.expectedTestUpdatedAt).toBe(STAMP);
    expect(sent?.bundle.group.stimuli.map((item) => item.title)).toEqual([
      "Parks and heat",
    ]);
    expect(sent?.bundle.group.recordings).toEqual([]);
    expect(sent?.bundle.group.members).toEqual([
      { questionId: MEMBER_ID, optionOrder: "shuffle" },
    ]);
    expect(sent?.bundle.questions.map((item) => item.id)).toEqual([MEMBER_ID]);
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Bài đọc · Parks and heat" }),
      ).toHaveFocus(),
    );
    await vi.advanceTimersByTimeAsync(1700);
    expect(puts).toHaveLength(1);
  });

  it("saves an edit in the pane right after a dialog save, with no conflict", async () => {
    const user = renderBuilder();
    const dialog = await openFromLine(user, `Bài đọc · ${TITLE}`);
    await user.type(titleField(dialog), "!");
    await user.click(save(dialog));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());

    await user.type(screen.getByLabelText("Tên nhóm câu hỏi"), " B");

    await vi.advanceTimersByTimeAsync(1500);
    await waitFor(() => expect(puts).toHaveLength(2));
    expect(puts[1]?.expectedRevision).toBe(2);
    expect(puts[1]?.bundle.group.title).toBe("Reading club B");
    expect(puts[1]?.bundle.group.stimuli[0]?.title).toBe(`${TITLE}!`);
    await waitFor(() => expect(stored.revision).toBe(3));
    expect(screen.queryByText("Nhóm đã thay đổi ở nơi khác")).toBeNull();
  });

  it("stores a recording from Media as one audio material named after the file and one recording, whose transcript never reaches the preview", async () => {
    storeGroup(bundle([]));
    const user = renderBuilder();
    const dialog = await openFromMenu(user);
    expect(
      within(dialog).getByRole("button", { name: "Không có", pressed: true }),
    ).toHaveFocus();

    await user.click(within(dialog).getByRole("button", { name: "Bài nghe" }));
    expect(
      within(dialog).getByText("MP3 hoặc M4A, tối đa 50 MB và 5 phút"),
    ).toBeVisible();
    expect(save(dialog)).toBeDisabled();
    expect(within(dialog).getByText("Chọn một tệp âm thanh để lưu.")).toBeVisible();

    await user.click(within(dialog).getByRole("button", { name: "Từ Media" }));
    const library = await screen.findByRole("dialog", { name: "Chọn từ Media" });
    expect(
      within(library).getByRole("button", { name: /^Âm thanh/, pressed: true }),
    ).toBeVisible();
    await user.click(
      await within(library).findByRole("button", { name: /lecture\.mp3/ }),
    );
    await user.click(within(library).getByRole("button", { name: "Đính kèm" }));

    expect(await within(dialog).findByText("lecture.mp3")).toBeVisible();
    expect(
      within(dialog).getByRole("button", { name: "Thay lecture.mp3" }),
    ).toBeVisible();
    expect(
      within(dialog).getByRole("switch", { name: "Cho học sinh tạm dừng" }),
    ).toBeChecked();
    await user.type(within(dialog).getByLabelText(/Lời thoại/), "Lời thoại bí mật");
    expect(
      within(dialog).getByText("Không bắt buộc. Học viên xem sau khi nộp bài."),
    ).toBeVisible();
    await user.click(save(dialog));

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(puts).toHaveLength(1);
    const group = puts[0]!.bundle.group;
    expect(group.stimuli).toEqual([
      {
        id: expect.any(String),
        title: "lecture",
        content: {
          format: "semantic_v1",
          blocks: [{ type: "audio", assetId: AUDIO_ID, label: "Bài nghe" }],
        },
        gaps: [],
      },
    ]);
    expect(group.recordings).toEqual([
      {
        id: expect.any(String),
        assetId: AUDIO_ID,
        policy: {
          maxPlays: 2,
          allowSeek: false,
          showTranscriptAfterSubmit: true,
          allowPause: true,
        },
        transcript: "Lời thoại bí mật",
      },
    ]);
    expect(group.members).toHaveLength(1);
    expect(
      JSON.stringify(groupPreview(storedGroupBundle(puts[0]!.bundle), [lectureFile])),
    ).not.toContain("Lời thoại bí mật");
    expect(await line("Bài nghe · lecture.mp3 · 2 lượt nghe")).toBeVisible();
  });

  it("shows an upload's refusal and saves nothing", async () => {
    storeGroup(bundle([]));
    const user = renderBuilder();
    const dialog = await openFromMenu(user);
    await user.click(within(dialog).getByRole("button", { name: "Bài nghe" }));

    await user.upload(
      within(dialog).getByLabelText("Chọn tệp từ máy"),
      new File(["RIFF"], "song.wav", { type: "audio/wav" }),
    );

    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Không dùng được tệp này",
    );
    expect(save(dialog)).toBeDisabled();
    expect(puts).toEqual([]);
  });

  it("asks before None replaces a passage, naming it and its linked gaps, and keeps the group and its members", async () => {
    storeGroup(bundle([passage(TITLE, { bound: true })]));
    const user = renderBuilder();
    const dialog = await openFromLine(user, `Bài đọc · ${TITLE} · 1 chỗ trống`);
    await user.click(within(dialog).getByRole("button", { name: "Không có" }));
    await user.click(save(dialog));

    const confirm = await screen.findByRole("dialog", {
      name: "Gỡ nội dung dùng chung?",
    });
    expect(confirm).toHaveTextContent(
      `Bài đọc “${TITLE}” sẽ bị gỡ khỏi nhóm và 1 chỗ trống không còn liên kết với câu hỏi.`,
    );
    expect(puts).toEqual([]);
    await user.click(within(confirm).getByRole("button", { name: "Gỡ và lưu" }));

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(puts).toHaveLength(1);
    expect(puts[0]?.bundle.group.stimuli).toEqual([]);
    expect(puts[0]?.bundle.group.recordings).toEqual([]);
    expect(puts[0]?.bundle.group.members).toHaveLength(1);
    expect(puts[0]?.bundle.questions).toHaveLength(1);
    expect(puts[0]?.bundle.group.title).toBe("Reading club");
  });

  it("refuses to save, with one sentence, while the title is empty or a gap is unbound", async () => {
    storeGroup(bundle([passage(TITLE, { bound: false })]));
    const user = renderBuilder();
    const dialog = await openFromLine(user, `Bài đọc · ${TITLE} · 1 chỗ trống`);
    expect(save(dialog)).toBeDisabled();
    expect(
      within(dialog).getByText(
        "Hãy liên kết đầy đủ các chỗ trống tới câu hỏi hoặc ô trả lời còn tồn tại.",
      ),
    ).toBeVisible();
    expect(
      within(dialog).getByRole("group", { name: "Chỗ trống 1, chưa liên kết" }),
    ).toBeVisible();

    await user.click(within(dialog).getByRole("button", { name: "Không có" }));
    await user.click(within(dialog).getByRole("button", { name: "Bài đọc" }));
    await user.clear(titleField(dialog));
    expect(within(dialog).getByText("Nhập tiêu đề từ 1 đến 200 ký tự.")).toBeVisible();
    expect(save(dialog)).toBeDisabled();
    expect(puts).toEqual([]);
  });

  it("stays open with the editor's message when a save fails, and refuses with one sentence once the group changed elsewhere", async () => {
    const user = renderBuilder();
    const dialog = await openFromLine(user, `Bài đọc · ${TITLE}`);
    await user.type(titleField(dialog), "!");
    answer = "fail";
    await user.click(save(dialog));

    expect(await within(dialog).findByText("Máy chủ đang bận.")).toBeVisible();
    expect(dialog).toBeVisible();

    answer = "stale";
    await user.click(save(dialog));

    expect(
      await within(dialog).findByText(
        "Nhóm đã thay đổi ở nơi khác nên không thể lưu tại đây; hãy lưu bản sao vào ngân hàng từ khung chỉnh sửa.",
      ),
    ).toBeVisible();
    expect(save(dialog)).toBeDisabled();
    expect(puts).toHaveLength(2);
  });

  it("closes on Cancel without asking when nothing changed, and sends nothing", async () => {
    const user = renderBuilder();
    const dialog = await openFromLine(user, `Bài đọc · ${TITLE}`);
    await user.click(within(dialog).getByRole("button", { name: "Huỷ" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() =>
      expect(screen.getByRole("button", { name: `Bài đọc · ${TITLE}` })).toHaveFocus(),
    );
    expect(puts).toEqual([]);
  });

  it("asks before discarding an edit, and sends nothing", async () => {
    const user = renderBuilder();
    const dialog = await openFromLine(user, `Bài đọc · ${TITLE}`);
    await user.type(titleField(dialog), "!");
    await user.keyboard("{Escape}");
    const confirm = await screen.findByRole("dialog", { name: "Bỏ các thay đổi?" });
    await user.click(within(confirm).getByRole("button", { name: "Tiếp tục sửa" }));
    await waitFor(() =>
      expect(screen.queryByRole("dialog", { name: "Bỏ các thay đổi?" })).toBeNull(),
    );
    expect(titleField(dialog)).toHaveValue(`${TITLE}!`);

    await user.click(within(dialog).getByRole("button", { name: "Huỷ" }));
    const again = await screen.findByRole("dialog", { name: "Bỏ các thay đổi?" });
    await user.click(within(again).getByRole("button", { name: "Bỏ thay đổi" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.getByRole("button", { name: `Bài đọc · ${TITLE}` })).toBeVisible();
    expect(puts).toEqual([]);
  });

  it("leaves a group that holds more than it shows in the pane, which says so, and drops nothing", async () => {
    storeGroup(bundle([passage(TITLE), passage("Second text")]));
    const user = renderBuilder();
    await user.click(await line("2 ngữ liệu · 0 bản ghi âm"));

    expect(
      await screen.findByText(
        "Nhóm này có nhiều nội dung dùng chung hơn hộp thoại có thể hiển thị, nên được chỉnh sửa ngay tại đây.",
      ),
    ).toBeVisible();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByLabelText("Tên nhóm câu hỏi")).toHaveValue("Reading club");
    expect(puts).toEqual([]);
  });
});
