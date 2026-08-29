import { useState } from "react";
import { useTranslation } from "react-i18next";
import { X } from "lucide-react";
import { Badge } from "@/components/ui/badge";

interface TagsFieldProps {
  tags: string[];
  onChange: (tags: string[]) => void;
}

/** The deck's A-04 tag field: chips in a bordered box with an inline input. */
export function TagsField({ tags, onChange }: TagsFieldProps) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState("");

  function commit() {
    const tag = draft.trim();
    if (tag !== "" && !tags.includes(tag)) onChange([...tags, tag]);
    setDraft("");
  }

  return (
    <div>
      <label
        className="mb-1.5 block text-[0.8125rem] font-medium"
        htmlFor="question-tags"
      >
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
          id="question-tags"
          value={draft}
          placeholder={t("questionEditor.addTag")}
          className="h-6 w-20 border-0 bg-transparent p-0 text-sm outline-none"
          onChange={(event) => setDraft(event.target.value)}
          onBlur={commit}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              event.preventDefault();
              commit();
            }
          }}
        />
      </div>
    </div>
  );
}
