import {
  AlignLeft,
  CircleCheck,
  CircleDot,
  SquareCheck,
  TextCursorInput,
  type LucideIcon,
} from "lucide-react";
import type { QuestionType } from "@/features/question-bank/questionSchema";

/**
 * QUESTION_TYPE_ICONS is the deck's icon for each question type, shared by the
 * builder's type menu and its outline rows.
 */
export const QUESTION_TYPE_ICONS: Readonly<Record<QuestionType, LucideIcon>> = {
  single_choice: CircleDot,
  multiple_choice: SquareCheck,
  true_false: CircleCheck,
  fill_blank: TextCursorInput,
  short_answer: AlignLeft,
};
