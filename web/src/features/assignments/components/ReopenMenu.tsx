import { useTranslation } from "react-i18next";
import { Calendar, CalendarClock, ChevronDown, Clock } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

export type ReopenChoice = "today" | "day" | "threeDays" | "pick";

/**
 * G-09's "Gia hạn cho tất cả": a menu of moments first, so the common case is
 * two clicks; the dialog that follows only asks for the reason.
 */
export function ReopenMenu({
  count,
  todayPossible,
  onChoose,
}: Readonly<{
  count: number;
  todayPossible: boolean;
  onChoose: (choice: ReopenChoice) => void;
}>) {
  const { t } = useTranslation();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm">
          <CalendarClock aria-hidden="true" />
          {t("assignments.detail.reopen")}
          <ChevronDown aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-60">
        <p className="text-muted-foreground px-2 py-1 text-xs">
          {t("assignments.detail.reopenTitle", { count })}
        </p>
        <DropdownMenuItem disabled={!todayPossible} onSelect={() => onChoose("today")}>
          <Clock className="text-muted-foreground" aria-hidden="true" />
          {t("assignments.detail.reopenUntilToday")}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => onChoose("day")}>
          <Calendar className="text-muted-foreground" aria-hidden="true" />
          {t("assignments.detail.reopenPlusDay")}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => onChoose("threeDays")}>
          <Calendar className="text-muted-foreground" aria-hidden="true" />
          {t("assignments.detail.reopenPlusThreeDays")}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={() => onChoose("pick")}>
          <CalendarClock className="text-muted-foreground" aria-hidden="true" />
          {t("assignments.detail.reopenPick")}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <p className="text-muted-foreground px-2 py-1 text-xs leading-relaxed">
          {t("assignments.detail.reopenMenuHint")}
        </p>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
