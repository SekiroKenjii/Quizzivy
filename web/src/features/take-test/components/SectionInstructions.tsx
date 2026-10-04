import { Headphones } from "lucide-react";
import { Note } from "./Note";
import type { SectionGroup } from "../sections";

/**
 * SectionInstructions states a part's instructions above its first question.
 * A listening part gets the note with the headphones, led by the part's
 * title; any other part gets one muted line.
 */
export function SectionInstructions({
  group,
  audio,
}: Readonly<{ group: SectionGroup; audio: boolean }>) {
  const text = group.section.instructions?.trim() ?? "";
  if (text === "") return null;

  if (audio) {
    return (
      <Note icon={Headphones} title={group.section.title}>
        {text}
      </Note>
    );
  }
  return (
    <p role="note" className="text-muted-fg text-sm leading-normal">
      {text}
    </p>
  );
}
