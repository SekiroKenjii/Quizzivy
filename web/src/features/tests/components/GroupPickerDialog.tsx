import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Field, FieldLabel } from "@/components/ui/field";
import { ApiError } from "@/lib/api/errors";
import { ListSkeleton, LoadError } from "@/components/shared/ListState";
import { listGroups, type GroupSummary } from "@/features/question-groups/api";
import type { OutlineSection } from "../outline";

export function GroupPickerDialog({
  open,
  sections,
  onOpenChange,
  onPick,
}: Readonly<{
  open: boolean;
  sections: OutlineSection[];
  onOpenChange: (open: boolean) => void;
  onPick: (group: GroupSummary, sectionIndex: number) => Promise<void>;
}>) {
  const { t } = useTranslation();
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("builder.chooseWholeGroup")}</DialogTitle>
          <DialogDescription>{t("builder.groupCopyHint")}</DialogDescription>
        </DialogHeader>
        {open ? <GroupChoices sections={sections} onPick={onPick} /> : null}
      </DialogContent>
    </Dialog>
  );
}

function GroupChoices({
  sections,
  onPick,
}: Readonly<{
  sections: OutlineSection[];
  onPick: (group: GroupSummary, sectionIndex: number) => Promise<void>;
}>) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(1);
  const [destination, setDestination] = useState("0");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const list = useQuery({
    queryKey: ["admin-groups", "picker", query, page],
    queryFn: ({ signal }) =>
      listGroups({ q: query, page, limit: 20, status: "active" }, signal),
  });
  async function choose(group: GroupSummary) {
    setBusy(true);
    setError(null);
    try {
      await onPick(group, Number(destination));
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : t("groups.saveFailed"));
      void list.refetch();
    } finally {
      setBusy(false);
    }
  }
  function choices() {
    if (list.isPending) return <ListSkeleton rows={3} />;
    if (list.isError)
      return (
        <LoadError error={list.error} onRetry={() => void list.refetch()}>
          {t("groups.loadFailed")}
        </LoadError>
      );
    return (
      <>
        <div className="flex max-h-72 flex-col gap-2 overflow-y-auto">
          {list.data.items.length === 0 ? (
            <p className="text-muted-foreground text-sm">
              {t("builder.bankNoMatches")}
            </p>
          ) : (
            list.data.items.map((group) => (
              <Button
                key={group.id}
                variant="outline"
                className="h-auto justify-between gap-4 py-3"
                disabled={busy || !sections[Number(destination)]}
                onClick={() => void choose(group)}
              >
                <span className="truncate">{group.title}</span>
                <span className="text-muted-foreground shrink-0">
                  {t("builder.outlineQuestionsOnly", {
                    questions: group.questionCount,
                  })}
                </span>
              </Button>
            ))
          )}
        </div>
        <div className="flex items-center justify-between gap-3">
          <Button
            variant="ghost"
            disabled={page === 1 || busy}
            onClick={() => setPage(page - 1)}
          >
            {t("pagination.previous")}
          </Button>
          <span className="text-muted-foreground text-sm">{page}</span>
          <Button
            variant="ghost"
            disabled={page * 20 >= list.data.total || busy}
            onClick={() => setPage(page + 1)}
          >
            {t("pagination.next")}
          </Button>
        </div>
      </>
    );
  }
  return (
    <fieldset disabled={busy} className="flex min-w-0 flex-col gap-4">
      <Field>
        <FieldLabel htmlFor="group-destination">
          {t("builder.destinationSection")}
        </FieldLabel>
        <Select value={destination} onValueChange={setDestination}>
          <SelectTrigger id="group-destination">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              {sections.map((section, index) => (
                <SelectItem
                  key={section.clientId ?? section.id ?? index}
                  value={String(index)}
                >
                  {section.title}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
      </Field>
      <Input
        aria-label={t("builder.searchBank")}
        placeholder={t("builder.searchBank")}
        value={query}
        onChange={(event) => {
          setQuery(event.target.value);
          setPage(1);
        }}
      />
      {error ? (
        <p role="alert" className="text-sm">
          {error}
        </p>
      ) : null}
      {choices()}
    </fieldset>
  );
}
