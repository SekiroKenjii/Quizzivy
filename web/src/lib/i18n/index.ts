import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import vi from "./locales/vi.json";

/**
 * Vietnamese is the product language (§2). `vi` is the default and the
 * fallback, deliberately: if a key is missing, a Vietnamese string is a far
 * better failure than an English one in front of a student.
 */
export const SUPPORTED_LOCALES = ["vi", "en"] as const;
export type Locale = (typeof SUPPORTED_LOCALES)[number];
export const DEFAULT_LOCALE: Locale = "vi";

const STORAGE_KEY = "quizzivy.locale";

function initialLocale(): Locale {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored && (SUPPORTED_LOCALES as readonly string[]).includes(stored)) {
      return stored as Locale;
    }
  } catch {
    // Private mode, or storage disabled. Fall through to the default.
  }
  return DEFAULT_LOCALE;
}

void i18n.use(initReactI18next).init({
  resources: { vi: { translation: vi } },
  partialBundledLanguages: true,
  lng: initialLocale(),
  fallbackLng: DEFAULT_LOCALE,
  interpolation: { escapeValue: false }, // React already escapes
  returnNull: false,
});

/**
 * loadLocale makes `locale`'s strings available. `vi` is bundled; `en` is a
 * separate chunk fetched on first use, so its strings never weigh on a
 * Vietnamese reader's download. It resolves at once when the strings are
 * already loaded and rejects when the chunk cannot be fetched.
 */
export async function loadLocale(locale: Locale): Promise<void> {
  if (i18n.hasResourceBundle(locale, "translation")) return;
  const strings = await import("./locales/en.json");
  i18n.addResourceBundle(locale, "translation", strings.default, true, true);
}

/**
 * localeReady is what the first render waits for: null when the starting
 * language's strings are bundled, or the load of the stored language's
 * strings, which settles either way so a failed fetch still renders, in the
 * fallback language.
 */
export const localeReady: Promise<void> | null = i18n.hasResourceBundle(
  i18n.language,
  "translation",
)
  ? null
  : loadLocale(i18n.language as Locale).then(
      () => i18n.changeLanguage(i18n.language).then(() => undefined),
      () => undefined,
    );

let latest = 0;

function apply(locale: Locale): Promise<void> {
  const changed = i18n.changeLanguage(locale);
  document.documentElement.lang = locale;
  return changed.then(() => undefined);
}

/**
 * setLocale applies the current explicit language choice and mirrors it when
 * storage permits. With the language's strings loaded it switches at once, as
 * it always has; otherwise it switches once they arrive, unless a later
 * choice came first, and stays on the current language if they cannot be
 * fetched. The promise always resolves.
 */
export function setLocale(locale: Locale): Promise<void> {
  const request = ++latest;
  try {
    localStorage.setItem(STORAGE_KEY, locale);
  } catch {
    // Preference is not persisted; the app still works.
  }
  if (i18n.hasResourceBundle(locale, "translation")) return apply(locale);
  return loadLocale(locale).then(
    () => (request === latest ? apply(locale) : undefined),
    () => undefined,
  );
}

export default i18n;
