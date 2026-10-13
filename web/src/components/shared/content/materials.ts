import type { components } from "@/lib/api/schema";
import { nfc } from "@/lib/nfc";

type StudentGroup = components["schemas"]["StudentGroup"];
type Material = StudentGroup["stimuli"][number];

/**
 * hasReading reports whether a material holds something to read: any block
 * that is not an audio node, or legacy Markdown. A material of recordings
 * only is listened to, and its name, often the file's, is never printed for
 * a student.
 */
export function hasReading(material: Material): boolean {
  const content = material.content;
  if (content.format !== "semantic_v1") return true;
  return content.blocks.some((block) => block.type !== "audio");
}

/**
 * MaterialsHead is what heads a group's materials in the stimulus pane: the
 * small uppercase kicker, and the 22px heading when there is one.
 */
export interface MaterialsHead {
  kicker: string;
  heading: string | undefined;
}

/**
 * materialsHead heads a group's materials as the deck does: the heading is
 * the title of the first material with something to read, and the kicker is
 * the group's title, or `sectionTitle` when the heading already says the
 * group's title. A group whose materials are all recordings has a kicker and
 * no heading. Both come back in NFC.
 */
export function materialsHead(group: StudentGroup, sectionTitle = ""): MaterialsHead {
  const first = group.stimuli.find(hasReading);
  const heading = first === undefined ? undefined : nfc(first.title);
  const title = nfc(group.title);
  return { kicker: heading === title ? nfc(sectionTitle) : title, heading };
}
