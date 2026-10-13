import { useId } from "react";
import { useTranslation } from "react-i18next";
import { Check, Plus, Sparkles, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { AiDistractor } from "@/features/ai/api";
import { AiCreditNote } from "@/features/ai/components/AiCreditNote";
import { cn } from "@/lib/utils";

/**
 * AiDistractorsPanel is "Suggested wrong options" under a choice question's
 * options (V-10): a row for each suggestion, its text and why a learner might
 * pick it, with "Add", which reads "Added" once taken; or, when the call
 * failed or found none, the sentence that says why. "Close suggestions" closes
 * it, and `note` is what the call cost and what is left. The panel saves
 * nothing: "Add" hands the text to the options.
 */
export function AiDistractorsPanel({
  items,
  error,
  note,
  added,
  onAdd,
  onClose,
}: Readonly<{
  items: readonly AiDistractor[];
  error: string | null;
  note: string | null;
  added: ReadonlySet<number>;
  onAdd: (index: number) => void;
  onClose: () => void;
}>) {
  const { t } = useTranslation();
  const title = useId();
  const row = useId();
  return (
    <section
      aria-labelledby={title}
      className="bg-brand-soft flex flex-col gap-2 rounded-[10px] border p-3"
    >
      <div className="flex items-center justify-between gap-2">
        <h3
          id={title}
          className="text-brand-ink text-meta flex items-center gap-1.5 leading-normal font-semibold"
        >
          <Sparkles aria-hidden="true" className="size-3.25 flex-none" />
          {t("ai.distract.title")}
        </h3>
        <Button
          variant="ghost"
          size="md"
          aria-label={t("ai.distract.close")}
          className="text-muted-fg hover:bg-card hover:text-fg dark:hover:bg-card size-6.5 flex-none rounded-[6px] p-0 [&_svg:not([class*='size-'])]:size-3.5"
          onClick={onClose}
        >
          <X aria-hidden="true" />
        </Button>
      </div>
      {error === null ? null : <p className="text-sm leading-normal">{error}</p>}
      {items.length === 0 ? null : (
        <ul className="flex flex-col gap-2">
          {items.map((item, index) => {
            const done = added.has(index);
            return (
              <li
                key={`${index}-${item.text}`}
                className="bg-card flex items-center gap-2.5 rounded-[8px] border py-2 pr-2 pl-3"
              >
                <span id={`${row}-${index}`} className="min-w-0 flex-1">
                  <span className="text-ui block leading-[1.5] font-medium break-words">
                    {item.text}
                  </span>
                  <span className="text-muted-fg block text-xs leading-normal text-pretty">
                    {item.why}
                  </span>
                </span>
                <Button
                  variant="outline"
                  size="md"
                  aria-describedby={`${row}-${index}`}
                  aria-disabled={done || undefined}
                  className={cn(
                    "text-meta rounded-seg h-7 flex-none gap-1.25 px-2.5 [&_svg:not([class*='size-'])]:size-3.25",
                    done && "aria-disabled:hover:bg-card cursor-default opacity-55",
                  )}
                  onClick={() => {
                    if (!done) onAdd(index);
                  }}
                >
                  {done ? <Check aria-hidden="true" /> : <Plus aria-hidden="true" />}
                  {t(done ? "ai.distract.added" : "ai.distract.add")}
                </Button>
              </li>
            );
          })}
        </ul>
      )}
      {note === null ? null : (
        <div className="flex">
          <AiCreditNote text={note} warning={false} />
        </div>
      )}
    </section>
  );
}
