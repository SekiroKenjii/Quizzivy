import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { GroupBundle, StoredGroup } from "@/features/question-groups/api";
import {
  readGroupRecovery,
  withStoredPause,
} from "@/features/question-groups/recovery";
import { useGroupEditor } from "@/features/question-groups/useGroupEditor";
import type { DraftScope } from "@/lib/drafts/store";
import "@/lib/i18n";

const GROUP = "018f0000-0000-7000-8000-0000000005a1";
const MEMBER = "018f0000-0000-7000-8000-0000000005b1";
const MATERIAL = "018f0000-0000-7000-8000-0000000005c1";
const RECORDING = "018f0000-0000-7000-8000-0000000005d1";
const ADDED = "018f0000-0000-7000-8000-0000000005d2";
const LECTURE = "018f0000-0000-7000-8000-0000000005e1";
const PROMPT_AUDIO = "018f0000-0000-7000-8000-0000000005e2";
const STAMP = "2026-10-01T00:00:00Z";

const BEFORE_PAUSE_DRAFT =
  '{"version":1,"revision":3,"testUpdatedAt":null,"bundle":{"group":{' +
  `"id":"${GROUP}","title":"Part 2 · Listening","instructions":null,` +
  `"members":[{"questionId":"${MEMBER}","optionOrder":"shuffle"}],` +
  `"stimuli":[{"id":"${MATERIAL}","title":"lecture","content":{"format":"semantic_v1",` +
  `"blocks":[{"type":"audio","assetId":"${LECTURE}","label":"Audio"}]},"gaps":[]}],` +
  `"recordings":[{"id":"${RECORDING}","assetId":"${LECTURE}",` +
  '"policy":{"maxPlays":1,"allowSeek":false,"showTranscriptAfterSubmit":true},' +
  '"transcript":"Edited on this device"},' +
  `{"id":"${ADDED}","assetId":"${PROMPT_AUDIO}",` +
  '"policy":{"maxPlays":2,"allowSeek":false,"showTranscriptAfterSubmit":true},' +
  '"transcript":null}]},' +
  `"questions":[{"id":"${MEMBER}","input":{"type":"single_choice","level":null,` +
  '"skill":"listening","prompt":"Where is the bank?","points":1,' +
  `"mediaAssetId":"${PROMPT_AUDIO}","mediaAlt":null,` +
  '"audio":{"maxPlays":2,"allowSeek":false,"showTranscriptAfterSubmit":false},' +
  '"transcript":null,"explanation":null,"sampleAnswer":null,"tags":[],' +
  '"options":[{"id":null,"text":"Next to the park","isCorrect":true},' +
  '{"id":null,"text":"Behind the school","isCorrect":false}],"blanks":[]}}]}}';

function storedGroup(): StoredGroup {
  const draft = readGroupRecovery(JSON.parse(BEFORE_PAUSE_DRAFT), GROUP)!.bundle;
  const bundle: GroupBundle = {
    group: {
      ...draft.group,
      recordings: [
        {
          id: RECORDING,
          assetId: LECTURE,
          policy: {
            maxPlays: 1,
            allowSeek: false,
            showTranscriptAfterSubmit: true,
            allowPause: false,
          },
          transcript: "Stored on the server",
        },
      ],
    },
    questions: draft.questions.map((question) => ({
      ...question,
      input: {
        ...question.input,
        audio: {
          maxPlays: 2,
          allowSeek: false,
          showTranscriptAfterSubmit: true,
          allowPause: false,
        },
      },
    })),
  };
  return {
    bundle,
    ownerSectionId: null,
    revision: 3,
    testUpdatedAt: null,
    archivedAt: null,
    createdAt: STAMP,
    updatedAt: STAMP,
    assets: [],
    unavailableAssetIds: [],
  };
}

function scope(): DraftScope {
  return {
    read: vi.fn().mockResolvedValue(null),
    write: vi.fn().mockResolvedValue(undefined),
    remove: vi.fn().mockResolvedValue(undefined),
  };
}

function pauses(bundle: GroupBundle) {
  return {
    recordings: bundle.group.recordings.map((recording) => [
      recording.id,
      recording.policy.allowPause,
    ]),
    member: bundle.questions[0]?.input.audio?.allowPause,
  };
}

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe("a group draft a tab left on the device before “Students can pause”", () => {
  it("is still read, and has no allowPause anywhere", () => {
    const recovered = readGroupRecovery(JSON.parse(BEFORE_PAUSE_DRAFT), GROUP);

    expect(recovered).not.toBeNull();
    expect(BEFORE_PAUSE_DRAFT).not.toContain("allowPause");
    expect(pauses(recovered!.bundle)).toEqual({
      recordings: [
        [RECORDING, undefined],
        [ADDED, undefined],
      ],
      member: undefined,
    });
  });

  it("restores the rule the server holds for the same recording and member question, and true for a recording the server does not have", () => {
    const stored = storedGroup();
    const recovered = readGroupRecovery(JSON.parse(BEFORE_PAUSE_DRAFT), GROUP)!;

    const restored = withStoredPause(recovered, stored.bundle);

    expect(pauses(restored.bundle)).toEqual({
      recordings: [
        [RECORDING, false],
        [ADDED, true],
      ],
      member: false,
    });
    expect(restored.bundle.group.recordings[0]?.transcript).toBe(
      "Edited on this device",
    );
    expect(restored.bundle.questions[0]?.input.audio?.showTranscriptAfterSubmit).toBe(
      false,
    );
    expect(restored.revision).toBe(3);
  });

  it("saves the restored bundle with pausing still off", async () => {
    const stored = storedGroup();
    const local = scope();
    const restored = withStoredPause(
      readGroupRecovery(JSON.parse(BEFORE_PAUSE_DRAFT), GROUP)!,
      stored.bundle,
    );
    const save = vi.fn(async (bundle: GroupBundle, revision: number) => ({
      ...stored,
      bundle,
      revision: revision + 1,
    }));
    const { result } = renderHook(() =>
      useGroupEditor({ stored, recovered: restored, scope: local, save }),
    );

    expect(pauses(result.current.bundle)).toEqual(pauses(restored.bundle));
    await act(async () => {
      await result.current.saveNow();
    });

    expect(save).toHaveBeenCalledTimes(1);
    const [sent, revision] = save.mock.calls[0]!;
    expect(revision).toBe(3);
    expect(pauses(sent)).toEqual({
      recordings: [
        [RECORDING, false],
        [ADDED, true],
      ],
      member: false,
    });
  });

  it("keeps a value a newer draft wrote, even where the server holds another", () => {
    const stored = storedGroup();
    const newer = JSON.parse(BEFORE_PAUSE_DRAFT) as {
      bundle: { group: { recordings: { policy: Record<string, unknown> }[] } };
    };
    newer.bundle.group.recordings[0]!.policy["allowPause"] = true;

    const restored = withStoredPause(readGroupRecovery(newer, GROUP)!, stored.bundle);

    expect(restored.bundle.group.recordings[0]?.policy.allowPause).toBe(true);
  });
});
