import { SUPPORTED_LOCALES, type Locale } from "@/lib/i18n";
import type { ThemePreference } from "@/lib/theme";

/**
 * DeviceChoices are the presentation choices a person made on this browser
 * while no account held them: the theme, the language and "Larger text in
 * tests", each absent when nothing valid was chosen.
 */
export type DeviceChoices = {
  theme?: ThemePreference;
  locale?: Locale;
  largerTestText?: boolean;
};

/** DeviceChoiceName names one of the three device choices. */
export type DeviceChoiceName = keyof DeviceChoices;

const STORAGE_KEYS: Record<DeviceChoiceName, string> = {
  theme: "quizzivy.theme",
  locale: "quizzivy.locale",
  largerTestText: "quizzivy.testText",
};
const MIRROR_KEY = "quizzivy.preferenceMirror";
const THEMES: ReadonlySet<string> = new Set<ThemePreference>([
  "light",
  "dark",
  "system",
]);
const LOCALES: ReadonlySet<string> = new Set<string>(SUPPORTED_LOCALES);
const TEST_TEXT: ReadonlyMap<string, boolean> = new Map([
  ["large", true],
  ["default", false],
]);

function mirrored(): Set<string> {
  try {
    return new Set((localStorage.getItem(MIRROR_KEY) ?? "").split(" ").filter(Boolean));
  } catch {
    return new Set();
  }
}

function writeMirrored(names: ReadonlySet<string>): void {
  try {
    if (names.size === 0) localStorage.removeItem(MIRROR_KEY);
    else localStorage.setItem(MIRROR_KEY, [...names].join(" "));
  } catch {
    return;
  }
}

function stored(name: DeviceChoiceName): string | null {
  try {
    return localStorage.getItem(STORAGE_KEYS[name]);
  } catch {
    return null;
  }
}

/**
 * readDeviceChoices returns the valid choices this browser stores under the
 * keys v0.9.0 wrote, leaving out a value the app copied there from an
 * account (markMirrored) and any value storage cannot give.
 */
export function readDeviceChoices(): DeviceChoices {
  const copied = mirrored();
  const choices: DeviceChoices = {};
  const theme = copied.has("theme") ? null : stored("theme");
  if (theme !== null && THEMES.has(theme)) choices.theme = theme as ThemePreference;
  const locale = copied.has("locale") ? null : stored("locale");
  if (locale !== null && LOCALES.has(locale)) choices.locale = locale as Locale;
  const text = copied.has("largerTestText") ? null : stored("largerTestText");
  const larger = text === null ? undefined : TEST_TEXT.get(text);
  if (larger !== undefined) choices.largerTestText = larger;
  return choices;
}

/**
 * markMirrored records that the named keys now hold an account's value, so a
 * later sign-in on this browser does not take them for a device choice.
 */
export function markMirrored(names: readonly DeviceChoiceName[]): void {
  if (names.length === 0) return;
  const copied = mirrored();
  for (const name of names) copied.add(name);
  writeMirrored(copied);
}

/**
 * markChosen records that the named key holds a choice made on this browser
 * with no account signed in.
 */
export function markChosen(name: DeviceChoiceName): void {
  const copied = mirrored();
  if (!copied.delete(name)) return;
  writeMirrored(copied);
}
