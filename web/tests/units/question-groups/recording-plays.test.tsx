import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import { DeckScale } from "@/components/ui/deck-scale";
import {
  BuilderGroupPane,
  type GroupPaneBridge,
} from "@/features/tests/components/BuilderGroupPane";
import type { GroupBundle, StoredGroup } from "@/features/question-groups/api";
import type { GroupRecording } from "@/features/question-groups/model";
import type { DraftScope } from "@/lib/drafts/store";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import "@/lib/i18n";

globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

const API = "http://localhost:8080";
const GROUP = "018f0000-0000-7000-8000-0000000006a1";
const SECTION = "018f0000-0000-7000-8000-0000000006b1";
const MEMBER = "018f0000-0000-7000-8000-0000000006c1";
const RECORDING = "018f0000-0000-7000-8000-0000000006d1";
const LECTURE = "018f0000-0000-7000-8000-0000000006e1";
const STAMP = "2026-10-01T00:00:00Z";
const PLAYS = "Số lần nghe";
const PAUSE = "Cho học sinh tạm dừng";

const drafts = vi.hoisted(() => ({ scope: null as DraftScope | null }));

vi.mock("@/lib/drafts/store", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/drafts/store")>()),
  openDraftScope: () => Promise.resolve(drafts.scope),
}));

function bundle(policy: GroupRecording["policy"]): GroupBundle {
  return {
    group: {
      id: GROUP,
      title: "Part 2 · Listening",
      instructions: null,
      members: [{ questionId: MEMBER, optionOrder: "shuffle" }],
      stimuli: [
        {
          id: "018f0000-0000-7000-8000-0000000006f1",
          title: "lecture",
          content: {
            format: "semantic_v1",
            blocks: [{ type: "audio", assetId: LECTURE, label: "Bài nghe" }],
          },
          gaps: [],
        },
      ],
      recordings: [{ id: RECORDING, assetId: LECTURE, policy, transcript: null }],
    },
    questions: [
      {
        id: MEMBER,
        input: {
          type: "single_choice",
          level: null,
          skill: "listening",
          prompt: "Where is the bank?",
          points: 1,
          options: [
            { text: "Next to the park", isCorrect: true },
            { text: "Behind the school", isCorrect: false },
          ],
          blanks: [],
          tags: [],
        },
      },
    ],
  };
}

function stored(value: GroupBundle, revision = 1): StoredGroup {
  return {
    bundle: value,
    ownerSectionId: SECTION,
    revision,
    testUpdatedAt: STAMP,
    archivedAt: null,
    createdAt: STAMP,
    updatedAt: STAMP,
    assets: [
      {
        id: LECTURE,
        kind: "audio",
        url: "https://example.test/lecture.mp3",
        mimeType: "audio/mpeg",
        bytes: 2_400_000,
        durationMs: 110_000,
        originalFilename: "lecture.mp3",
        createdAt: STAMP,
      },
    ],
    unavailableAssetIds: [],
  };
}

function emptyScope(): DraftScope {
  return {
    read: vi.fn().mockResolvedValue(null),
    write: vi.fn().mockResolvedValue(undefined),
    remove: vi.fn().mockResolvedValue(undefined),
  };
}

let current: StoredGroup;
let saves: GroupBundle[];

beforeEach(() => {
  useAuthStore.setState({ user: teacherUser });
  drafts.scope = emptyScope();
  saves = [];
  server.use(
    http.get(`${API}/teacher/question-groups/${GROUP}`, () =>
      contractJson("/teacher/question-groups/{id}", "get", 200, current),
    ),
  );
});

afterEach(() => {
  useAuthStore.setState({ user: null });
  drafts.scope = null;
});

function renderPane() {
  const flushRef: { current: (() => Promise<void>) | null } = { current: null };
  const save = async (value: GroupBundle, revision: number) => {
    saves.push(value);
    current = { ...current, bundle: value, revision: revision + 1 };
    return current;
  };
  const router = createMemoryRouter(
    [
      {
        path: "/",
        element: (
          <DeckScale>
            <BuilderGroupPane
              id={GROUP}
              selectedQuestionId={null}
              coordinate={(operation) => operation()}
              save={save}
              onChange={() => undefined}
              onSelectedQuestionChange={() => undefined}
              flushRef={flushRef}
              retryRef={{ current: null }}
              bridgeRef={{ current: null as GroupPaneBridge | null }}
              onStatus={() => undefined}
            />
          </DeckScale>
        ),
      },
    ],
    { initialEntries: ["/"] },
  );
  const view = render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return {
    user: userEvent.setup(),
    unmount: view.unmount,
    saveNow: () => act(async () => flushRef.current?.()),
  };
}

describe("a group's recording in the builder's group pane", () => {
  it("shows a stored 3 plays as a fourth option, and choosing Twice saves 2", async () => {
    current = stored(
      bundle({ maxPlays: 3, allowSeek: false, showTranscriptAfterSubmit: true }),
    );
    const { user, saveNow } = renderPane();

    const plays = await screen.findByRole("radiogroup", { name: PLAYS });
    expect(
      within(plays)
        .getAllByRole("radio")
        .map((radio) => radio.closest("label")?.textContent),
    ).toEqual(["Một lần", "Hai lần", "Không giới hạn", "3 lần"]);
    expect(within(plays).getByRole("radio", { name: "3 lần" })).toBeChecked();

    await user.click(within(plays).getByRole("radio", { name: "Hai lần" }));
    await saveNow();

    expect(saves.at(-1)?.group.recordings[0]?.policy.maxPlays).toBe(2);
    expect(within(plays).getAllByRole("radio")).toHaveLength(3);
    expect(within(plays).getByRole("radio", { name: "Hai lần" })).toBeChecked();
  });

  it("shows “Students can pause” under Plays, on for a policy stored without the field", async () => {
    current = stored(
      bundle({ maxPlays: 2, allowSeek: false, showTranscriptAfterSubmit: true }),
    );
    renderPane();

    const pause = await screen.findByRole("switch", { name: PAUSE });
    expect(pause).toBeChecked();
    const order = [
      screen.getByRole("radiogroup", { name: PLAYS }),
      pause,
      screen.getByRole("switch", { name: "Cho tua tới" }),
    ];
    for (const [index, element] of order.slice(1).entries())
      expect(
        order[index]!.compareDocumentPosition(element) &
          Node.DOCUMENT_POSITION_FOLLOWING,
      ).toBeTruthy();
  });

  it("saves pausing off for the recording and reads it back off", async () => {
    current = stored(
      bundle({
        maxPlays: 2,
        allowSeek: false,
        showTranscriptAfterSubmit: true,
        allowPause: true,
      }),
    );
    const first = renderPane();
    const pause = await screen.findByRole("switch", { name: PAUSE });
    pause.focus();
    await first.user.keyboard(" ");
    expect(pause).not.toBeChecked();
    await first.saveNow();

    expect(saves).toHaveLength(1);
    expect(saves[0]?.group.recordings[0]?.policy).toEqual({
      maxPlays: 2,
      allowSeek: false,
      showTranscriptAfterSubmit: true,
      allowPause: false,
    });
    first.unmount();

    renderPane();
    expect(await screen.findByRole("switch", { name: PAUSE })).not.toBeChecked();
  });

  it("restores a device draft written before the switch with pausing still off, and saves it off", async () => {
    current = stored(
      bundle({
        maxPlays: 2,
        allowSeek: false,
        showTranscriptAfterSubmit: true,
        allowPause: false,
      }),
    );
    const before = bundle({
      maxPlays: 1,
      allowSeek: false,
      showTranscriptAfterSubmit: true,
    });
    drafts.scope = {
      ...emptyScope(),
      read: vi.fn().mockResolvedValue({
        owner: teacherUser.id,
        item: `group:${GROUP}`,
        token: "draft",
        createdAt: Date.now(),
        expiresAt: Date.now() + 60_000,
        payload: { version: 1, revision: 1, testUpdatedAt: STAMP, bundle: before },
      }),
    };
    const { user, saveNow } = renderPane();

    await user.click(await screen.findByRole("button", { name: "Khôi phục bản nháp" }));
    expect(await screen.findByRole("switch", { name: PAUSE })).not.toBeChecked();
    expect(
      within(screen.getByRole("radiogroup", { name: PLAYS })).getByRole("radio", {
        name: "Một lần",
      }),
    ).toBeChecked();
    await saveNow();

    expect(saves.at(-1)?.group.recordings[0]?.policy).toEqual({
      maxPlays: 1,
      allowSeek: false,
      showTranscriptAfterSubmit: true,
      allowPause: false,
    });
  });
});
