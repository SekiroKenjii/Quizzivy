import { useId, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { listQuestions } from "@/features/question-bank/api";
import { fold } from "@/lib/fold";
import { useAuthStore } from "@/stores/auth";
import { useTranslation } from "react-i18next";
import { X } from "lucide-react";
import { Badge } from "@/components/ui/badge";

interface TagsFieldProps {
  tags: string[];
  onChange: (tags: string[]) => void;
}

/** The deck's A-04 tag field: chips in a bordered box with an inline input. */
export function TagsField({ tags, onChange }: Readonly<TagsFieldProps>) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState("");
  const id = useId();
  const client = useQueryClient();
  const userId = useAuthStore((state) => state.user?.id ?? "anonymous");
  const recentKey = ["recent-question-tags", userId];
  const [recent, setRecent] = useState(
    () => client.getQueryData<string[]>(recentKey) ?? [],
  );
  const available = useQuery({
    queryKey: ["question-tag-suggestions"],
    queryFn: ({ signal }) => listQuestions({ limit: 1 }, signal),
    staleTime: 60_000,
  });
  const known = [...new Set([...recent, ...(available.data?.tags ?? [])])];
  const matches = known
    .filter((tag) => !tags.includes(tag) && fold(tag).includes(fold(draft.trim())))
    .slice(0, 6);

  function add(value: string) {
    const entered = value.trim();
    const tag = known.find((item) => fold(item) === fold(entered)) ?? entered;
    if (tag !== "" && !tags.includes(tag)) {
      onChange([...tags, tag]);
      const next = [tag, ...recent.filter((item) => item !== tag)].slice(0, 6);
      setRecent(next);
      client.setQueryDefaults(recentKey, { gcTime: Infinity });
      client.setQueryData(recentKey, next);
    }
    setDraft("");
  }

  function commit() {
    add(draft);
  }

  return (
    <div data-tags-field="">
      <label className="mb-1.5 block text-[0.8125rem] font-medium" htmlFor={id}>
        {t("questionEditor.tags")}
      </label>
      <div className="flex min-h-9 flex-wrap items-center gap-1.5 rounded-md border p-1.5">
        {tags.map((tag) => (
          <Badge key={tag} variant="secondary">
            {tag}
            <button
              type="button"
              aria-label={t("questionEditor.removeTag", { tag })}
              onClick={() => onChange(tags.filter((current) => current !== tag))}
            >
              <X className="size-3" aria-hidden="true" />
            </button>
          </Badge>
        ))}
        <input
          id={id}
          list={`${id}-suggestions`}
          autoComplete="off"
          value={draft}
          placeholder={t("questionEditor.addTag")}
          className="h-6 w-20 border-0 bg-transparent p-0 text-sm outline-none"
          onChange={(event) => setDraft(event.target.value)}
          onBlur={(event) => {
            if (
              !event.currentTarget
                .closest("[data-tags-field]")
                ?.contains(event.relatedTarget)
            )
              commit();
          }}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              event.preventDefault();
              commit();
            }
          }}
        />
      </div>
      <datalist id={`${id}-suggestions`}>
        {matches.map((tag) => (
          <option key={tag} value={tag} />
        ))}
      </datalist>
      {matches.length === 0 ? null : (
        <div className="mt-2 space-y-1">
          <p className="text-muted-foreground text-xs">
            {t(
              draft.trim()
                ? "questionEditor.matchingTags"
                : "questionEditor.suggestedTags",
            )}
          </p>
          <div className="flex flex-wrap gap-1.5">
            {matches.map((tag) => (
              <button
                key={tag}
                type="button"
                className="rounded-md focus-visible:outline-2 focus-visible:outline-offset-2"
                onClick={() => add(tag)}
              >
                <Badge variant="outline">{tag}</Badge>
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
