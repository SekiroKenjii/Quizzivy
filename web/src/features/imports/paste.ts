/**
 * PasteScan is the quick count of a pasted test, as "Found so far" shows it:
 * the parts, the questions, how many have an answer, the labels of those
 * that have none, in order, and the numbers a part uses twice.
 */
export interface PasteScan {
  sections: number;
  questions: number;
  answered: number;
  missing: string[];
  duplicates: number[];
}

/**
 * PASTE_EXAMPLE is the example "How to lay out pasted text" shows and copies:
 * the `page-example` case of `api/testdata/pasted-text-counts.json`, which
 * has an answer for every question (DG-15).
 */
export const PASTE_EXAMPLE = [
  "Part 1. Choose the best answer.",
  "1. She ___ in Hanoi since 2019.",
  "A. lives",
  "B. has lived *",
  "C. lived",
  "",
  "Part 2. True, False or Not given.",
  "2. Parks are cheaper than roads.",
  "Answer: Not given",
  "",
  "Part 3. Write ONE word in each gap.",
  "3. We have lived here ___ 2015.",
  "4. She has worked here ___ three years.",
  "",
  "Answer key: 3-since 4-for",
].join("\n");

interface Label {
  text: string;
  number: number;
  sub: number;
  keyword: string;
  end: number;
}

interface Option {
  letter: string;
  text: string;
  star: boolean;
}

interface Value {
  value: string;
  ambiguous: boolean;
}

interface Group {
  label: string;
  instruction: string;
  stimulus: string[];
  questions: Question[];
  accepting: boolean;
}

interface Question {
  label: Label;
  instruction: string;
  stem: string[];
  options: Option[];
  keys: Value[];
  section: Section | null;
  group: Group | null;
}

interface Item {
  question?: Question;
  group?: Group;
}

interface Section {
  ordinal: number;
  tasks: number;
  items: Item[];
}

interface KeyEntry {
  label: Label;
  value: string;
  section: number;
}

interface Span {
  at: number;
  start: number;
  end: number;
  letter: string;
}

type Owner = Question | Group | Section | null;

type State = "known" | "unknown" | "conflict" | "unsupported";

const MAX_JUMP = 20;
const MAX_TITLE = 200;
const TITLE_MAX = 120;

const TASK_TRUE_FALSE = 1;
const TASK_NO_BLANKS = 2;

const KEYWORD_LABEL = /^\s*(question|câu|cau|q)\s*(\d{1,3})(?:\.(\d{1,2}))?\s*[.:)]?/iu;
const BARE_LABEL = /^\s*(\d{1,3})(?:\.(\d{1,2}))?\s*[.)]/u;
const SUB_LABEL = /^\s*(\d{1,3})\.(\d{1,2})(?:\s|$)/u;
const COLON_LABEL = /^\s*(\d{1,3})(?:\.(\d{1,2}))?\s*:/u;
const ROMAN_LABEL = /^\s*([IVXL]{1,6})\s*[.):]/u;
const BARE_ROMAN = /^\s*([IVXL]{1,6})\s*$/u;
const SECTION_WORD =
  /^\s*(?:part|section|phần|phan|bài tập|bai tap|bài|bai|exercise|task)\s+/iu;
const SECTION_MARK = /^(\d{1,2}|[ivx]{1,5}|[a-h])(?=[\s.:)\-–]|$)/iu;
const PAPER_NAMES = [
  /^đề(?: ?thi)?(?: ?(?:số|so))? ?(\d{1,3})\b/iu,
  /^de ?so ?(\d{1,3})\b/iu,
  /^(?:test|paper|exam) ?(\d{1,3})\b/iu,
];
const ANSWER_PREFIX = /^(?:đáp ?án|dap ?an|answer ?key|answers?) ?(?:của ?)?/iu;
const KEY_HEADING = /^(?:đáp\s*án|dap\s*an|answer\s*keys?|answers|keys?)\s*[:.]?$/iu;
const ANSWER_LINE = /^\s*(?:answer\s*[:-]|(?:ans|đáp\s*án|key)\s*:)\s*\S/iu;
const INLINE_ANSWER = /(?:^|\s)(?:answer\s*[:-]|(?:ans|đáp\s*án|key)\s*:)(.*)$/iu;
const MULTI_KEY_PREFIX = /^\s*(?:answer\s*key|answers|key|đáp\s*án)\s*:\s*/iu;
const MULTI_KEY_PAIR = /(?:^|\s)(\d{1,3}(?:\.\d{1,2})?)\s*[-:.)]\s*/gu;
const IMPERATIVE = new RegExp(
  String.raw`^[^\p{L}]*(?:` +
    [
      "choose",
      "circle",
      "find",
      "read",
      "complete",
      "write",
      "rewrite",
      "finish",
      "identify",
      "decide",
      "match",
      "fill",
      "give",
      "put",
      "listen",
      "mark",
      "use",
      "make",
      "supply",
      "change",
      "select",
      "underline",
      "pick",
      "answer",
      "look",
      "arrange",
      "combine",
      "correct",
      "each of the following",
      "chọn",
      "tìm",
      "đọc",
      "hoàn thành",
      "viết",
      "điền",
      "sắp xếp",
      "nối",
    ].join("|") +
    String.raw`)\b`,
  "iu",
);
const TRUE_FALSE_TASK =
  /true\s*(?:or|\/)\s*false|\bT\s*\/\s*F\b|đúng\s*(?:hay|hoặc|\/)\s*sai/iu;
const NO_BLANK_TASKS = [
  /mistake|needs? correction|\berrors?\b|incorrect|sửa lỗi/iu,
  /means? the same|closest in meaning|rewrite|finish the (?:second )?sentence|viết lại/iu,
  /write (?:complete|full)? ?sentences|given (?:words|clues|cues)|words given|from the (?:words|cues)/iu,
  /rearrange|reorder|in the correct order|meaningful sentences|make questions|sắp xếp/iu,
];
const FUNCTION_WORDS = new Set([
  "a",
  "an",
  "the",
  "and",
  "or",
  "but",
  "of",
  "to",
  "in",
  "on",
  "at",
  "by",
  "for",
  "from",
  "with",
  "between",
  "as",
  "than",
  "that",
  "is",
  "are",
]);

const ROMAN: Record<string, number> = { I: 1, V: 5, X: 10, L: 50 };

/** normalizePaste is the text the page sends: NFC, without a leading BOM, with LF line ends. */
export function normalizePaste(text: string): string {
  return text
    .normalize("NFC")
    .replace(/^\uFEFF/u, "")
    .replace(/\r\n?/gu, "\n");
}

/** pasteLength counts the code points of the NFC text, which is what the server limits. */
export function pasteLength(text: string): number {
  return Array.from(text.normalize("NFC")).length;
}

function matchable(line: string): string {
  return line.replace(/[^\S\t\n]/gu, " ");
}

function lines(text: string): string[] {
  return normalizePaste(text)
    .split("\n")
    .map(matchable)
    .filter((line) => line.trim() !== "");
}

function collapsed(text: string): string {
  return text.replace(/\s+/gu, " ");
}

function paperAt(text: string, title: boolean): number | null {
  for (const name of PAPER_NAMES) {
    const match = name.exec(text);
    if (!match) continue;
    const word = match[0].toLowerCase();
    if (!title && (word.startsWith("exam") || /^đề ?thi/u.test(word))) continue;
    return Number(match[1]);
  }
  return null;
}

function paperTitle(text: string): number | null {
  const words = collapsed(text);
  for (let at = 0; at < words.length; at++) {
    if (at > 0 && words[at - 1] !== " ") continue;
    const paper = paperAt(words.slice(at), true);
    if (paper !== null) return paper;
  }
  return null;
}

function paperHeading(line: string): number | null {
  const text = collapsed(line).replace(/^ /u, "");
  const prefix = ANSWER_PREFIX.exec(text);
  const afterPrefix = prefix ? paperAt(text.slice(prefix[0].length), false) : null;
  return afterPrefix ?? paperAt(text, false);
}

function visible(text: string): boolean {
  return /\S/u.test(text);
}

function isSpace(char: string | undefined): boolean {
  return char !== undefined && /\s/u.test(char);
}

function isDigit(char: string | undefined): boolean {
  return char !== undefined && char >= "0" && char <= "9";
}

function skipSpace(text: string, from: number): number {
  let at = from;
  while (at < text.length && isSpace(text[at])) at++;
  return at;
}

function toRoman(value: number): string {
  const steps: [number, string][] = [
    [50, "L"],
    [40, "XL"],
    [10, "X"],
    [9, "IX"],
    [5, "V"],
    [4, "IV"],
    [1, "I"],
  ];
  let out = "";
  let rest = value;
  for (const [step, symbol] of steps) for (; rest >= step; rest -= step) out += symbol;
  return out;
}

function romanValue(text: string): number | null {
  let total = 0;
  for (let i = 0; i < text.length; i++) {
    const value = ROMAN[text[i]!]!;
    const next = ROMAN[text[i + 1] ?? ""] ?? 0;
    total += value < next ? -value : value;
  }
  return total > 0 && toRoman(total) === text ? total : null;
}

function romanHeading(
  line: string,
  bare: boolean,
): { value: number; end: number } | null {
  const marked = ROMAN_LABEL.exec(line);
  const markedValue = marked ? romanValue(marked[1]!) : null;
  if (marked && markedValue !== null)
    return { value: markedValue, end: marked[0].length };
  const plain = bare ? BARE_ROMAN.exec(line) : null;
  const plainValue = plain ? romanValue(plain[1]!) : null;
  return plain && plainValue !== null
    ? { value: plainValue, end: plain[0].length }
    : null;
}

function namedSection(line: string): number | null {
  const word = SECTION_WORD.exec(line);
  const mark = word ? SECTION_MARK.exec(line.slice(word[0].length)) : null;
  if (!mark) return null;
  return /^\d+$/u.test(mark[1]!) ? Number(mark[1]) : 0;
}

function toLabel(match: RegExpExecArray, keyword: string, offset: number): Label {
  const number = Number(match[1 + offset]);
  const subText = match[2 + offset];
  const sub = subText === undefined ? 0 : Number(subText);
  const text = subText === undefined ? String(number) : `${number}.${sub}`;
  return { text, number, sub, keyword, end: match[0].length };
}

function questionLabel(line: string): Label | null {
  const keyword = KEYWORD_LABEL.exec(line);
  if (keyword) return toLabel(keyword, keyword[1]!.toLowerCase(), 1);
  const bare = BARE_LABEL.exec(line);
  if (bare && !isDigit(line[bare[0].length])) return toLabel(bare, "", 0);
  const sub = SUB_LABEL.exec(line);
  return sub ? toLabel(sub, "", 0) : null;
}

function textLabel(line: string): Label | null {
  const label = questionLabel(line);
  if (label) return label;
  const colon = COLON_LABEL.exec(line);
  return colon ? toLabel(colon, "", 0) : null;
}

function isInstruction(text: string): boolean {
  const trimmed = text.trim();
  const first = /\p{L}/u.exec(trimmed);
  return first !== null && /\p{Lu}/u.test(first[0]) && IMPERATIVE.test(trimmed);
}

function trimChars(text: string, chars: string): string {
  let start = 0;
  let end = text.length;
  while (start < end && chars.includes(text[start]!)) start++;
  while (end > start && chars.includes(text[end - 1]!)) end--;
  return text.slice(start, end);
}

function normalizedInstruction(text: string): string {
  return trimChars(text, " .:;-–")
    .split(/\s+/u)
    .filter((word) => word !== "")
    .join(" ")
    .toLowerCase();
}

function tasksOf(text: string): number {
  let tasks = TRUE_FALSE_TASK.test(text) ? TASK_TRUE_FALSE : 0;
  if (NO_BLANK_TASKS.some((pattern) => pattern.test(text))) tasks |= TASK_NO_BLANKS;
  return tasks;
}

function softWrapped(previous: string, next: string): boolean {
  if (previous === "" || next === "") return false;
  const words = previous.split(/\s+/u);
  if (FUNCTION_WORDS.has(words[words.length - 1]!.toLowerCase())) return true;
  const last = Array.from(previous).at(-1)!;
  const first = Array.from(next)[0]!;
  if ('.?!:;"”’)'.includes(last) || /\p{Lu}/u.test(first)) return false;
  if (first === "(") return isDigit(Array.from(next)[1]);
  return !"-–—•*\"“‘'".includes(first);
}

function optionAt(text: string, at: number, from: number, letter: string): boolean {
  const mark = text[at + 1];
  if (text[at]?.toUpperCase() !== letter || (mark !== "." && mark !== ")"))
    return false;
  if (at > from && !isSpace(text[at - 1]) && text[at - 1] !== "(") return false;
  return !isDigit(text[at + 2]);
}

function nextLetter(letter: string): string {
  return String.fromCharCode(letter.charCodeAt(0) + 1);
}

function scanOptions(text: string, from: number, first: string): Span[] {
  const out: Span[] = [];
  let letter = first;
  let at = from;
  while (at < text.length) {
    if (!optionAt(text, at, from, letter)) {
      at++;
      continue;
    }
    const previous = out.at(-1);
    if (previous) previous.end = at;
    const start = skipSpace(text, at + 2);
    out.push({ at, start, end: text.length, letter });
    letter = nextLetter(letter);
    at = start;
  }
  return out;
}

function toOption(text: string, span: Span): Option {
  let content = text.slice(span.start, span.end).trim();
  let star = false;
  if (content.startsWith("*")) {
    star = true;
    content = content.slice(1).trim();
  }
  if (content.endsWith("*")) {
    star = true;
    content = content.slice(0, -1).trim();
  }
  return { letter: span.letter, text: content, star };
}

function gapCount(text: string): number {
  let count = 0;
  for (let at = 0; at < text.length;) {
    if (!"….".includes(text[at]!) && text[at] !== "_") {
      at++;
      continue;
    }
    let weight = 0;
    let underscores = true;
    for (; at < text.length && ("….".includes(text[at]!) || text[at] === "_"); at++) {
      underscores = underscores && text[at] === "_";
      weight += text[at] === "…" ? 3 : 1;
    }
    if (weight >= 4 || (underscores && weight >= 3)) count++;
  }
  return count;
}

function labelFollows(previous: Label, next: Label): boolean {
  if (next.keyword !== "") return true;
  if (previous.sub > 0 && next.number === previous.number)
    return next.sub === previous.sub + 1;
  return next.sub === 0 && next.number === previous.number + 1;
}

function prefixedKeyEntries(line: string, from: number): KeyEntry[] {
  const text = line.slice(from);
  const prefix = MULTI_KEY_PREFIX.exec(text);
  if (!prefix) return [];
  const value = text.slice(prefix[0].length);
  const pairs = [...value.matchAll(MULTI_KEY_PAIR)];
  if (pairs.length < 2 || pairs[0]!.index !== 0) return [];
  const entries: KeyEntry[] = [];
  for (const [i, pair] of pairs.entries()) {
    const label = questionLabel(`${pair[1]}.`);
    const answer = value
      .slice(pair.index + pair[0].length, pairs[i + 1]?.index ?? value.length)
      .trim();
    if (!label || answer === "") return [];
    entries.push({ label, value: answer, section: 0 });
  }
  return entries;
}

function skipKeyPunctuation(text: string, from: number, to: number): number {
  let at = from;
  while (at < to && (isSpace(text[at]) || ".:)".includes(text[at]!))) at++;
  return at;
}

function nextKeyLabel(
  text: string,
  from: number,
  to: number,
  current: Label,
): { at: number; label: Label } | null {
  for (let at = from + 1; at < to; at++) {
    if (!isSpace(text[at - 1]) || !/[\dQqCc]/u.test(text[at]!)) continue;
    const label = questionLabel(text.slice(at, to));
    if (label && labelFollows(current, label)) return { at, label };
  }
  return null;
}

function chunkEntries(text: string, from: number, to: number): KeyEntry[] {
  const out: KeyEntry[] = [];
  let at = skipSpace(text, from);
  let label = questionLabel(text.slice(at, to));
  while (label) {
    const valueStart = at + label.end;
    const next = nextKeyLabel(text, valueStart, to, label);
    const stop = next?.at ?? to;
    const value = text.slice(skipKeyPunctuation(text, valueStart, stop), stop).trim();
    if (value !== "") out.push({ label, value, section: 0 });
    if (!next) break;
    at = next.at;
    label = next.label;
  }
  return out;
}

function lineEntries(line: string, from: number): KeyEntry[] {
  const out: KeyEntry[] = [];
  let start = from;
  for (let at = from; at <= line.length; at++) {
    if (at < line.length && line[at] !== "\t") continue;
    if (visible(line.slice(start, at))) out.push(...chunkEntries(line, start, at));
    start = at + 1;
  }
  return out;
}

class KeyBook {
  readonly papers = new Map<number, KeyEntry[]>();
  private current: KeyEntry[];
  private section = 0;

  constructor(paper: number) {
    this.current = this.paper(paper);
  }

  paper(number: number): KeyEntry[] {
    const existing = this.papers.get(number);
    if (existing) return existing;
    const created: KeyEntry[] = [];
    this.papers.set(number, created);
    return created;
  }

  read(line: string): void {
    const heading = paperHeading(line);
    if (heading !== null) {
      this.current = this.paper(heading);
      this.section = 0;
      return;
    }
    if (KEY_HEADING.test(line.trim())) return;
    const roman = romanHeading(line, false);
    if (roman) this.section = roman.value;
    const from = roman?.end ?? 0;
    const prefixed = prefixedKeyEntries(line, from);
    const entries = prefixed.length > 0 ? prefixed : lineEntries(line, from);
    if (entries.length === 0) {
      this.continueLast(line.slice(from));
      return;
    }
    for (const entry of entries) entry.section = this.section;
    this.current.push(...entries);
  }

  choose(examPaper: number): KeyEntry[] {
    if (this.papers.size === 1) return [...this.papers.values()][0]!;
    return examPaper > 0 ? (this.papers.get(examPaper) ?? []) : [];
  }

  private continueLast(rest: string): void {
    const last = this.current.at(-1);
    if (last && visible(rest)) last.value += `\n${rest.trim()}`;
  }
}

function trueFalseValue(value: string): "T" | "F" | null {
  const word = trimChars(value, " .").toLowerCase();
  if (["t", "true", "đúng", "dung"].includes(word)) return "T";
  if (["f", "false", "sai"].includes(word)) return "F";
  return null;
}

function isTrueFalseWord(value: string): boolean {
  return ["true", "false", "đúng", "sai"].includes(
    trimChars(value, " .").toLowerCase(),
  );
}

function isNotGiven(value: string): boolean {
  return value.trim().toLowerCase() === "not given";
}

function letterToken(text: string, at: number): number {
  let end = at;
  if (text[end] === "(") end++;
  if (!/[A-Ha-h]/u.test(text[end] ?? "")) return -1;
  end++;
  if (text[end] === ")") end++;
  return end;
}

function letterSeparator(text: string, at: number): number {
  let end = skipSpace(text, at);
  if (",;/&".includes(text[end] ?? " ")) end = skipSpace(text, end + 1);
  return end;
}

function isLetter(value: string): boolean {
  const text = value.trim().replace(/\.$/u, "");
  let at = letterToken(text, 0);
  while (at > 0 && at < text.length) {
    const next = letterSeparator(text, at);
    at = next > at ? letterToken(text, next) : -1;
  }
  return at === text.length && text !== "";
}

function letters(value: string): string[] {
  return [...new Set(value.toUpperCase().match(/[A-H]/gu) ?? [])].sort((a, b) =>
    a.localeCompare(b),
  );
}

function isShortAnswer(value: string): boolean {
  const words = value
    .trim()
    .split(/\s+/u)
    .filter((word) => word !== "");
  return words.length <= 5 && !/[→?!]/u.test(value) && !value.includes("->");
}

function settle(
  values: readonly Value[],
  parse: (value: string) => string | null,
): State {
  if (values.length === 0) return "unknown";
  let agreed = "";
  let conflict = false;
  for (const { value, ambiguous } of values) {
    const normalized = parse(value);
    if (normalized === null || ambiguous) conflict = true;
    else if (agreed === "") agreed = normalized;
    else if (agreed !== normalized) conflict = true;
  }
  return conflict || agreed === "" ? "conflict" : "known";
}

function choiceValue(options: readonly Option[], value: string): string | null {
  const truth = trueFalseValue(value);
  const byTruth =
    truth && options.find((option) => trueFalseValue(option.text) === truth);
  if (byTruth) return byTruth.letter;
  if (isLetter(value)) {
    const ids = letters(value).map(
      (letter) => options.find((option) => option.letter === letter)?.letter,
    );
    return ids.includes(undefined) ? null : ids.join(",");
  }
  const want = value.trim().toLowerCase();
  return options.find((option) => option.text.toLowerCase() === want)?.letter ?? null;
}

function splitAt(text: string, cut: (at: number) => boolean): string[] {
  const parts: string[] = [];
  let start = 0;
  for (let at = 0; at < text.length; at++) {
    if (!cut(at)) continue;
    parts.push(text.slice(start, at).trim());
    start = at + 1;
  }
  parts.push(text.slice(start).trim());
  return parts;
}

const DASH = "-–";

function spacedDash(text: string, at: number): boolean {
  return DASH.includes(text[at]!) && (isSpace(text[at - 1]) || isSpace(text[at + 1]));
}

function splitInto(value: string, count: number): string[] | null {
  if (count === 1) return [value];
  const text = value.trim();
  const separators = [
    (at: number) => spacedDash(text, at),
    (at: number) => DASH.includes(text[at]!),
    (at: number) => ",;".includes(text[at]!),
    (at: number) => text[at] === "/",
  ];
  for (const cut of separators) {
    const parts = splitAt(text, cut);
    if (parts.length === count && !parts.includes("")) return parts;
  }
  return null;
}

function blanksState(values: readonly Value[], count: number): State {
  const perBlank: Value[][] = Array.from({ length: count }, () => []);
  for (const { value, ambiguous } of values) {
    const parts = splitInto(value, count);
    for (let i = 0; i < count; i++)
      perBlank[i]!.push(
        parts ? { value: parts[i]!, ambiguous } : { value, ambiguous: true },
      );
  }
  let state: State = "known";
  for (const blank of perBlank) {
    const one = settle(blank, (value) => value.trim() || null);
    if (one === "conflict") state = "conflict";
    else if (one === "unknown" && state === "known") state = "unknown";
  }
  return state;
}

const SYNTHETIC_TRUE_FALSE: readonly Option[] = [
  { letter: "T", text: "True", star: false },
  { letter: "F", text: "False", star: false },
];

function answerState(question: Question, tasks: number): State {
  const { options, keys } = question;
  const any = (test: (value: string) => boolean) => keys.some((key) => test(key.value));
  const choose = (from: readonly Option[]) =>
    settle(keys, (value) => choiceValue(from, value));
  if (options.length >= 1) return choose(options);
  if (any(isNotGiven))
    return settle(keys, (value) => (value === "" ? null : value.trim()));
  if (tasks & TASK_TRUE_FALSE || any(isTrueFalseWord))
    return choose(SYNTHETIC_TRUE_FALSE);
  if (any(isLetter)) return "unsupported";
  const gaps = question.stem.reduce((total, text) => total + gapCount(text), 0);
  const blanks =
    gaps > 0 &&
    !(tasks & TASK_NO_BLANKS) &&
    keys.every((key) => isShortAnswer(key.value));
  if (blanks) return blanksState(keys, gaps);
  return settle(keys, (value) => (value === "" ? null : value.trim()));
}

class Exam {
  readonly sections: Section[] = [];
  readonly keyEntries: KeyEntry[] = [];
  readonly keyLines: string[] = [];
  readonly labelLines: { number: number; owner: Owner }[] = [];
  paper = 0;
  private title = "";
  private section: Section | null = null;
  private question: Question | null = null;
  private group: Group | null = null;
  private lastNumber = 0;
  private lastRoman = 0;
  private explicit = false;
  private inKey = false;
  private extraPaper = false;

  read(line: string): void {
    if (this.extraPaper) return;
    if (this.inKey) {
      this.keyLines.push(line);
      return;
    }
    const handled =
      this.prefixedKeyLine(line) ||
      this.keySection(line) ||
      this.titleLine(line) ||
      this.anotherPaper(line) ||
      this.sectionLine(line) ||
      this.questionLine(line) ||
      this.optionLine(line) ||
      this.answerLine(line);
    if (!handled) this.textLine(line);
  }

  finish(): void {
    this.closeQuestion();
    this.closeGroup();
    if (!this.explicit) this.inferSections();
  }

  private prefixedKeyLine(line: string): boolean {
    const entries = prefixedKeyEntries(line, 0);
    if (entries.length === 0) return false;
    this.closeQuestion();
    this.keyEntries.push(...entries);
    return true;
  }

  private keySection(line: string): boolean {
    const text = line.trim();
    const answerPrefix = /^(?:đáp án|dap an|answer)/iu.test(text);
    if (!KEY_HEADING.test(text) && !(paperHeading(line) !== null && answerPrefix))
      return false;
    this.closeQuestion();
    this.closeGroup();
    this.inKey = true;
    this.keyLines.push(line);
    return true;
  }

  private titleLine(line: string): boolean {
    const text = line.trim();
    if (this.title !== "" || this.section || this.group || this.question) return false;
    if (isInstruction(line) || textLabel(line) || romanHeading(line, true))
      return false;
    if (namedSection(text) !== null || Array.from(text).length > MAX_TITLE)
      return false;
    this.title = text;
    this.paper = paperTitle(text) ?? 0;
    return true;
  }

  private anotherPaper(line: string): boolean {
    const heading = paperHeading(line);
    if (heading === null || this.paper === 0 || heading === this.paper) return false;
    this.closeQuestion();
    this.closeGroup();
    this.extraPaper = true;
    return true;
  }

  private sectionLine(line: string): boolean {
    const roman = romanHeading(line, true);
    if (roman) {
      const rest = line.slice(roman.end);
      const at = skipSpace(rest, 0);
      const option = optionAt(rest, at, at, "A");
      const heading = visible(rest) ? !option : true;
      if (heading && this.romanInSequence(roman.value, rest)) {
        this.openSection(line, roman.value);
        this.lastRoman = roman.value;
        return true;
      }
    }
    const ordinal = namedSection(line);
    if (ordinal === null) return false;
    this.openSection(line, ordinal);
    return true;
  }

  private romanInSequence(value: number, rest: string): boolean {
    if (value === this.lastRoman + 1) return true;
    return (
      this.lastRoman > 0 &&
      value >= this.lastRoman &&
      value <= this.lastRoman + MAX_JUMP &&
      isInstruction(rest)
    );
  }

  private openSection(line: string, ordinal: number): void {
    this.closeQuestion();
    this.closeGroup();
    const section: Section = { ordinal, tasks: tasksOf(line.trim()), items: [] };
    this.sections.push(section);
    this.section = section;
    this.explicit = true;
  }

  private ensureSection(): Section {
    if (this.section) return this.section;
    const section: Section = { ordinal: 0, tasks: 0, items: [] };
    this.sections.push(section);
    this.section = section;
    return section;
  }

  private questionLine(line: string): boolean {
    const label = textLabel(line);
    if (!label) return false;
    if (!this.acceptLabel(label) || this.gapReference(line, label)) {
      this.recordLabel(label, this.question ?? this.group ?? this.section);
      return false;
    }
    if (label.sub > 0) this.adoptParent(label);
    const question: Question = {
      label,
      instruction: "",
      stem: [],
      options: [],
      keys: [],
      section: null,
      group: null,
    };
    const end = this.inlineKey(line, question, label.end);
    const options = scanOptions(line.slice(0, end), label.end, "A");
    const rest = line.slice(label.end, end);
    if (options.length >= 2) {
      const stem = line.slice(label.end, options[0]!.at);
      if (visible(stem)) question.stem.push(stem);
      question.options = options.map((span) => toOption(line.slice(0, end), span));
    } else if (visible(rest) && isInstruction(rest)) question.instruction = rest.trim();
    else if (visible(rest)) question.stem.push(rest);
    this.attach(question);
    this.recordLabel(label, question);
    return true;
  }

  private recordLabel(label: Label, owner: Owner): void {
    if (label.sub === 0) this.labelLines.push({ number: label.number, owner });
  }

  private gapReference(line: string, label: Label): boolean {
    if (label.sub === 0) return false;
    const start = skipSpace(line, label.end);
    let end = start;
    while (end < line.length && "_….".includes(line[end]!)) end++;
    return end > start && gapCount(line.slice(start, end)) > 0;
  }

  private acceptLabel(label: Label): boolean {
    if (label.keyword !== "") return true;
    if (label.sub > 0) return this.hasParent(label);
    return (
      label.number === this.lastNumber + 1 ||
      label.number === 1 ||
      this.awaitingFirstQuestion() ||
      (this.lastNumber > 0 &&
        label.number > this.lastNumber &&
        label.number <= this.lastNumber + MAX_JUMP)
    );
  }

  private awaitingFirstQuestion(): boolean {
    if (!this.section || this.question) return false;
    return this.section.items.every(
      (item) => !item.question && (item.group?.questions.length ?? 0) === 0,
    );
  }

  private hasParent(label: Label): boolean {
    const parent = String(label.number);
    return this.group?.label === parent || this.question?.label.text === parent;
  }

  private adoptParent(label: Label): void {
    const parent = String(label.number);
    if (this.group?.label === parent) return;
    const question = this.question;
    if (!question || question.label.text !== parent || question.options.length > 0)
      return;
    const group: Group = {
      label: parent,
      instruction: question.instruction,
      stimulus: question.stem,
      questions: [],
      accepting: true,
    };
    const items = question.section?.items ?? [];
    const at = items.findIndex((item) => item.question === question);
    if (at >= 0) items[at] = { group };
    this.question = null;
    this.group = group;
  }

  private attach(question: Question): void {
    this.closeQuestion();
    const section = this.ensureSection();
    question.section = section;
    if (this.group && !this.admits(this.group, question)) this.closeGroup();
    if (this.group) {
      question.group = this.group;
      this.group.questions.push(question);
    } else section.items.push({ question });
    if (question.label.sub === 0) this.lastNumber = question.label.number;
    this.question = question;
  }

  private admits(group: Group, question: Question): boolean {
    if (!group.accepting) return false;
    if (group.label !== "")
      return question.label.sub > 0 && String(question.label.number) === group.label;
    return question.label.sub === 0 && question.instruction === "";
  }

  private inlineKey(line: string, question: Question, from: number): number {
    const text = line.slice(from);
    const match = INLINE_ANSWER.exec(text);
    const value = match?.[1]?.trim() ?? "";
    if (!match || value === "") return line.length;
    question.keys.push({ value, ambiguous: false });
    return from + match.index;
  }

  private optionLine(line: string): boolean {
    const question = this.question;
    if (!question) return false;
    const first =
      question.options.length === 0 ? "A" : nextLetter(question.options.at(-1)!.letter);
    const start = skipSpace(line, 0);
    if (start >= line.length || !optionAt(line, start, start, first)) return false;
    const end = this.inlineKey(line, question, start);
    const cut = line.slice(0, end);
    question.options.push(
      ...scanOptions(cut, start, first).map((span) => toOption(cut, span)),
    );
    return true;
  }

  private answerLine(line: string): boolean {
    if (!this.question || !ANSWER_LINE.test(line)) return false;
    this.inlineKey(line, this.question, 0);
    return true;
  }

  private textLine(line: string): void {
    const question = this.question;
    if (question && question.options.length === 0) {
      question.stem.push(line);
      return;
    }
    if (isInstruction(line) && !this.continuesStimulus(line)) {
      this.closeQuestion();
      this.closeGroup();
      this.openGroup({ instruction: line.trim(), stimulus: [] });
      return;
    }
    const group = this.group;
    if (group && group.accepting && group.questions.length === 0) {
      group.stimulus.push(line);
      return;
    }
    if (this.section && this.section.items.length === 0 && !question)
      this.openGroup({ instruction: "", stimulus: [line] });
  }

  private openGroup(start: { instruction: string; stimulus: string[] }): void {
    const group: Group = { label: "", ...start, questions: [], accepting: true };
    this.ensureSection().items.push({ group });
    this.group = group;
  }

  private continuesStimulus(text: string): boolean {
    const group = this.group;
    if (!group || !group.accepting || group.questions.length > 0) return false;
    const last = group.stimulus.at(-1);
    return last !== undefined && softWrapped(last.trim(), text.trim());
  }

  private closeQuestion(): void {
    const question = this.question;
    if (question) {
      const starred = question.options.filter((option) => option.star);
      if (starred.length > 0)
        question.keys.push({
          value: starred.map((option) => option.letter).join(","),
          ambiguous: false,
        });
    }
    this.question = null;
  }

  private closeGroup(): void {
    const group = this.group;
    if (!group) return;
    group.accepting = false;
    if (group.questions.length === 0 && this.section) {
      const items = this.section.items;
      const at = items.findIndex((item) => item.group === group);
      if (at >= 0) items.splice(at, 1);
    }
    this.group = null;
  }

  private inferSections(): void {
    if (this.sections.length !== 1) return;
    const inferred: Section[] = [];
    let key = "";
    for (const item of this.sections[0]!.items) {
      const instruction = item.group?.instruction ?? item.question?.instruction ?? "";
      const next = normalizedInstruction(instruction);
      if (inferred.length === 0 || next !== key || (item.group && next === "")) {
        inferred.push({ ordinal: 0, tasks: 0, items: [] });
        key = next;
      }
      const current = inferred.at(-1)!;
      current.items.push(item);
      if (item.question) item.question.section = current;
    }
    for (const section of inferred) {
      const first = section.items[0]!;
      const instruction = first.group?.instruction ?? first.question?.instruction ?? "";
      if (instruction === "") continue;
      section.tasks = tasksOf(instruction);
      for (const item of section.items) if (item.group) item.group.instruction = "";
    }
    this.sections.splice(0, 1, ...inferred);
  }
}

function allQuestions(sections: readonly Section[]): Question[] {
  return sections.flatMap((section) =>
    section.items.flatMap((item) =>
      item.question ? [item.question] : (item.group?.questions ?? []),
    ),
  );
}

function applyKeys(exam: Exam, questions: readonly Question[]): void {
  if (exam.keyLines.length === 0 && exam.keyEntries.length === 0) return;
  const book = new KeyBook(exam.paper);
  for (const line of exam.keyLines) book.read(line);
  book.paper(exam.paper).push(...exam.keyEntries);
  const byLabel = new Map<string, Question[]>();
  for (const question of questions)
    byLabel.set(question.label.text, [
      ...(byLabel.get(question.label.text) ?? []),
      question,
    ]);
  for (const entry of book.choose(exam.paper)) {
    let targets = byLabel.get(entry.label.text) ?? [];
    if (targets.length > 1 && entry.section > 0) {
      const inSection = targets.filter(
        (target) => target.section?.ordinal === entry.section,
      );
      if (inSection.length > 0) targets = inSection;
    }
    for (const target of targets)
      target.keys.push({ value: entry.value, ambiguous: targets.length > 1 });
  }
}

function trueFalseSections(questions: readonly Question[]): Set<Section | null> {
  const sawTrue = new Set<Section | null>();
  const other = new Set<Section | null>();
  for (const question of questions) {
    if (question.options.length > 0) continue;
    for (const key of question.keys) {
      const value = trueFalseValue(key.value);
      if (value === null) other.add(question.section);
      else if (value === "T") sawTrue.add(question.section);
    }
  }
  return new Set([...sawTrue].filter((section) => !other.has(section)));
}

function questionTasks(question: Question, trueFalse: Set<Section | null>): number {
  let tasks =
    tasksOf(question.instruction) | tasksOf(question.group?.instruction ?? "");
  tasks |= question.section?.tasks ?? 0;
  if (trueFalse.has(question.section) && question.options.length === 0)
    tasks |= TASK_TRUE_FALSE;
  return tasks;
}

function sectionsOf(sections: readonly Section[]): Map<Owner, Section> {
  const home = new Map<Owner, Section>();
  for (const section of sections) {
    home.set(section, section);
    for (const item of section.items) {
      const owners = [item.question, item.group, ...(item.group?.questions ?? [])];
      for (const owner of owners) if (owner) home.set(owner, section);
    }
  }
  return home;
}

function duplicateNumbers(exam: Exam): number[] {
  const home = sectionsOf(exam.sections);
  const seen = new Map<Section, Set<number>>();
  const duplicates: number[] = [];
  for (const { number, owner } of exam.labelLines) {
    const section = home.get(owner);
    if (!section) continue;
    const numbers = seen.get(section) ?? new Set<number>();
    seen.set(section, numbers);
    if (numbers.has(number) && !duplicates.includes(number)) duplicates.push(number);
    numbers.add(number);
  }
  return duplicates;
}

/**
 * scanPastedText counts a pasted test by the server's grammar for text
 * sources (T-R4.56) and its answer rules, so "Found so far" agrees with the
 * review the server builds; `api/testdata/pasted-text-counts.json` pins the
 * two together, and the server's review stays the authority (DG-13). Lines
 * are read in the server's order: a one-line key, a key section, the title,
 * another paper, a part heading, a question label the sequence admits, the
 * open question's next option, its answer line, then text, which can open an
 * instruction or a passage. Without a heading, parts are inferred where the
 * instruction changes. A question's answer counts only when every value it
 * has resolves to the same answer; a conflict counts as neither answered nor
 * missing. Not ported: numbered blanks inside a passage and companion files.
 */
export function scanPastedText(text: string): PasteScan {
  const exam = new Exam();
  for (const line of lines(text)) exam.read(line);
  exam.finish();
  const questions = allQuestions(exam.sections);
  applyKeys(exam, questions);
  const trueFalse = trueFalseSections(questions);
  let answered = 0;
  const missing: string[] = [];
  for (const question of questions) {
    const state = answerState(question, questionTasks(question, trueFalse));
    if (state === "known") answered += 1;
    if (state === "unknown") missing.push(question.label.text);
  }
  return {
    sections: exam.sections.length,
    questions: questions.length,
    answered,
    missing,
    duplicates: duplicateNumbers(exam),
  };
}

/**
 * pastedTitle is the first non-blank line of a pasted test when it reads as
 * a title: at most 120 characters, and not a part heading, a question label,
 * an option or an instruction. It is "" otherwise.
 */
export function pastedTitle(text: string): string {
  const first = lines(text)[0]?.trim() ?? "";
  if (first === "" || pasteLength(first) > TITLE_MAX) return "";
  if (namedSection(first) !== null || textLabel(first) !== null) return "";
  if (romanHeading(first, true) !== null || isInstruction(first)) return "";
  if (optionAt(first, 0, 0, "A")) return "";
  return first;
}
