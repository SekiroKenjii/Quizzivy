import type {
  ImportDraftOption,
  ImportDraftQuestion,
  ImportDraftSection,
  ImportFinding,
  ImportReview,
  ImportReviewSummary,
  ImportRun,
  ImportSource,
  ImportSourceRole,
  ImportSourceView,
  WordImport,
} from "@/features/imports/api";
import type { ContentDocument } from "@/components/shared/content/model";
import { http } from "msw";
import { contractJson } from "@tests/support/contractResponse";

export const BASE = "http://localhost:8080";
export const IMPORT_ID = "018f0000-0000-7000-8000-0000000000e1";
export const USER_ID = "018f0000-0000-7000-8000-0000000000e2";
export const EXAM_SOURCE_ID = "018f0000-0000-7000-8000-0000000000e3";
export const KEY_SOURCE_ID = "018f0000-0000-7000-8000-0000000000e4";
export const RUN_ID = "0192e7a0-0000-7000-8000-0000000000e5";
export const TEST_ID = "018f0000-0000-7000-8000-0000000000e6";
export const AT = "2026-09-20T01:00:00Z";

export function capabilities(processingEnabled = true) {
  return http.get(`${BASE}/admin/imports/capabilities`, () =>
    contractJson("/admin/imports/capabilities", "get", 200, {
      intakeEnabled: true,
      processingEnabled,
      retention: { afterCommitDays: 30, afterCancelDays: 7, idleDays: 60 },
    }),
  );
}

export function source(
  role: ImportSourceRole,
  over: Partial<ImportSource> = {},
): ImportSource {
  return {
    id: role === "exam" ? EXAM_SOURCE_ID : KEY_SOURCE_ID,
    role,
    filename: role === "exam" ? "de-thi-hk1.docx" : "dap-an-hk1.docx",
    format: "docx",
    bytes: 20480,
    sha256: "a".repeat(64),
    uploadedBy: USER_ID,
    createdAt: AT,
    ...over,
  };
}

export function run(over: Partial<ImportRun> = {}): ImportRun {
  return {
    id: RUN_ID,
    status: "running",
    stage: "extraction",
    attempt: 1,
    maxAttempts: 3,
    createdAt: AT,
    updatedAt: AT,
    ...over,
  };
}

export function wordImport(over: Partial<WordImport> = {}): WordImport {
  return {
    id: IMPORT_ID,
    title: "Đề thi học kỳ 1",
    status: "needs_review",
    revision: 4,
    sourceRevision: 2,
    sources: [source("exam"), source("answer_key")],
    pendingUploads: 0,
    createdBy: USER_ID,
    createdAt: AT,
    updatedAt: AT,
    ...over,
  };
}

export function text(value: string): ContentDocument {
  return {
    format: "semantic_v1",
    blocks: [
      { type: "paragraph", content: [{ type: "text", text: value, marks: [] }] },
    ],
  };
}

export function option(id: string, label: string, value: string): ImportDraftOption {
  return { id, label, content: text(value) };
}

export function question(
  over: Partial<ImportDraftQuestion> & Pick<ImportDraftQuestion, "id" | "label">,
): ImportDraftQuestion {
  return {
    type: "single_choice",
    prompt: text(`Question ${over.label}`),
    options: [
      option(`${over.id}-a`, "A", "went"),
      option(`${over.id}-b`, "B", "goes"),
      option(`${over.id}-c`, "C", "gone"),
      option(`${over.id}-d`, "D", "going"),
    ],
    blanks: [],
    answer: { state: "known", optionIds: [`${over.id}-a`], evidence: [] },
    points: "1",
    origins: {
      type: "inferred_structure",
      prompt: "source_explicit",
      options: "source_explicit",
      answer: "source_explicit",
      points: "defaulted",
    },
    source: [
      { sourceId: EXAM_SOURCE_ID, blockId: `block-${over.id}`, start: 0, end: 5 },
    ],
    ...over,
  };
}

export function section(
  questions: ImportDraftQuestion[],
  over: Partial<ImportDraftSection> = {},
): ImportDraftSection {
  return {
    id: "section-1",
    title: "Phần 1",
    origin: "source_explicit",
    items: questions.map((item) => ({ question: item })),
    source: [],
    ...over,
  };
}

export function finding(
  over: Partial<ImportFinding> & Pick<ImportFinding, "id" | "code" | "severity">,
): ImportFinding {
  return { count: 1, evidence: [], ...over };
}

export function summary(over: Partial<ImportReviewSummary> = {}): ImportReviewSummary {
  return {
    sections: 1,
    groups: 0,
    questions: 2,
    included: 2,
    excluded: 0,
    totalPoints: "2.00",
    answersKnown: 1,
    answersMissing: 1,
    answersConflicting: 0,
    blocking: 1,
    needsDecision: 0,
    ...over,
  };
}

export function review(
  sections: ImportDraftSection[],
  findings: ImportFinding[],
  over: Partial<ImportReview> = {},
): ImportReview {
  return {
    importId: IMPORT_ID,
    revision: 1,
    draft: {
      version: "word-draft-v1",
      title: "Đề thi học kỳ 1",
      sections,
      notices: [],
      acknowledged: [],
    },
    findings,
    summary: summary(),
    ready: false,
    reprocessed: false,
    updatedAt: AT,
    ...over,
  };
}

export function sourceView(
  role: ImportSourceRole,
  blocks: ImportSourceView["blocks"],
): ImportSourceView {
  return {
    sourceId: role === "exam" ? EXAM_SOURCE_ID : KEY_SOURCE_ID,
    role,
    filename: role === "exam" ? "de-thi-hk1.docx" : "dap-an-hk1.docx",
    blocks,
  };
}

export function errorBody(code: string, message: string) {
  return {
    error: { code, message, requestId: "018f0000-0000-7000-8000-0000000000ff" },
  };
}

export function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((settle) => {
    resolve = settle;
  });
  return { promise, resolve };
}
