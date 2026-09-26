import { describe, expect, it } from "vitest";
import type { ContentDocument } from "@/components/shared/content/model";
import {
  ACCEPTED_LIMITS,
  BLANK_LIMIT,
  candidateOptions,
  changeType,
  clampAccepted,
  dropsAnswerData,
  exceedsBlankLimit,
  pickCandidate,
  restore,
  exclude,
  setPrompt,
  validPoints,
} from "@/features/imports/draft";
import {
  keyPapers,
  matchesFilter,
  orderFindings,
  stepFinding,
} from "@/features/imports/findings";
import {
  isRetryable,
  isWaitingToRetry,
  runErrorKey,
  stageStates,
} from "@/features/imports/status";
import { EXAM_SOURCE_ID, finding, option, question, run } from "./fixtures";

const value = (text: string) => ({ value: text, evidence: [] });
const TRUE_FALSE = { truth: "Đúng", falsity: "Sai" };

function withGaps(ids: string[]): ContentDocument {
  return {
    format: "semantic_v1",
    blocks: [
      {
        type: "paragraph",
        content: [
          { type: "text", text: "She ", marks: [] },
          ...ids.map((id, index) => ({
            type: "gap" as const,
            id,
            label: String(index + 1),
          })),
        ],
      },
    ],
  };
}

describe("reading a conflicting key the way recognition does", () => {
  const q = question({ id: "q", label: "1" });

  it("maps option letters, with or without brackets", () => {
    expect(candidateOptions(q, value("B"))).toEqual(["q-b"]);
    expect(candidateOptions(q, value("(c)."))).toEqual(["q-c"]);
  });

  it("maps option text", () => {
    expect(candidateOptions(q, value("gone"))).toEqual(["q-c"]);
  });

  it("refuses a value that names no option or too many for a single answer", () => {
    expect(candidateOptions(q, value("E"))).toBeNull();
    expect(candidateOptions(q, value("A, C"))).toBeNull();
    expect(candidateOptions({ ...q, type: "multiple_choice" }, value("A, C"))).toEqual([
      "q-a",
      "q-c",
    ]);
  });

  it("maps Đúng/Sai onto a true/false question", () => {
    const tf = question({
      id: "t",
      label: "2",
      type: "true_false",
      options: [option("t-t", "T", "True"), option("t-f", "F", "False")],
    });
    expect(candidateOptions(tf, value("Sai"))).toEqual(["t-f"]);
  });

  it("records a picked value as the teacher's decision with that value's evidence", () => {
    const evidence = [{ sourceId: EXAM_SOURCE_ID, blockId: "k", start: 0, end: 1 }];
    const picked = pickCandidate(
      {
        ...q,
        answer: {
          state: "conflict",
          optionIds: [],
          evidence: [],
          candidates: [{ value: "B", evidence }],
        },
      },
      { value: "B", evidence },
    );
    expect(picked?.answer).toEqual({ state: "known", optionIds: ["q-b"], evidence });
    expect(picked?.origins.answer).toBe("teacher_entered");
    expect(picked?.origins.options).toBe("source_explicit");
  });
});

describe("changing a question's type", () => {
  it("keeps the key when single choice becomes multiple choice", () => {
    const q = question({ id: "q", label: "1" });
    const next = changeType(q, "multiple_choice", TRUE_FALSE);
    expect(next.answer.optionIds).toEqual(["q-a"]);
    expect(next.origins.type).toBe("teacher_entered");
    expect(dropsAnswerData(q, next)).toBe(false);
  });

  it("warns when a change drops options or keys", () => {
    const multiple = question({
      id: "q",
      label: "1",
      type: "multiple_choice",
      answer: { state: "known", optionIds: ["q-a", "q-b"], evidence: [] },
    });
    expect(
      dropsAnswerData(multiple, changeType(multiple, "single_choice", TRUE_FALSE)),
    ).toBe(true);
    expect(
      dropsAnswerData(multiple, changeType(multiple, "short_answer", TRUE_FALSE)),
    ).toBe(true);
  });

  it("binds fill-blank answers to the prompt's gaps", () => {
    const q = question({ id: "q", label: "1", prompt: withGaps(["g1"]) });
    const blank = changeType(q, "fill_blank", TRUE_FALSE);
    expect(blank.options).toEqual([]);
    expect(blank.blanks).toEqual([
      { gapId: "g1", label: "1", accepted: [], caseSensitive: false },
    ]);
    expect(blank.answer.state).toBe("unknown");

    const answered = {
      ...blank,
      blanks: [{ ...blank.blanks[0]!, accepted: ["went"] }],
    };
    const widened = setPrompt(answered, withGaps(["g1", "g2"]));
    expect(widened.blanks.map((item) => [item.gapId, item.accepted])).toEqual([
      ["g1", ["went"]],
      ["g2", []],
    ]);
    expect(widened.origins.prompt).toBe("teacher_entered");
  });
});

describe("small draft rules", () => {
  it("excludes with a reason and restores without one", () => {
    const q = question({ id: "q", label: "1" });
    const excluded = exclude(q, "Matching question");
    expect(excluded.excluded).toEqual({ reason: "Matching question" });
    expect("excluded" in restore(excluded)).toBe(false);
  });

  it("accepts only positive scores with at most two decimals", () => {
    expect(validPoints("1")).toBe(true);
    expect(validPoints("0.25")).toBe(true);
    expect(validPoints("0")).toBe(false);
    expect(validPoints("1.255")).toBe(false);
    expect(validPoints("-1")).toBe(false);
  });
});

describe("findings and runs", () => {
  it("never lets an informational note into a navigation set, nor an acknowledged item into Cần xác nhận", () => {
    const info = finding({
      id: "i",
      code: "FORMATTING_SIMPLIFIED",
      severity: "informational",
    });
    const done = finding({
      id: "d",
      code: "NUMBERING_IRREGULAR",
      severity: "review_required",
      acknowledged: true,
    });
    expect(matchesFilter(info, "all")).toBe(false);
    expect(matchesFilter(done, "review")).toBe(false);
    expect(matchesFilter(done, "all")).toBe(true);
  });

  it("orders findings by where their target sits, whole-import findings first", () => {
    const later = finding({
      id: "b",
      code: "MISSING_ANSWER",
      severity: "blocking",
      target: "q2",
    });
    const earlier = finding({
      id: "a",
      code: "MISSING_ANSWER",
      severity: "blocking",
      target: "q1",
    });
    const global = finding({ id: "g", code: "NO_QUESTIONS", severity: "blocking" });
    const positions = new Map([
      ["q1", 1],
      ["q2", 2],
    ]);
    expect(
      orderFindings([later, earlier, global], positions).map((item) => item.id),
    ).toEqual(["g", "a", "b"]);
  });

  it("parses the paper numbers an ambiguous key offers", () => {
    expect(
      keyPapers(
        finding({
          id: "p",
          code: "AMBIGUOUS_KEY_PAPER",
          severity: "blocking",
          field: "101, 102,x",
        }),
      ),
    ).toEqual([101, 102]);
  });

  it("places a run on the teacher's stages without inventing progress", () => {
    expect(stageStates("queued")).toEqual([
      "waiting",
      "waiting",
      "waiting",
      "waiting",
      "waiting",
    ]);
    expect(stageStates("extraction")).toEqual([
      "done",
      "current",
      "waiting",
      "waiting",
      "waiting",
    ]);
    expect(stageStates("recognition")).toEqual([
      "done",
      "done",
      "current",
      "current",
      "waiting",
    ]);
    expect(stageStates("ready")).toEqual(["done", "done", "done", "done", "done"]);
  });

  it("offers a retry only when the same files can succeed", () => {
    expect(isRetryable("PROCESSING_TIMEOUT")).toBe(true);
    expect(isRetryable("SOURCE_UNSUPPORTED")).toBe(false);
    expect(isRetryable("LEGACY_CONVERSION_UNAVAILABLE")).toBe(false);
    for (const code of [
      "PDF_NO_TEXT",
      "PDF_PROTECTED",
      "PDF_INVALID",
      "PDF_TOO_LARGE",
    ]) {
      expect(isRetryable(code)).toBe(false);
      expect(runErrorKey(code)).toBe(code);
    }
    expect(runErrorKey("SOMETHING_NEW")).toBe("UNKNOWN");
    expect(runErrorKey("PIPELINE_RETIRED")).toBe("PIPELINE_RETIRED");
  });
});

describe("round-two rules", () => {
  const positions = new Map([
    ["q1", 1],
    ["q2", 2],
    ["q3", 3],
  ]);
  const a = finding({
    id: "a",
    code: "MISSING_ANSWER",
    severity: "blocking",
    target: "q1",
  });
  const c = finding({
    id: "c",
    code: "MISSING_ANSWER",
    severity: "blocking",
    target: "q3",
  });

  it("moves on from a finding that was resolved, instead of starting over", () => {
    const ordered = [a, c];
    expect(stepFinding(ordered, { id: "b", at: 2 }, 1, positions)?.id).toBe("c");
    expect(stepFinding(ordered, { id: "b", at: 2 }, -1, positions)?.id).toBe("a");
    expect(stepFinding(ordered, { id: "c", at: 3 }, 1, positions)?.id, "wraps").toBe(
      "a",
    );
    expect(stepFinding(ordered, null, -1, positions)?.id).toBe("c");
    expect(stepFinding([], null, 1, positions)).toBeUndefined();
  });

  it("keeps a remaining finding on the same question as the next one", () => {
    const sibling = finding({
      id: "a2",
      code: "IRREGULAR_OPTION_COUNT",
      severity: "review_required",
      target: "q1",
    });
    expect(stepFinding([sibling, c], { id: "a", at: 1 }, 1, positions)?.id).toBe("a2");
  });

  it("bounds accepted answers to the contract", () => {
    const many = Array.from({ length: 25 }, (_, index) => `answer ${index}`);
    expect(clampAccepted(many)).toHaveLength(ACCEPTED_LIMITS.count);
    expect(Array.from(clampAccepted(["ồ".repeat(600)])[0]!)).toHaveLength(
      ACCEPTED_LIMITS.length,
    );
    expect(clampAccepted([" go ", "go", "", "went"])).toEqual(["go", "went"]);
  });

  it("seeds a question converted to true/false with the teacher's words", () => {
    const q = question({ id: "q", label: "1" });
    const tf = changeType(q, "true_false", TRUE_FALSE);
    expect(tf.options.map((item) => item.label)).toEqual(["T", "F"]);
    expect(JSON.stringify(tf.options)).toContain("Đúng");
    expect(JSON.stringify(tf.options)).not.toContain("True");
  });

  it("groups run errors by what the teacher can do", () => {
    expect(runErrorKey("CONVERSION_BUSY")).toBe("TEMPORARY");
    expect(runErrorKey("WORKER_LEASE_LOST")).toBe("TEMPORARY");
    expect(runErrorKey("PROCESSOR_OUTPUT_INVALID")).toBe("INTERNAL");
    expect(runErrorKey("STORAGE_INTEGRITY_FAILED")).toBe("STORAGE_INTEGRITY_FAILED");
    expect(isRetryable("STORAGE_INTEGRITY_FAILED")).toBe(false);
    expect(isRetryable("PRIVATE_ANSWER_EVIDENCE")).toBe(false);
    expect(isRetryable("PROCESSOR_OUTPUT_INVALID")).toBe(true);
    expect(isRetryable("STORAGE_QUOTA_EXCEEDED")).toBe(true);
  });

  it("recognises a run queued again after a temporary failure", () => {
    expect(
      isWaitingToRetry(run({ status: "queued", errorCode: "CONVERSION_BUSY" })),
    ).toBe(true);
    expect(isWaitingToRetry(run({ status: "running" }))).toBe(false);
    expect(stageStates("extraction", true)).toEqual([
      "done",
      "waiting",
      "waiting",
      "waiting",
      "waiting",
    ]);
  });
});

describe("round-three rules", () => {
  const gaps = (count: number) =>
    withGaps(Array.from({ length: count }, (_, index) => `g${index + 1}`));

  it("refuses a fill-blank prompt with more gaps than the contract binds", () => {
    const q = question({
      id: "q",
      label: "1",
      type: "fill_blank",
      prompt: gaps(1),
      options: [],
      blanks: [{ gapId: "g1", label: "1", accepted: ["went"], caseSensitive: false }],
    });
    expect(exceedsBlankLimit(gaps(BLANK_LIMIT))).toBe(false);
    expect(exceedsBlankLimit(gaps(BLANK_LIMIT + 1))).toBe(true);
    expect(setPrompt(q, gaps(BLANK_LIMIT + 1))).toBe(q);

    const full = setPrompt(q, gaps(BLANK_LIMIT));
    expect(full.blanks).toHaveLength(BLANK_LIMIT);
    expect(full.blanks[0]!.accepted).toEqual(["went"]);
  });

  it("binds no more than the limit when a question becomes fill-blank", () => {
    const q = question({ id: "q", label: "1", prompt: gaps(BLANK_LIMIT + 3) });
    expect(changeType(q, "fill_blank", TRUE_FALSE).blanks).toHaveLength(BLANK_LIMIT);
  });

  it("keeps a single-choice prompt whatever its gaps", () => {
    const q = question({ id: "q", label: "1" });
    expect(setPrompt(q, gaps(BLANK_LIMIT + 1)).prompt).toEqual(gaps(BLANK_LIMIT + 1));
  });
});
