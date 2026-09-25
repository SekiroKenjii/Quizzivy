import type { ContentDocument } from "@/components/shared/content/model";
import { questionGaps } from "@/components/shared/content/gaps";
import { contentPlainText } from "@/components/shared/content/plainText";
import type {
  ImportDraftAnswer,
  ImportDraftBlank,
  ImportDraftOption,
  ImportDraftQuestion,
  ImportDraftSection,
  ImportFieldOrigins,
  ImportKeyValue,
  ImportReview,
  ImportSourceRef,
} from "./api";

export type QuestionType = ImportDraftQuestion["type"];

/** EDITABLE_TYPES are the interactions a teacher may choose; unsupported is only ever a recognition result. */
export const EDITABLE_TYPES: readonly Exclude<QuestionType, "unsupported">[] = [
  "single_choice",
  "multiple_choice",
  "true_false",
  "fill_blank",
  "short_answer",
];

/** ReviewWorking is the editable part of a review: what the save request carries. */
export interface ReviewWorking {
  title: string;
  sections: ImportDraftSection[];
  acknowledged: string[];
}

/** workingFrom copies the editable fields out of a server review. */
export function workingFrom(review: ImportReview): ReviewWorking {
  return {
    title: review.draft.title,
    sections: review.draft.sections,
    acknowledged: review.draft.acknowledged,
  };
}

/** QuestionPlace locates one question in the draft, in document order. */
export interface QuestionPlace {
  question: ImportDraftQuestion;
  sectionId: string;
  groupId: string | null;
}

/** questionPlaces lists every question, group members included, in document order. */
export function questionPlaces(
  sections: readonly ImportDraftSection[],
): QuestionPlace[] {
  return sections.flatMap((section) =>
    section.items.flatMap((item): QuestionPlace[] => {
      if (item.question)
        return [{ question: item.question, sectionId: section.id, groupId: null }];
      const group = item.group;
      if (group)
        return group.questions.map((question) => ({
          question,
          sectionId: section.id,
          groupId: group.id,
        }));
      return [];
    }),
  );
}

/** draftPositions numbers every section, group and question in reading order. */
export function draftPositions(
  sections: readonly ImportDraftSection[],
): Map<string, number> {
  const positions = new Map<string, number>();
  const place = (id: string) => {
    if (!positions.has(id)) positions.set(id, positions.size);
  };
  for (const section of sections) {
    place(section.id);
    for (const item of section.items) {
      if (item.question) place(item.question.id);
      if (item.group) {
        place(item.group.id);
        item.group.questions.forEach((question) => place(question.id));
      }
    }
  }
  return positions;
}

/** blockKey identifies one block of one source; block IDs are unique only within a source. */
export function blockKey(sourceId: string, blockId: string): string {
  return `${sourceId}\t${blockId}`;
}

/**
 * blockOwners maps each source block, by blockKey, to the question it belongs
 * to: the question's own text first, then its answer evidence, and a passage
 * block to its group's first question.
 */
export function blockOwners(
  sections: readonly ImportDraftSection[],
): Map<string, string> {
  const owners = new Map<string, string>();
  const claim = (refs: readonly ImportSourceRef[], questionId: string) => {
    for (const ref of refs) {
      const key = blockKey(ref.sourceId, ref.blockId);
      if (!owners.has(key)) owners.set(key, questionId);
    }
  };
  const places = questionPlaces(sections);
  for (const place of places) claim(place.question.source, place.question.id);
  for (const section of sections)
    for (const item of section.items) {
      const first = item.group?.questions[0];
      if (item.group && first) claim(item.group.source, first.id);
    }
  for (const place of places) claim(place.question.answer.evidence, place.question.id);
  return owners;
}

/** updateQuestion replaces one question immutably, keeping every untouched section, item and group by reference. */
export function updateQuestion(
  sections: ImportDraftSection[],
  questionId: string,
  update: (question: ImportDraftQuestion) => ImportDraftQuestion,
): ImportDraftSection[] {
  return sections.map((section) => {
    let changed = false;
    const items = section.items.map((item) => {
      if (item.question?.id === questionId) {
        changed = true;
        return { ...item, question: update(item.question) };
      }
      const group = item.group;
      if (group?.questions.some((question) => question.id === questionId)) {
        changed = true;
        return {
          ...item,
          group: {
            ...group,
            questions: group.questions.map((question) =>
              question.id === questionId ? update(question) : question,
            ),
          },
        };
      }
      return item;
    });
    return changed ? { ...section, items } : section;
  });
}

function entered(
  question: ImportDraftQuestion,
  ...fields: (keyof ImportFieldOrigins)[]
): ImportFieldOrigins {
  const origins = { ...question.origins };
  for (const field of fields) origins[field] = "teacher_entered";
  return origins;
}

/** isChoiceType reports whether a type is answered by marking options. */
export function isChoiceType(type: QuestionType): boolean {
  return (
    type === "single_choice" || type === "multiple_choice" || type === "true_false"
  );
}

function answer(
  state: ImportDraftAnswer["state"],
  optionIds: string[],
  evidence: ImportSourceRef[],
  text?: string,
): ImportDraftAnswer {
  return {
    state,
    optionIds,
    evidence,
    ...(text === undefined || text === "" ? {} : { text }),
  };
}

/** setCorrectOptions records the teacher's key for a choice question; an empty key is unknown, never "all false". */
export function setCorrectOptions(
  question: ImportDraftQuestion,
  optionIds: string[],
): ImportDraftQuestion {
  return {
    ...question,
    answer: answer(
      optionIds.length > 0 ? "known" : "unknown",
      optionIds,
      question.answer.evidence,
    ),
    origins: entered(question, "answer"),
  };
}

const LETTER = /^\(?([A-Ha-h])\)?$/;

function letterList(value: string): string[] | null {
  const tokens = value
    .replace(/\.$/, "")
    .split(/[\s,;/&]+/)
    .filter((token) => token !== "");
  const letters = tokens.map((token) => LETTER.exec(token)?.[1]?.toUpperCase());
  if (letters.length === 0 || letters.some((letter) => letter === undefined))
    return null;
  return [...new Set(letters as string[])];
}

const TRUTH: Readonly<Record<string, "true" | "false">> = {
  true: "true",
  t: "true",
  đúng: "true",
  false: "false",
  f: "false",
  sai: "false",
};

function optionText(option: ImportDraftOption): string {
  return contentPlainText(option.content).trim().toLocaleLowerCase("vi");
}

/** candidateOptions maps a conflicting source value onto option IDs the way recognition reads keys, or null when it names no option. */
export function candidateOptions(
  question: ImportDraftQuestion,
  candidate: ImportKeyValue,
): string[] | null {
  const value = candidate.value.trim();
  let ids: string[] = [];
  const truth = TRUTH[value.toLocaleLowerCase("vi").replace(/\.$/, "")];
  const letters = letterList(value);
  if (truth !== undefined && question.type === "true_false") {
    const found = question.options.find(
      (option) =>
        optionText(option) === truth ||
        TRUTH[optionText(option)] === truth ||
        option.label.toLowerCase() === truth[0],
    );
    ids = found ? [found.id] : [];
  } else if (letters !== null) {
    const found = letters.map(
      (letter) =>
        question.options.find((option) => option.label.toUpperCase() === letter)?.id,
    );
    if (found.some((id) => id === undefined)) return null;
    ids = found as string[];
  } else {
    const wanted = value.toLocaleLowerCase("vi");
    const found = question.options.find((option) => optionText(option) === wanted);
    ids = found ? [found.id] : [];
  }
  if (ids.length === 0) return null;
  if (question.type !== "multiple_choice" && ids.length !== 1) return null;
  return ids;
}

/** ACCEPTED_LIMITS are the contract's bounds on one blank's accepted answers: how many, and code points each. */
export const ACCEPTED_LIMITS = { count: 20, length: 500 } as const;

/** clampAccepted trims, de-duplicates and bounds accepted answers to ACCEPTED_LIMITS, dropping empty ones. */
export function clampAccepted(values: readonly string[]): string[] {
  const kept = values
    .map((value) => Array.from(value.trim()).slice(0, ACCEPTED_LIMITS.length).join(""))
    .filter((value) => value !== "");
  return [...new Set(kept)].slice(0, ACCEPTED_LIMITS.count);
}

function alternatives(value: string): string[] {
  return clampAccepted(value.split(/[/\n]/));
}

/**
 * pickCandidate settles a conflicting key on the value the teacher chose,
 * keeping that value's evidence. It returns null when the value cannot be
 * applied as it stands, and the teacher enters the key by hand instead.
 */
export function pickCandidate(
  question: ImportDraftQuestion,
  candidate: ImportKeyValue,
): ImportDraftQuestion | null {
  const origins = entered(question, "answer");
  if (isChoiceType(question.type)) {
    const ids = candidateOptions(question, candidate);
    if (ids === null) return null;
    return { ...question, answer: answer("known", ids, candidate.evidence), origins };
  }
  if (question.type === "short_answer") {
    const text = candidate.value.trim();
    if (text === "") return null;
    return {
      ...question,
      answer: answer("known", [], candidate.evidence, text),
      origins,
    };
  }
  if (question.type === "fill_blank" && question.blanks.length === 1) {
    const accepted = alternatives(candidate.value);
    if (accepted.length === 0) return null;
    return {
      ...question,
      blanks: [{ ...question.blanks[0]!, accepted }],
      answer: answer("known", [], candidate.evidence),
      origins,
    };
  }
  return null;
}

/** setBlanks records the teacher's accepted answers; the key is known once every blank has one. */
export function setBlanks(
  question: ImportDraftQuestion,
  blanks: ImportDraftBlank[],
): ImportDraftQuestion {
  const complete =
    blanks.length > 0 && blanks.every((blank) => blank.accepted.length > 0);
  let state: ImportDraftAnswer["state"] = "unknown";
  if (complete) state = "known";
  else if (question.answer.state === "conflict") state = "conflict";
  return {
    ...question,
    blanks,
    answer: {
      ...answer(state, [], question.answer.evidence),
      ...(state === "conflict" && question.answer.candidates
        ? { candidates: question.answer.candidates }
        : {}),
    },
    origins: entered(question, "answer"),
  };
}

/** setSample records a short answer's sample; an empty sample leaves the key unknown. */
export function setSample(
  question: ImportDraftQuestion,
  text: string,
): ImportDraftQuestion {
  return {
    ...question,
    answer: answer(
      text.trim() === "" ? "unknown" : "known",
      [],
      question.answer.evidence,
      text,
    ),
    origins: entered(question, "answer"),
  };
}

function promptGaps(prompt: ContentDocument): { id: string; label: string }[] {
  return prompt.format === "semantic_v1" ? questionGaps(prompt) : [];
}

/** BLANK_LIMIT is the contract's bound on the blanks, and so the prompt gaps, of one fill-blank question. */
export const BLANK_LIMIT = 20;

/** exceedsBlankLimit reports whether a prompt has more gaps than a fill-blank question may bind. */
export function exceedsBlankLimit(prompt: ContentDocument): boolean {
  return promptGaps(prompt).length > BLANK_LIMIT;
}

function blanksForGaps(
  gaps: readonly { id: string; label: string }[],
  blanks: readonly ImportDraftBlank[],
): ImportDraftBlank[] {
  const byGap = new Map(blanks.map((blank) => [blank.gapId, blank]));
  return gaps.slice(0, BLANK_LIMIT).map(
    (gap) =>
      byGap.get(gap.id) ?? {
        gapId: gap.id,
        label: gap.label,
        accepted: [],
        caseSensitive: false,
      },
  );
}

/**
 * setPrompt replaces a question's prompt and keeps a fill-blank question's
 * blanks bound to the prompt's gaps. A fill-blank prompt over BLANK_LIMIT
 * gaps is refused and the question is returned unchanged.
 */
export function setPrompt(
  question: ImportDraftQuestion,
  prompt: ContentDocument,
): ImportDraftQuestion {
  if (question.type === "fill_blank" && exceedsBlankLimit(prompt)) return question;
  const next = { ...question, prompt, origins: entered(question, "prompt") };
  if (question.type !== "fill_blank") return next;
  const before = promptGaps(question.prompt).map((gap) => gap.id);
  const after = promptGaps(prompt);
  if (before.join("\u0000") === after.map((gap) => gap.id).join("\u0000")) return next;
  return setBlanks(next, blanksForGaps(after, question.blanks));
}

/** setOptionContent replaces one option's content; the option keeps its ID and its place in the key. */
export function setOptionContent(
  question: ImportDraftQuestion,
  optionId: string,
  content: ContentDocument,
): ImportDraftQuestion {
  return {
    ...question,
    options: question.options.map((option) =>
      option.id === optionId ? { ...option, content } : option,
    ),
    origins: entered(question, "options"),
  };
}

const LABELS = "ABCDEFGH";

/** addOption appends an empty option under the next unused letter. */
export function addOption(question: ImportDraftQuestion): ImportDraftQuestion {
  const used = new Set(question.options.map((option) => option.label.toUpperCase()));
  const label =
    [...LABELS].find((letter) => !used.has(letter)) ??
    String(question.options.length + 1);
  return {
    ...question,
    options: [
      ...question.options,
      {
        id: crypto.randomUUID(),
        label,
        content: {
          format: "semantic_v1",
          blocks: [{ type: "paragraph", content: [] }],
        },
      },
    ],
    origins: entered(question, "options"),
  };
}

/** removeOption drops one option and any key that pointed at it. */
export function removeOption(
  question: ImportDraftQuestion,
  optionId: string,
): ImportDraftQuestion {
  const options = question.options.filter((option) => option.id !== optionId);
  const optionIds = question.answer.optionIds.filter((id) => id !== optionId);
  if (optionIds.length === question.answer.optionIds.length)
    return { ...question, options, origins: entered(question, "options") };
  const state = optionIds.length > 0 ? "known" : "unknown";
  return {
    ...question,
    options,
    answer: answer(state, optionIds, question.answer.evidence),
    origins: entered(question, "options", "answer"),
  };
}

/** TrueFalseText is the option text a question converted to true/false starts with, in the teacher's language. */
export interface TrueFalseText {
  truth: string;
  falsity: string;
}

function trueFalseOptions(text: TrueFalseText): ImportDraftOption[] {
  const option = (label: string, text: string): ImportDraftOption => ({
    id: crypto.randomUUID(),
    label,
    content: {
      format: "semantic_v1",
      blocks: [{ type: "paragraph", content: [{ type: "text", text, marks: [] }] }],
    },
  });
  return [option("T", text.truth), option("F", text.falsity)];
}

/**
 * changeType converts a question to another interaction, keeping every piece
 * of answer data the target can still represent. Callers compare the result
 * with dropsAnswerData before applying it.
 */
export function changeType(
  question: ImportDraftQuestion,
  type: QuestionType,
  trueFalse: TrueFalseText,
): ImportDraftQuestion {
  if (type === question.type) return question;
  const base = { ...question, type, origins: entered(question, "type") };
  const evidence = question.answer.evidence;
  if (type === "fill_blank") {
    const blanks = blanksForGaps(promptGaps(question.prompt), question.blanks);
    const complete =
      blanks.length > 0 && blanks.every((blank) => blank.accepted.length > 0);
    return {
      ...base,
      options: [],
      blanks,
      answer: answer(complete ? "known" : "unknown", [], evidence),
    };
  }
  if (type === "short_answer") {
    const text = question.answer.text ?? "";
    return {
      ...base,
      options: [],
      blanks: [],
      answer: answer(text.trim() === "" ? "unknown" : "known", [], evidence, text),
    };
  }
  if (type === "true_false" && question.options.length !== 2) {
    return {
      ...base,
      options: trueFalseOptions(trueFalse),
      blanks: [],
      answer: answer("unknown", [], evidence),
      origins: entered(question, "type", "options"),
    };
  }
  const keepConflict =
    question.answer.state === "conflict" && isChoiceType(question.type);
  if (keepConflict) return { ...base, blanks: [] };
  const optionIds =
    type === "multiple_choice" || question.answer.optionIds.length <= 1
      ? question.answer.optionIds
      : [];
  return {
    ...base,
    blanks: [],
    answer: answer(optionIds.length > 0 ? "known" : "unknown", optionIds, evidence),
  };
}

/** dropsAnswerData reports whether moving from one version of a question to the next discards options or key data. */
export function dropsAnswerData(
  before: ImportDraftQuestion,
  after: ImportDraftQuestion,
): boolean {
  const kept = new Set(after.options.map((option) => option.id));
  if (before.options.some((option) => !kept.has(option.id))) return true;
  const keys = new Set(after.answer.optionIds);
  if (before.answer.optionIds.some((id) => !keys.has(id))) return true;
  const accepted = new Map(after.blanks.map((blank) => [blank.gapId, blank.accepted]));
  if (
    before.blanks.some(
      (blank) =>
        blank.accepted.length > 0 &&
        (accepted.get(blank.gapId) ?? []).length < blank.accepted.length,
    )
  )
    return true;
  if ((before.answer.candidates?.length ?? 0) > (after.answer.candidates?.length ?? 0))
    return true;
  return (before.answer.text ?? "").trim() !== "" && after.answer.text === undefined;
}

/** setPoints records the teacher's score for one question. */
export function setPoints(
  question: ImportDraftQuestion,
  points: string,
): ImportDraftQuestion {
  return { ...question, points, origins: entered(question, "points") };
}

/** exclude leaves a question out of the draft test, with the teacher's reason. */
export function exclude(
  question: ImportDraftQuestion,
  reason: string,
): ImportDraftQuestion {
  return { ...question, excluded: { reason } };
}

/** restore puts an excluded question back. */
export function restore(question: ImportDraftQuestion): ImportDraftQuestion {
  const next = { ...question };
  delete next.excluded;
  return next;
}

/** POINTS_PATTERN accepts the decimal scores the server stores: positive, at most two decimals. */
export const POINTS_PATTERN = /^(?:0|[1-9]\d{0,5})(?:\.\d{1,2})?$/;

/** validPoints reports whether a typed score can be sent. */
export function validPoints(value: string): boolean {
  return POINTS_PATTERN.test(value) && Number(value) > 0;
}
