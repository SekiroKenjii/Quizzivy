import type { TFunction } from "i18next";
import type { DataColumn } from "@/components/shared/data/DataTable";
import type { AdminQuestion } from "@/features/question-bank/api";
import {
  LevelCell,
  QuestionCell,
  TagChips,
  TypeCell,
  UpdatedCell,
  UsedCell,
} from "./BankCells";

/**
 * BANK_ASIDE_WIDTH is what the filter aside and its gap take from the content
 * width from 1024px: the deck's 220px column and 14px gap.
 */
export const BANK_ASIDE_WIDTH = 234;

const GAP = 12;
const FRAME = 2 + 2 * 16;
const BASE = FRAME + 16 + GAP + 220;

const COLUMNS = [
  { id: "level", deckFrom: 540, track: 70 },
  { id: "type", deckFrom: 640, track: 140 },
  { id: "used", deckFrom: 780, track: 80 },
  { id: "updated", deckFrom: 900, track: 90 },
] as const;

/**
 * bankThresholds gives each column the content width it shows from: the
 * deck's threshold, or more where the filter aside's `aside` pixels would
 * leave the table narrower than its tracks need, the columns that show
 * earlier included. The deck measures its thresholds on the content width
 * and so overflows the table by up to 62px beside the aside; this drops the
 * column instead.
 */
export function bankThresholds(aside: number): Record<string, number> {
  let need = BASE;
  const out: Record<string, number> = {};
  for (const column of COLUMNS) {
    need += GAP + column.track;
    out[column.id] = Math.max(column.deckFrom, need + aside);
  }
  return out;
}

/**
 * bankColumns is the bank's table as the deck draws it: Question with its
 * tags, then Level, Type, Used and Updated as the content width allows
 * (bankThresholds). Below the Type column's threshold the Question cell
 * carries the type and level on a second line.
 */
export function bankColumns(t: TFunction, aside: number): DataColumn<AdminQuestion>[] {
  const from = bankThresholds(aside);
  return [
    {
      id: "question",
      header: t("bank.prompt"),
      track: "minmax(220px,3fr)",
      cell: (question, shown) => (
        <QuestionCell question={question} inline={!shown.has("type")} />
      ),
      aside: (question) => <TagChips tags={question.tags} />,
    },
    {
      id: "type",
      header: t("bank.type"),
      track: "140px",
      showFrom: from["type"]!,
      cell: (question) => <TypeCell question={question} />,
    },
    {
      id: "level",
      header: t("bank.levelFilter"),
      track: "70px",
      showFrom: from["level"]!,
      cell: (question) => <LevelCell question={question} />,
    },
    {
      id: "used",
      header: t("bank.used"),
      track: "80px",
      showFrom: from["used"]!,
      cell: (question) => <UsedCell question={question} />,
    },
    {
      id: "updated",
      header: t("common.updated"),
      track: "90px",
      showFrom: from["updated"]!,
      cell: (question) => <UpdatedCell question={question} />,
    },
  ];
}
