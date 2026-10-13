import { useState } from "react";
import { useAiAvailable, useAiBlock, type AiBlock } from "@/features/ai/useAiAvailable";
import { aiPreferencesOf } from "@/features/auth/preferences";
import { useAuthStore } from "@/stores/auth";

/**
 * ReadWithAi is an import's "Read with AI" choice: whether the card is
 * offered, why it cannot be turned on now, whether the import was already
 * created with it (`locked`), whether a start asks for it (`asks`), and
 * whether the files are, or will be, read with AI (`on`). `asks` is never
 * true while the choice is blocked.
 */
export interface ReadWithAi {
  offered: boolean;
  blocked: AiBlock | null;
  locked: boolean;
  asks: boolean;
  on: boolean;
  choose: (on: boolean) => void;
}

/**
 * useReadWithAi holds an import's "Read with AI" choice. While `eligible` (a
 * new file import whose server takes PDFs) and the assistant is available
 * (`useAiAvailable`), the card is offered; it starts from the teacher's "Read
 * imports with AI" preference, and changing it changes this import alone,
 * never the preference. `created` is the import's own choice once it exists,
 * and null before; from then on the choice is locked to it.
 */
export function useReadWithAi(eligible: boolean, created: boolean | null): ReadWithAi {
  const offered = useAiAvailable() && eligible;
  const blocked = useAiBlock(offered && created === null);
  const preferred = useAuthStore(
    (s) => aiPreferencesOf(s.user?.preferences?.ai, "vi").readImports,
  );
  const [chosen, setChosen] = useState<boolean | null>(null);
  const asks = offered && blocked === null && (chosen ?? preferred);
  return {
    offered,
    blocked,
    locked: created !== null,
    asks,
    on: created ?? asks,
    choose: setChosen,
  };
}
