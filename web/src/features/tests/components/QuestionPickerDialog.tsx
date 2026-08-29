import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { listQuestions } from "@/features/question-bank/api";

interface QuestionPickerDialogProps {
  open: boolean;
  /** Already in the outline: offering them again invites a duplicate. */
  excluded: ReadonlySet<string>;
  onOpenChange: (open: boolean) => void;
  onPick: (questionId: string) => void;
}

/** A-04's "Lấy từ ngân hàng": the second test is faster only if the first is reusable. */
export function QuestionPickerDialog({
  open,
  excluded,
  onOpenChange,
  onPick,
}: QuestionPickerDialogProps) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("builder.fromBank")}</DialogTitle>
        </DialogHeader>
        <Input
          value={query}
          placeholder={t("builder.searchBank")}
          aria-label={t("builder.searchBank")}
          onChange={(event) => setQuery(event.target.value)}
        />
        {open ? (
          <BankList
            query={query}
            excluded={excluded}
            onPick={(id) => {
              onPick(id);
              onOpenChange(false);
            }}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  );
}

function BankList({
  query,
  excluded,
  onPick,
}: {
  query: string;
  excluded: ReadonlySet<string>;
  onPick: (questionId: string) => void;
}) {
  const { t } = useTranslation();
  const bank = useQuery({
    queryKey: ["admin-questions", query],
    queryFn: ({ signal }) =>
      listQuestions(
        query.trim() === "" ? { limit: 50 } : { q: query.trim(), limit: 50 },
        signal,
      ),
  });

  if (bank.isPending) {
    return (
      <p className="text-muted-foreground text-sm" role="status" aria-live="polite">
        {t("common.loading")}
      </p>
    );
  }
  if (bank.isError) {
    return (
      <p role="alert" className="text-destructive text-sm">
        {t("builder.bankFailed")}
      </p>
    );
  }

  const fetched = bank.data.items;
  const items = fetched.filter((question) => !excluded.has(question.id));
  if (items.length === 0) {
    // Three different answers, and the teacher's next move differs for each:
    // write a question, clear the search, or stop looking.
    const message =
      fetched.length === 0
        ? query.trim() === ""
          ? "builder.bankEmpty"
          : "builder.bankNoMatches"
        : "builder.bankAllAdded";
    return <p className="text-muted-foreground text-sm">{t(message)}</p>;
  }

  return (
    <ul className="max-h-80 space-y-1 overflow-y-auto">
      {items.map((question) => (
        <li key={question.id}>
          <button
            type="button"
            onClick={() => onPick(question.id)}
            className="hover:bg-accent focus-visible:ring-ring flex w-full items-center justify-between gap-4 rounded-md px-3 py-2 text-left text-sm focus-visible:ring-2 focus-visible:outline-none"
          >
            <span className="truncate">{question.prompt}</span>
            <span className="flex shrink-0 items-center gap-2">
              <Badge>{t(`questionEditor.type.${question.type}`)}</Badge>
              <span className="text-muted-foreground tabular-nums">
                {t("builder.points", { points: question.points })}
              </span>
            </span>
          </button>
        </li>
      ))}
    </ul>
  );
}
