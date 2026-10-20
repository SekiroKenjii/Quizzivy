import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { pausedOpacity, type PausedOpacityUse } from "@tests/support/pausedOpacity";

const SRC = resolve(import.meta.dirname, "../../../src");

const BASELINE: Readonly<Record<string, number>> = {
  "components/shared/form/fields/SelectField.tsx": 1,
  "features/ai/components/AiEntryButton.tsx": 1,
  "features/assignments/components/SettingsGroups.tsx": 2,
  "features/assignments/pages/teacher/AssignmentDetailPage.tsx": 1,
  "features/attempts/pages/teacher/AttemptReviewPage.tsx": 2,
  "features/auth/components/SettingsSections.tsx": 1,
  "features/auth/components/StudentSettingsSections.tsx": 2,
  "features/classes/components/AddMemberDialog.tsx": 1,
  "features/classes/components/ClassCard.tsx": 1,
  "features/classes/components/ClassSettingsCard.tsx": 1,
  "features/imports/components/ReadWithAiCard.tsx": 1,
  "features/imports/components/confirm/ConfirmSummary.tsx": 1,
  "features/join/components/JoinDialog.tsx": 1,
  "features/media/components/AudioPlayer.tsx": 2,
  "features/notifications/components/NotificationsPanel.tsx": 2,
  "features/notifications/components/StudentNotificationsSection.tsx": 1,
  "features/question-bank/components/AiDistractorsPanel.tsx": 1,
  "features/question-groups/pages/teacher/GroupEditorPage.tsx": 1,
  "features/settings/sections/AiCreditRequest.tsx": 1,
  "features/settings/sections/AiPreferences.tsx": 1,
  "features/settings/sections/Appearance.tsx": 2,
  "features/settings/sections/AssignmentDefaults.tsx": 1,
  "features/settings/sections/Devices.tsx": 1,
  "features/settings/sections/Notifications.tsx": 1,
  "features/tests/components/SharedContentDialog.tsx": 1,
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

function found(root: string): Map<string, PausedOpacityUse[]> {
  const out = new Map<string, PausedOpacityUse[]>();
  for (const path of sources(root)) {
    const file = relative(root, path).replaceAll("\\", "/");
    if (file.startsWith("components/ui/")) continue;
    const uses = pausedOpacity(readFileSync(path, "utf8"), file);
    if (uses.length > 0) out.set(file, uses);
  }
  return out;
}

function disagreements(
  uses: ReadonlyMap<string, PausedOpacityUse[]>,
  baseline: Readonly<Record<string, number>>,
): string[] {
  const files = new Set([...uses.keys(), ...Object.keys(baseline)]);
  return [...files]
    .sort((a, b) => a.localeCompare(b))
    .flatMap((file) => {
      const here = uses.get(file) ?? [];
      const allowed = baseline[file] ?? 0;
      if (here.length > allowed) {
        const where = here.map((u) => `${file}:${u.line} ${u.text}`).join(", ");
        return [
          `${file}: ${here.length} paused uses of opacity, the baseline allows ${allowed}: ${where}. Fade the colours instead.`,
        ];
      }
      if (here.length < allowed)
        return [
          `${file}: ${here.length} paused uses of opacity, the baseline says ${allowed}. Lower the baseline to ${here.length}.`,
        ];
      return [];
    });
}

describe("a paused control fades its colours, never the element (D-029)", () => {
  it("matches the baseline exactly: no new use, and a use removed is taken off the baseline", () => {
    expect(
      disagreements(found(SRC), BASELINE),
      "An aria-disabled control stays focusable, and opacity on it halves its focus outline. Fade its colours (bg-primary/50, text-foreground/50) instead; when you remove a use, lower its file's count in BASELINE.",
    ).toEqual([]);
  });

  it.each([
    ["aria-disabled:opacity-50", "the plain spelling"],
    ["aria-disabled:hover:opacity-50", "behind another variant"],
    ["[&[aria-disabled]]:opacity-50", "an arbitrary variant on the attribute"],
    ["[&[aria-disabled=true]]:opacity-60", "the attribute with a value"],
    ["data-[x]:aria-disabled:opacity-40", "after another variant"],
    ["aria-[disabled=true]:opacity-50", "the arbitrary aria variant"],
    ["aria-[disabled]:opacity-50", "the bare arbitrary aria variant"],
    ["aria-disabled:!opacity-50", "important in front"],
    ["aria-disabled:opacity-50!", "important behind"],
    ["group-aria-disabled:opacity-50", "a group state"],
    ["aria-disabled:[opacity:0.5]", "an arbitrary property"],
  ])("sees %s (%s)", (token) => {
    expect(pausedOpacity(`const c = "px-2 ${token} h-7";`)).toHaveLength(1);
  });

  it.each([
    "aria-disabled:bg-primary/50",
    "aria-disabled:text-foreground/50",
    "aria-disabled:[&>span]:bg-switch-thumb/50",
    "disabled:opacity-50",
    "opacity-50",
    "transition-opacity",
    "[&>svg]:opacity-40",
  ])("lets %s through when no element is paused", (token) => {
    expect(pausedOpacity(`const c = "px-2 ${token} h-7";`)).toEqual([]);
  });

  it.each([
    ['className="opacity-50"', "a literal class"],
    ['className={cn("h-9", pending && "opacity-50")}', "a conditional class"],
    ['className={pending ? "opacity-60" : "hover:bg-muted"}', "a ternary"],
    ["className={`h-9 ${x} opacity-50`}", "a template"],
    ["style={{ opacity: pending ? 0.5 : 1 }}", "a style"],
  ])("sees opacity on an element that is paused: %s (%s)", (attributes) => {
    const source = `const a = <button aria-disabled={pending || undefined} ${attributes} />;`;
    expect(pausedOpacity(source)).toHaveLength(1);
  });

  it("lets a conditional opacity through on an element that is not paused", () => {
    const source =
      'const a = <button className={cn("h-9", pending && "opacity-50")} />;';
    expect(pausedOpacity(source)).toEqual([]);
  });

  it.each([
    ['className="hover:opacity-80 disabled:opacity-50"', "other states"],
    ['className="aria-disabled:bg-primary/50"', "a colour fade"],
  ])("lets %s through on a paused element (%s)", (attributes) => {
    expect(pausedOpacity(`const a = <button aria-disabled ${attributes} />;`)).toEqual(
      [],
    );
  });

  it("names the file, the line and the token", () => {
    const uses = pausedOpacity(
      'const a = 1;\nconst b = <b aria-disabled className={cn(x && "opacity-50")} />;',
    );
    expect(uses).toEqual([{ line: 2, text: "opacity-50" }]);
  });

  it("would be caught: a new use, a new file, and a baseline left too high", () => {
    const use = { line: 3, text: "opacity-50" };
    const uses = new Map([
      ["features/ai/components/AiEntryButton.tsx", [use, use]],
      ["features/x/New.tsx", [use]],
    ]);
    const problems = disagreements(uses, {
      "features/ai/components/AiEntryButton.tsx": 1,
      "features/gone/Old.tsx": 2,
    });
    expect(problems).toEqual([
      "features/ai/components/AiEntryButton.tsx: 2 paused uses of opacity, the baseline allows 1: features/ai/components/AiEntryButton.tsx:3 opacity-50, features/ai/components/AiEntryButton.tsx:3 opacity-50. Fade the colours instead.",
      "features/gone/Old.tsx: 0 paused uses of opacity, the baseline says 2. Lower the baseline to 0.",
      "features/x/New.tsx: 1 paused uses of opacity, the baseline allows 0: features/x/New.tsx:3 opacity-50. Fade the colours instead.",
    ]);
  });
});
