import { useTranslation } from "react-i18next";
import { Headphones } from "lucide-react";
import { Note } from "./Note";
import type { SectionGroup } from "../sections";

/**
 * A section's instructions, above its first question (S-05). A listening
 * section gets the boxed note with the headphones, led by the section's
 * title; any other section gets the plain muted line the fill-blank frame draws.
 */
export function SectionInstructions({
  group,
  audio,
}: Readonly<{ group: SectionGroup; audio: boolean }>) {
  const { t } = useTranslation();
  const text = group.section.instructions?.trim() ?? "";
  if (text === "") return null;

  if (audio) {
    return (
      <Note icon={Headphones}>
        {t("takeTest.sectionNote", { title: group.section.title, text })}
      </Note>
    );
  }
  return (
    <p role="note" className="text-muted-foreground text-xs leading-relaxed">
      {text}
    </p>
  );
}
