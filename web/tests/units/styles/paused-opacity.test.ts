import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { describe, expect, it } from "vitest";

const SRC = resolve(import.meta.dirname, "../../../src");
const FORM = "aria-disabled:opacity-";

const BASELINE: Readonly<Record<string, number>> = {
  "features/ai/components/AiEntryButton.tsx": 1,
  "features/assignments/components/SettingsGroups.tsx": 2,
  "features/assignments/pages/teacher/AssignmentDetailPage.tsx": 1,
  "features/auth/components/SettingsSections.tsx": 1,
  "features/auth/components/StudentSettingsSections.tsx": 2,
  "features/classes/components/ClassCard.tsx": 1,
  "features/classes/components/ClassSettingsCard.tsx": 1,
  "features/imports/components/ReadWithAiCard.tsx": 1,
  "features/imports/components/confirm/ConfirmSummary.tsx": 1,
  "features/join/components/JoinDialog.tsx": 1,
  "features/media/components/AudioPlayer.tsx": 2,
  "features/notifications/components/NotificationsPanel.tsx": 2,
  "features/notifications/components/StudentNotificationsSection.tsx": 1,
  "features/question-groups/pages/teacher/GroupEditorPage.tsx": 1,
  "features/settings/sections/AiCreditRequest.tsx": 1,
  "features/settings/sections/AiPreferences.tsx": 1,
  "features/settings/sections/Appearance.tsx": 2,
  "features/settings/sections/AssignmentDefaults.tsx": 1,
  "features/settings/sections/Devices.tsx": 1,
  "features/settings/sections/Notifications.tsx": 1,
  "features/tests/components/VersionHistory.tsx": 1,
  "features/tests/pages/teacher/TestDetailPage.tsx": 1,
};

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return sources(path);
    return /\.tsx?$/.test(name) ? [path] : [];
  });
}

function counts(root: string): Map<string, number> {
  const found = new Map<string, number>();
  for (const path of sources(root)) {
    const file = relative(root, path).replaceAll("\\", "/");
    if (file.startsWith("components/ui/")) continue;
    const count = readFileSync(path, "utf8").split(FORM).length - 1;
    if (count > 0) found.set(file, count);
  }
  return found;
}

function overBaseline(
  found: ReadonlyMap<string, number>,
  baseline: Readonly<Record<string, number>>,
): string[] {
  return [...found].flatMap(([file, count]) => {
    const allowed = baseline[file];
    if (allowed === undefined)
      return [`${file}: ${count} use of ${FORM}* in a file with none on the baseline`];
    return count > allowed
      ? [`${file}: ${count} uses of ${FORM}*, the baseline allows ${allowed}`]
      : [];
  });
}

describe("a paused control fades its colours, never the element (D-029)", () => {
  it("adds no aria-disabled:opacity-* beyond the baseline, which only goes down", () => {
    const offenders = overBaseline(counts(SRC), BASELINE);
    expect(
      offenders,
      "An aria-disabled control stays focusable, and opacity on it halves its focus outline. Fade its colours (text-foreground/50, bg-primary/50) instead.",
    ).toEqual([]);
  });

  it("keeps the baseline honest: a file that left the list is removed from it", () => {
    const found = counts(SRC);
    const stale = Object.keys(BASELINE).filter((file) => !found.has(file));
    expect(stale).toEqual([]);
  });

  it("would be caught: a new use, a new file", () => {
    const found = new Map([
      ["features/ai/components/AiEntryButton.tsx", 2],
      ["features/x/New.tsx", 1],
    ]);
    expect(overBaseline(found, BASELINE)).toEqual([
      "features/ai/components/AiEntryButton.tsx: 2 uses of aria-disabled:opacity-*, the baseline allows 1",
      "features/x/New.tsx: 1 use of aria-disabled:opacity-* in a file with none on the baseline",
    ]);
  });
});
