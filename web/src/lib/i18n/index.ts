import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import vi from "./locales/vi.json";
import en from "./locales/en.json";

/**
 * Vietnamese is the product language (§2). `vi` is the default and the
 * fallback, deliberately: if a key is missing, a Vietnamese string is a far
 * better failure than an English one in front of a student.
 *
 * AGENTS.md: write the `vi` string first, then `en`. parity.test.ts fails the
 * build if the two drift apart.
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
  resources: {
    vi: { translation: vi },
    en: { translation: en },
  },
  lng: initialLocale(),
  fallbackLng: DEFAULT_LOCALE,
  interpolation: { escapeValue: false }, // React already escapes
  returnNull: false,
});

export function setLocale(locale: Locale) {
  void i18n.changeLanguage(locale);
  document.documentElement.lang = locale;
  try {
    localStorage.setItem(STORAGE_KEY, locale);
  } catch {
    // Preference is not persisted; the app still works.
  }
}

export default i18n;
