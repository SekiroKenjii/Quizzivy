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

interface Question {
  number: number;
  options: string[];
  values: string[];
  starred: string[];
}

interface KeyEntry {
  number: number;
  value: string;
}

interface Label {
  number: number;
  keyword: boolean;
  end: number;
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

const MAX_JUMP = 20;
const TITLE_MAX = 120;

const KEYWORD_LABEL = /^\s*(?:question|câu|cau|q)\s*(\d{1,3})\s*[.:)]?/iu;
const BARE_LABEL = /^\s*(\d{1,3})\s*[.)]/u;
const COLON_LABEL = /^\s*(\d{1,3})\s*:/u;
const SECTION_WORD =
  /^\s*(?:part|section|phần|phan|bài tập|bai tap|bài|bai|exercise|task)\s+/iu;
const SECTION_MARK = /^(?:\d{1,2}|[ivx]{1,5}|[a-h])(?=[\s.:)\-–]|$)/iu;
const BARE_ROMAN = /^\s*([IVXL]{1,6})\s*$/u;
const ANSWER_LINE = /^\s*(?:answer\s*[:-]|(?:ans|đáp\s*án|key)\s*:)\s*\S/iu;
const INLINE_ANSWER = /(?:^|\s)(?:answer\s*[:-]|(?:ans|đáp\s*án|key)\s*:)(.*)$/iu;
const KEY_PREFIX = /^\s*(?:answer\s*key|answers|key|đáp\s*án)\s*:\s*/iu;
const KEY_PAIR = /(?:^|\s)(\d{1,3})\s*[-:.)]\s*/gu;
const KEY_HEADING = /^(?:đáp\s*án|dap\s*an|answer\s*keys?|answers|keys?)\s*[:.]?$/iu;
const KEY_ENTRY = /^(\d{1,3})(.+)$/u;
const KEY_ENTRY_MARKS = /^[.:)\s]+/u;

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

function isNamedSection(line: string): boolean {
  const word = SECTION_WORD.exec(line);
  return word !== null && SECTION_MARK.test(line.slice(word[0].length));
}

function questionLabel(line: string): Label | null {
  const keyword = KEYWORD_LABEL.exec(line);
  if (keyword)
    return { number: Number(keyword[1]), keyword: true, end: keyword[0].length };
  const bare = BARE_LABEL.exec(line);
  if (bare && !/\d/u.test(line[bare[0].length] ?? ""))
    return { number: Number(bare[1]), keyword: false, end: bare[0].length };
  const colon = COLON_LABEL.exec(line);
  if (colon) return { number: Number(colon[1]), keyword: false, end: colon[0].length };
  return null;
}

function optionStart(line: string, letter: string): string | null {
  const at = line.length - line.trimStart().length;
  const head = line[at]?.toUpperCase();
  const mark = line[at + 1];
  if (head !== letter || (mark !== "." && mark !== ")")) return null;
  if (/\d/u.test(line[at + 2] ?? "")) return null;
  return line.slice(at + 2).trim();
}

function inlineAnswer(text: string): string | null {
  const value = INLINE_ANSWER.exec(text)?.[1]?.trim() ?? "";
  return value === "" ? null : value;
}

function keyEntries(line: string): KeyEntry[] {
  const prefix = KEY_PREFIX.exec(line);
  if (!prefix) return [];
  const value = line.slice(prefix[0].length);
  const pairs = [...value.matchAll(KEY_PAIR)];
  if (pairs.length < 2 || pairs[0]!.index !== 0) return [];
  const entries: KeyEntry[] = [];
  for (const [i, pair] of pairs.entries()) {
    const end = pairs[i + 1]?.index ?? value.length;
    const answer = value.slice(pair.index + pair[0].length, end).trim();
    if (answer === "") return [];
    entries.push({ number: Number(pair[1]), value: answer });
  }
  return entries;
}

class Scanner {
  private readonly questions: Question[] = [];
  private readonly keys: KeyEntry[] = [];
  private readonly duplicates = new Set<number>();
  private sections = 0;
  private part: Set<number> | null = null;
  private question: Question | null = null;
  private lastNumber = 0;
  private lastRoman = 0;
  private inKey = false;

  read(line: string): void {
    if (this.inKey) {
      this.readKeyEntry(line.trim());
      return;
    }
    if (this.readKeyLine(line) || this.readPartHeading(line) || this.readLabel(line))
      return;
    this.readQuestionLine(line);
  }

  result(): PasteScan {
    this.close();
    const byNumber = new Map<number, Question[]>();
    for (const question of this.questions)
      byNumber.set(question.number, [
        ...(byNumber.get(question.number) ?? []),
        question,
      ]);
    for (const key of this.keys)
      for (const target of byNumber.get(key.number) ?? [])
        target.values.push(key.value);
    const missing = this.questions
      .filter((item) => item.values.length === 0)
      .map((item) => String(item.number));
    return {
      sections: this.sections,
      questions: this.questions.length,
      answered: this.questions.length - missing.length,
      missing,
      duplicates: [...this.duplicates],
    };
  }

  private readKeyEntry(line: string): void {
    if (KEY_HEADING.test(line)) return;
    const entry = KEY_ENTRY.exec(line);
    if (entry)
      this.keys.push({
        number: Number(entry[1]),
        value: entry[2]!.replace(KEY_ENTRY_MARKS, ""),
      });
  }

  private readKeyLine(line: string): boolean {
    const entries = keyEntries(line);
    if (entries.length > 0) {
      this.close();
      this.keys.push(...entries);
      return true;
    }
    if (!KEY_HEADING.test(line.trim())) return false;
    this.close();
    this.inKey = true;
    return true;
  }

  private readPartHeading(line: string): boolean {
    const roman = BARE_ROMAN.exec(line);
    const romanNumber = roman ? romanValue(roman[1]!) : null;
    if (romanNumber !== null && romanNumber === this.lastRoman + 1) {
      this.lastRoman = romanNumber;
      this.openPart();
      return true;
    }
    if (!isNamedSection(line)) return false;
    this.openPart();
    return true;
  }

  private readLabel(line: string): boolean {
    const label = questionLabel(line);
    if (!label) return false;
    if (this.part?.has(label.number)) this.duplicates.add(label.number);
    if (!this.admits(label)) return false;
    this.close();
    const part = this.part ?? this.openPart();
    part.add(label.number);
    this.lastNumber = label.number;
    const question: Question = {
      number: label.number,
      options: [],
      values: [],
      starred: [],
    };
    const inline = inlineAnswer(line.slice(label.end));
    if (inline !== null) question.values.push(inline);
    this.questions.push(question);
    this.question = question;
    return true;
  }

  private admits(label: Label): boolean {
    if (label.keyword || label.number === 1) return true;
    return (
      this.lastNumber > 0 &&
      label.number > this.lastNumber &&
      label.number <= this.lastNumber + MAX_JUMP
    );
  }

  private readQuestionLine(line: string): void {
    const question = this.question;
    if (!question) return;
    const letter = String.fromCharCode(65 + question.options.length);
    const content = optionStart(line, letter);
    if (content !== null) {
      question.options.push(letter);
      if (content.startsWith("*") || content.endsWith("*"))
        question.starred.push(letter);
      return;
    }
    if (!ANSWER_LINE.test(line)) return;
    const value = inlineAnswer(line);
    if (value !== null) question.values.push(value);
  }

  private openPart(): Set<number> {
    this.close();
    this.sections += 1;
    const part = new Set<number>();
    this.part = part;
    return part;
  }

  private close(): void {
    if (this.question && this.question.starred.length > 0)
      this.question.values.push(this.question.starred.join(","));
    this.question = null;
  }
}

/**
 * scanPastedText counts a pasted test by the server's grammar for text
 * sources (T-R4.56), as far as `api/testdata/pasted-text-counts.json`
 * exercises it; the server's review is the authority (DG-13). A line is, in
 * this order: a one-line key with two or more pairs; a key heading, after
 * which every line is a key entry; a part heading; a question label the
 * sequence admits; the question's next option; its answer line; or text.
 */
export function scanPastedText(text: string): PasteScan {
  const scanner = new Scanner();
  for (const line of lines(text)) scanner.read(line);
  return scanner.result();
}

/**
 * pastedTitle is the first non-blank line of a pasted test when it reads as
 * a title: at most 120 characters, and not a part heading, a question label
 * or an option. It is "" otherwise.
 */
export function pastedTitle(text: string): string {
  const first = lines(text)[0]?.trim() ?? "";
  if (first === "" || pasteLength(first) > TITLE_MAX) return "";
  if (isNamedSection(first) || questionLabel(first) !== null) return "";
  if (optionStart(first, "A") !== null) return "";
  return first;
}
