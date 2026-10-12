import { describe, expect, it } from "vitest";
import type { TFunction } from "i18next";
import {
  emptyGroup,
  newGroupQuestion,
  memberValues,
} from "@/features/question-groups/model";
import {
  readGroupRecovery,
  independentBundle,
} from "@/features/question-groups/recovery";
import { storedGroupBundle } from "@tests/support/storedGroupBundle";

const t = ((key: string) => key) as TFunction;
function bundle() {
  const result = emptyGroup("Group");
  const member = newGroupQuestion(t);
  member.input.level = "c2";
  member.input.skill = "speaking";
  member.input.options = Array.from({ length: 9 }, (_, i) => ({
    text: String(i),
    isCorrect: i === 0,
  }));
  result.questions.push(member);
  result.group.members.push({ questionId: member.id, optionOrder: "fixed" });
  return result;
}

describe("stored group metadata", () => {
  it("keeps metadata and nine stored options through recovery and independent cloning", () => {
    const original = bundle();
    const recovered = readGroupRecovery(
      { version: 1, revision: 1, bundle: original },
      original.group.id,
    );
    expect(recovered?.bundle.questions[0]?.input).toMatchObject({
      level: "c2",
      skill: "speaking",
    });
    expect(recovered?.bundle.questions[0]?.input.options).toHaveLength(9);
    const clone = independentBundle(recovered!.bundle);
    expect(clone.group.id).not.toBe(original.group.id);
    expect(clone.questions[0]?.id).not.toBe(original.questions[0]?.id);
    expect(clone.questions[0]?.input).toMatchObject({ level: "c2", skill: "speaking" });
    expect(original.questions[0]?.input.level).toBe("c2");
  });
  it.each([
    ["set", "Bản đồ thị trấn"],
    ["null", null],
  ])(
    "keeps a member's image alt text through recovery and cloning when %s",
    (_what, alt) => {
      const original = bundle();
      original.questions[0]!.input.mediaAssetId =
        "018f0000-0000-7000-8000-0000000000c1";
      original.questions[0]!.input.mediaAlt = alt;
      const recovered = readGroupRecovery(
        { version: 1, revision: 1, bundle: original },
        original.group.id,
      );
      expect(recovered?.bundle.questions[0]?.input.mediaAlt).toBe(alt);
      expect(independentBundle(recovered!.bundle).questions[0]?.input.mediaAlt).toBe(
        alt,
      );
      expect(memberValues(recovered!.bundle.questions[0]!.input).mediaAlt).toBe(alt);
    },
  );
  it("gives the editor a member with no alt text as null", () => {
    expect(memberValues(bundle().questions[0]!.input).mediaAlt).toBeNull();
  });
  it("normalizes old version-one omissions while rejecting invalid enum metadata", () => {
    const original = bundle();
    const missing = {
      ...original,
      questions: original.questions.map(
        ({ id, input: { level: _level, skill: _skill, ...input } }) => ({ id, input }),
      ),
    };
    const recovered = readGroupRecovery(
      { version: 1, revision: 1, bundle: missing },
      original.group.id,
    );
    expect(recovered?.bundle.questions[0]?.input).toMatchObject({
      level: null,
      skill: null,
    });
    expect(memberValues(missing.questions[0]!.input)).toMatchObject({
      level: null,
      skill: null,
    });
    for (const field of ["level", "skill"]) {
      const invalid = structuredClone(original);
      Object.assign(invalid.questions[0]!.input, { [field]: "unknown" });
      expect(
        readGroupRecovery(
          { version: 1, revision: 1, bundle: invalid },
          original.group.id,
        ),
      ).toBeNull();
    }
    expect(newGroupQuestion(t).input).toMatchObject({ level: null, skill: null });
  });
  it("projects request omissions to null without overwriting supplied metadata or other fields", () => {
    const original = bundle();
    const projected = storedGroupBundle(original);
    expect(memberValues(original.questions[0]!.input)).toMatchObject({
      level: "c2",
      skill: "speaking",
    });
    expect(projected).toEqual(original);
    const omitted = {
      ...original,
      questions: original.questions.map(
        ({ id, input: { level: _level, skill: _skill, ...input } }) => ({ id, input }),
      ),
    };
    const normalized = storedGroupBundle(omitted);
    expect(normalized.questions[0]!.input).toMatchObject({
      level: null,
      skill: null,
      options: omitted.questions[0]!.input.options,
    });
  });
});
