import type { components } from "@/lib/api/schema";
import { isOptionContent } from "@/components/shared/content/optionContent";
import { contentPlainText } from "@/components/shared/content/plainText";
import {
  isQuestionContent,
  isQuestionPromptContent,
  type QuestionContent,
} from "@/components/shared/content/questionContent";
import type { ImportDraftGroup, ImportDraftQuestion, ImportDraftSection } from "./api";

type StudentQuestion = components["schemas"]["StudentQuestion"];
type StudentSection = components["schemas"]["StudentSection"];
type StudentGroup = components["schemas"]["StudentGroup"];
type GroupGapBinding = components["schemas"]["GroupGapBinding"];
type QuestionType = components["schemas"]["QuestionType"];
type ImportGapLink = ImportDraftGroup["gaps"][number];

/**
 * DraftPreview is an import draft in the shape the student's paper takes:
 * the questions it can draw, their sections and shared passages, and
 * `included`, the questions still in the test, which can exceed
 * `questions.length` while a question has a type the paper cannot draw.
 */
export interface DraftPreview {
  questions: StudentQuestion[];
  sections: StudentSection[];
  groups: StudentGroup[];
  included: number;
}

const DRAWN: ReadonlySet<string> = new Set<QuestionType>([
  "single_choice",
  "multiple_choice",
  "true_false",
  "fill_blank",
  "short_answer",
]);

function isDrawn(
  question: ImportDraftQuestion,
): question is ImportDraftQuestion & { type: QuestionType } {
  return question.excluded === undefined && DRAWN.has(question.type);
}

function asStudent(
  sectionId: string,
  question: ImportDraftQuestion & { type: QuestionType },
): StudentQuestion {
  return {
    id: question.id,
    sectionId,
    type: question.type,
    prompt: contentPlainText(question.prompt),
    promptContent: isQuestionPromptContent(question.prompt) ? question.prompt : null,
    points: Number(question.points) || 0,
    options: question.options.map((option) => ({
      id: option.id,
      text: contentPlainText(option.content),
      content: isOptionContent(option.content) ? option.content : null,
    })),
    blanks: question.blanks.map((blank, index) => ({
      id: `${question.id}-blank-${index + 1}`,
      ordinal: index + 1,
      gapId: blank.gapId,
      caseSensitive: blank.caseSensitive,
    })),
  };
}

function paragraphs(text: string | undefined): QuestionContent | null {
  const lines = (text ?? "").split("\n").filter((line) => line.trim() !== "");
  if (lines.length === 0) return null;
  const content = {
    format: "semantic_v1",
    blocks: lines.map((line) => ({
      type: "paragraph",
      content: [{ type: "text", text: line, marks: [] }],
    })),
  };
  return isQuestionContent(content) ? content : null;
}

function binding(link: ImportGapLink): GroupGapBinding {
  return link.blankGapId === undefined
    ? { kind: "question", gapId: link.gapId, questionId: link.questionId }
    : {
        kind: "blank",
        gapId: link.gapId,
        questionId: link.questionId,
        blankGapId: link.blankGapId,
      };
}

function asGroup(
  sectionId: string,
  group: ImportDraftGroup,
  questionIds: string[],
): StudentGroup {
  return {
    id: group.id,
    sectionId,
    title: group.label ?? "",
    instructions: paragraphs(group.instructions),
    questionIds,
    stimuli:
      group.stimulus === undefined
        ? []
        : [
            {
              id: `${group.id}-passage`,
              title: "",
              content: group.stimulus,
              gaps: group.gaps.map(binding),
            },
          ],
    recordings: [],
    assets: [],
  };
}

/**
 * draftPreview maps an import draft to the student's paper, in order, and
 * applies §13.5's boundary itself: nothing that grades (the answer, accepted
 * values, candidates) and nothing only the teacher sees (sources, origins,
 * exclusion reasons) reaches the result. Excluded questions are left out.
 */
export function draftPreview(sections: readonly ImportDraftSection[]): DraftPreview {
  const out: DraftPreview = { questions: [], sections: [], groups: [], included: 0 };
  for (const section of sections) {
    out.sections.push({
      id: section.id,
      title: section.title,
      instructions: section.instructions ?? null,
    });
    for (const item of section.items) {
      const members = item.question ? [item.question] : (item.group?.questions ?? []);
      out.included += members.filter((member) => member.excluded === undefined).length;
      const drawn = members
        .filter(isDrawn)
        .map((member) => asStudent(section.id, member));
      out.questions.push(...drawn);
      if (item.group && drawn.length > 0)
        out.groups.push(
          asGroup(
            section.id,
            item.group,
            drawn.map((question) => question.id),
          ),
        );
    }
  }
  return out;
}
