import { useTranslation } from "react-i18next";
import { SUPPORTED_LOCALES, type Locale } from "@/lib/i18n";

export function asLocale(language: string): Locale {
  return (SUPPORTED_LOCALES as readonly string[]).includes(language)
    ? (language as Locale)
    : "vi";
}

/** The active locale, narrowed to the ones the app ships. */
export function useLocale(): Locale {
  return asLocale(useTranslation().i18n.language);
}
