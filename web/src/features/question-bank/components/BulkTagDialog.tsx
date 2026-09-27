import { fold } from "@/lib/fold";
import { useAuthStore } from "@/stores/auth";
import { useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { X } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { tagQuestions } from "@/features/question-bank/api";
import { ApiError } from "@/lib/api/errors";
import { toast } from "@/components/ui/sonner";

/**
 * A-06's "Gắn thẻ" for a selection.
 *
 * Additive only. Removing tags from forty questions at once is the destructive
 * direction and the board does not draw it, so it is not offered here either —
 * a bulk action that can silently strip work is one nobody can undo.
 */
export function BulkTagDialog({
  questionIds,
  suggestions,
  open,
  onOpenChange,
  onApplied,
}: Readonly<{
  questionIds: string[];
  suggestions: string[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onApplied: () => void;
}>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const suggestionId = useId();
  const userId = useAuthStore((state) => state.user?.id ?? "anonymous");
  const recentKey = ["recent-question-tags", userId];
  const [recent, setRecent] = useState(
    () => queryClient.getQueryData<string[]>(recentKey) ?? [],
  );
  const known = [...new Set([...recent, ...suggestions])];
  const [tags, setTags] = useState<string[]>([]);
  const [draft, setDraft] = useState("");
  const [error, setError] = useState<string | null>(null);

  function close() {
    setTags([]);
    setDraft("");
    setError(null);
    onOpenChange(false);
  }

  function add(value: string) {
    const tag = known.find((item) => fold(item) === fold(value.trim())) ?? value.trim();
    if (tag === "" || tags.includes(tag)) return;
    setTags([...tags, tag]);
    const next = [tag, ...recent.filter((item) => item !== tag)].slice(0, 6);
    setRecent(next);
    queryClient.setQueryDefaults(recentKey, { gcTime: Infinity });
    queryClient.setQueryData(recentKey, next);
    setDraft("");
  }

  const pending =
    known.find((item) => fold(item) === fold(draft.trim())) ?? draft.trim();
  const effective =
    pending === "" || tags.includes(pending) ? tags : [...tags, pending];

  const apply = useMutation({
    mutationFn: () => tagQuestions(questionIds, effective),
    onSuccess: async () => {
      const next = [...new Set([...effective, ...recent])].slice(0, 6);
      setRecent(next);
      queryClient.setQueryDefaults(recentKey, { gcTime: Infinity });
      queryClient.setQueryData(recentKey, next);
      await queryClient.invalidateQueries({ queryKey: ["admin-questions"] });
      onApplied();
      close();
      toast(t("bank.tagged"));
    },
    onError: (cause) =>
      setError(cause instanceof ApiError ? cause.message : t("bank.tagFailed")),
  });

  const unused = known
    .filter((tag) => !tags.includes(tag) && fold(tag).includes(fold(draft.trim())))
    .slice(0, 6);

  return (
    <Dialog open={open} onOpenChange={(next) => (next ? onOpenChange(true) : close())}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("bank.bulkTag")}</DialogTitle>
          <DialogDescription>
            {t("bank.bulkTagHint", { count: questionIds.length })}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3">
          <div>
            <Label htmlFor="bulk-tag">{t("bank.tagFilter")}</Label>
            <div className="focus-within:ring-ring mt-1.5 flex min-h-9 flex-wrap items-center gap-1.5 rounded-md border p-2 focus-within:ring-2">
              {tags.map((tag) => (
                <Badge key={tag} variant="secondary" className="gap-1 pr-1">
                  {tag}
                  <button
                    type="button"
                    aria-label={t("bank.removeTag", { tag })}
                    onClick={() => setTags(tags.filter((x) => x !== tag))}
                  >
                    <X className="size-3" aria-hidden="true" />
                  </button>
                </Badge>
              ))}
              <Input
                list={suggestionId}
                id="bulk-tag"
                className="h-6 min-w-32 flex-1 border-0 p-0 shadow-none"
                placeholder={t("bank.tagPlaceholder")}
                value={draft}
                onChange={(event) => setDraft(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    event.preventDefault();
                    add(draft);
                  }
                }}
              />
            </div>
          </div>

          <datalist id={suggestionId}>
            {unused.map((tag) => (
              <option key={tag} value={tag} />
            ))}
          </datalist>
          {unused.length === 0 ? null : (
            <div className="flex flex-wrap gap-1.5">
              {unused.map((tag) => (
                <button key={tag} type="button" onClick={() => add(tag)}>
                  <Badge variant="outline">{tag}</Badge>
                </button>
              ))}
            </div>
          )}

          {error === null ? null : (
            <p role="alert" className="text-destructive text-sm">
              {error}
            </p>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={close}>
            {t("common.cancel")}
          </Button>
          <Button
            disabled={effective.length === 0 || apply.isPending}
            onClick={() => apply.mutate()}
          >
            {apply.isPending ? t("common.loading") : t("bank.applyTags")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
