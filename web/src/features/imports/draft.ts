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
  "tfng",
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
    type === "single_choice" ||
    type === "multiple_choice" ||
    type === "true_false" ||
    type === "tfng"
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

/** TFNG_LABELS are a tfng question's option labels, in the bank's fixed order: True, False, Not given. */
export const TFNG_LABELS = ["T", "F", "NG"] as const;

const TFNG: Readonly<Record<string, (typeof TFNG_LABELS)[number]>> = {
  true: "T",
  t: "T",
  đúng: "T",
  false: "F",
  f: "F",
  sai: "F",
  "not given": "NG",
  ng: "NG",
  "không có thông tin": "NG",
};

function optionText(option: ImportDraftOption): string {
  return contentPlainText(option.content).trim().toLocaleLowerCase("vi");
}

function tfngCandidate(question: ImportDraftQuestion, value: string): string[] | null {
  const label =
    TFNG[value.toLocaleLowerCase("vi").replace(/\.$/, "").replace(/\s+/g, " ")];
  const found = question.options.find((option) => option.label === label);
  return found ? [found.id] : null;
}

/** candidateOptions maps a conflicting source value onto option IDs the way recognition reads keys, or null when it names no option. */
export function candidateOptions(
  question: ImportDraftQuestion,
  candidate: ImportKeyValue,
): string[] | null {
  const value = candidate.value.trim();
  if (question.type === "tfng") return tfngCandidate(question, value);
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

/** withinChoices drops a "Choose N" count that the question's options can no longer hold. */
function withinChoices(question: ImportDraftQuestion): ImportDraftQuestion {
  if (typeof question.selectCount !== "number" || !selectCountOutOfRange(question))
    return question;
  return setSelectCount(question, null);
}

/** removeOption drops one option and any key that pointed at it, and a "Choose N" count the options left cannot hold. */
export function removeOption(
  question: ImportDraftQuestion,
  optionId: string,
): ImportDraftQuestion {
  const options = question.options.filter((option) => option.id !== optionId);
  const optionIds = question.answer.optionIds.filter((id) => id !== optionId);
  if (optionIds.length === question.answer.optionIds.length)
    return withinChoices({
      ...question,
      options,
      origins: entered(question, "options"),
    });
  const state = optionIds.length > 0 ? "known" : "unknown";
  return withinChoices({
    ...question,
    options,
    answer: answer(state, optionIds, question.answer.evidence),
    origins: entered(question, "options", "answer"),
  });
}

function fixedOption(label: string, text: string): ImportDraftOption {
  return {
    id: crypto.randomUUID(),
    label,
    content: {
      format: "semantic_v1",
      blocks: [{ type: "paragraph", content: [{ type: "text", text, marks: [] }] }],
    },
  };
}

function trueFalseOptions(): ImportDraftOption[] {
  return [fixedOption("T", "True"), fixedOption("F", "False")];
}

function tfngOptions(): ImportDraftOption[] {
  return [
    fixedOption("T", "True"),
    fixedOption("F", "False"),
    fixedOption("NG", "Not given"),
  ];
}

/** isTFNGShaped reports whether a question already holds the three tfng options in order. */
function isTFNGShaped(question: ImportDraftQuestion): boolean {
  return (
    question.options.length === TFNG_LABELS.length &&
    question.options.every((option, i) => option.label === TFNG_LABELS[i])
  );
}

/** truthLabel reads a true/false or tfng option as T, F or NG, or undefined when it is neither. */
function truthLabel(option: ImportDraftOption): string | undefined {
  if ((TFNG_LABELS as readonly string[]).includes(option.label)) return option.label;
  const truth = TRUTH[optionText(option)];
  if (truth === undefined) return undefined;
  return truth === "true" ? "T" : "F";
}

/** carriedKey maps a true/false or tfng key onto the fixed options of the other type, when they hold the same value. */
function carriedKey(
  question: ImportDraftQuestion,
  fixed: ImportDraftOption[],
): string[] {
  if (question.answer.state !== "known") return [];
  if (question.type !== "true_false" && question.type !== "tfng") return [];
  const ids = question.answer.optionIds.flatMap((id) => {
    const option = question.options.find((candidate) => candidate.id === id);
    const label = option === undefined ? undefined : truthLabel(option);
    const target = fixed.find((candidate) => candidate.label === label);
    return target === undefined ? [] : [target.id];
  });
  return ids.length === 1 ? ids : [];
}

/** fixedOptions are the canonical options a conversion to true/false or tfng needs, or null when the question already has them. */
function fixedOptions(
  question: ImportDraftQuestion,
  type: QuestionType,
): ImportDraftOption[] | null {
  if (type === "true_false" && question.options.length !== 2) return trueFalseOptions();
  if (type === "tfng" && !isTFNGShaped(question)) return tfngOptions();
  return null;
}

/** withoutTypeFields drops selectCount and maxWords where the type cannot carry them. */
function withoutTypeFields(question: ImportDraftQuestion): ImportDraftQuestion {
  const next = { ...question };
  if (next.type !== "multiple_choice") delete next.selectCount;
  if (next.type !== "short_answer") delete next.maxWords;
  return next;
}

/**
 * changeType converts a question to another interaction, keeping every piece
 * of answer data the target can still represent. A question converted to
 * true/false gets the canonical options "True" and "False", and one
 * converted to tfng gets "True", "False" and "Not given", which every screen
 * reads in the reader's language. A "Choose N" count stays only on a multiple
 * choice and a word limit only on a short answer. Callers compare the result
 * with dropsAnswerData before applying it.
 */
export function changeType(
  question: ImportDraftQuestion,
  type: QuestionType,
): ImportDraftQuestion {
  if (type === question.type) return question;
  const base = withoutTypeFields({
    ...question,
    type,
    origins: entered(question, "type"),
  });
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
  const fixed = fixedOptions(question, type);
  if (fixed !== null) {
    const carried = carriedKey(question, fixed);
    return {
      ...base,
      options: fixed,
      blanks: [],
      answer: answer(carried.length > 0 ? "known" : "unknown", carried, evidence),
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

/** dropsTypeFields reports whether a "Choose N" count or a word limit is lost between two versions of a question. */
function dropsTypeFields(
  before: ImportDraftQuestion,
  after: ImportDraftQuestion,
): boolean {
  return (
    (typeof before.selectCount === "number" && after.selectCount === undefined) ||
    (typeof before.maxWords === "number" && after.maxWords === undefined)
  );
}

function isFixedType(type: QuestionType): boolean {
  return type === "true_false" || type === "tfng";
}

/** hasStandardOptions reports whether every option reads as True, False or Not given, so rebuilding them loses no text the teacher wrote. */
function hasStandardOptions(question: ImportDraftQuestion): boolean {
  return question.options.every((option) => TFNG[optionText(option)] !== undefined);
}

/** dropsAnswerData reports whether moving from one version of a question to the next discards options, key data, a "Choose N" count or a word limit. Between true/false and tfng with standard option texts, only a key or a conflict that does not carry is lost. */
export function dropsAnswerData(
  before: ImportDraftQuestion,
  after: ImportDraftQuestion,
): boolean {
  if (
    isFixedType(before.type) &&
    isFixedType(after.type) &&
    before.type !== after.type &&
    hasStandardOptions(before)
  )
    return before.answer.state !== "unknown" && after.answer.state !== "known";
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
  if (dropsTypeFields(before, after)) return true;
  return (before.answer.text ?? "").trim() !== "" && after.answer.text === undefined;
}

/** SELECT_COUNT_LIMITS are the contract's bounds on a "Choose N" count. */
export const SELECT_COUNT_LIMITS = { min: 2, max: 7 } as const;

/** selectCountChoices lists the counts a multiple choice with these options may ask for: 2 up to one less than its options. */
export function selectCountChoices(question: ImportDraftQuestion): number[] {
  const top = Math.min(SELECT_COUNT_LIMITS.max, question.options.length - 1);
  const out: number[] = [];
  for (let n = SELECT_COUNT_LIMITS.min; n <= top; n++) out.push(n);
  return out;
}

/** setSelectCount records a multiple choice's "Choose N"; null makes it a plain multiple choice again. */
export function setSelectCount(
  question: ImportDraftQuestion,
  count: number | null,
): ImportDraftQuestion {
  const next = { ...question };
  if (count === null) delete next.selectCount;
  else next.selectCount = count;
  return next;
}

/** selectCountOutOfRange reports a "Choose N" count the question's options cannot hold: N must stay below the number of options. */
export function selectCountOutOfRange(question: ImportDraftQuestion): boolean {
  return (
    typeof question.selectCount === "number" &&
    !selectCountChoices(question).includes(question.selectCount)
  );
}

/** selectCountMismatch reports a "Choose N" question whose key marks a different number of options than N. */
export function selectCountMismatch(question: ImportDraftQuestion): boolean {
  return (
    question.type === "multiple_choice" &&
    typeof question.selectCount === "number" &&
    question.answer.state === "known" &&
    question.answer.optionIds.length !== question.selectCount
  );
}

/** MAX_WORDS_LIMITS are the contract's bounds on a short answer's word limit. */
export const MAX_WORDS_LIMITS = { min: 1, max: 100 } as const;

/** validMaxWords reports whether a typed word limit can be sent; an empty field means no limit. */
export function validMaxWords(value: string): boolean {
  if (value.trim() === "") return true;
  if (!/^\d{1,3}$/.test(value.trim())) return false;
  const n = Number(value.trim());
  return n >= MAX_WORDS_LIMITS.min && n <= MAX_WORDS_LIMITS.max;
}

/** setMaxWords records a short answer's word limit; null removes it. */
export function setMaxWords(
  question: ImportDraftQuestion,
  limit: number | null,
): ImportDraftQuestion {
  const next = { ...question };
  if (limit === null) delete next.maxWords;
  else next.maxWords = limit;
  return next;
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
