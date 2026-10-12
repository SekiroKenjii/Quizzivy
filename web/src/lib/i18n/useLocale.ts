import { useSyncExternalStore } from "react";
import { useTranslation } from "react-i18next";
import {
  SUPPORTED_LOCALES,
  chosenLocale,
  subscribeChosenLocale,
  type Locale,
} from "@/lib/i18n";

export function asLocale(language: string): Locale {
  return (SUPPORTED_LOCALES as readonly string[]).includes(language)
    ? (language as Locale)
    : "vi";
}

/** The active locale, narrowed to the ones the app ships. */
export function useLocale(): Locale {
  return asLocale(useTranslation().i18n.language);
}

/**
 * useChosenLocale returns the language chosen last, which can still be
 * loading while the screen shows the previous one. A language picker binds
 * to it.
 */
export function useChosenLocale(): Locale {
  return asLocale(useSyncExternalStore(subscribeChosenLocale, chosenLocale));
}
