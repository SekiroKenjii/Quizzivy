import { useEffect, useSyncExternalStore } from "react";

/** ThemePreference is what the user chose; `system` follows the device. */
export type ThemePreference = "light" | "dark" | "system";

/** ResolvedTheme is the theme on screen. */
export type ResolvedTheme = "light" | "dark";

const STORAGE_KEY = "quizzivy.theme";
const DARK_QUERY = "(prefers-color-scheme: dark)";

const listeners = new Set<() => void>();
let forcedLight = 0;

/**
 * readThemePreference returns the stored preference, or `light` when nothing
 * valid is stored or storage is unavailable.
 */
export function readThemePreference(): ThemePreference {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === "dark" || stored === "system") return stored;
  } catch {
    return "light";
  }
  return "light";
}

/** writeThemePreference stores the preference and applies it at once. */
export function writeThemePreference(preference: ThemePreference): void {
  try {
    localStorage.setItem(STORAGE_KEY, preference);
  } catch {
    // The theme still applies for this page; it is only not remembered.
  }
  apply();
}

function systemIsDark(): boolean {
  return (
    typeof window.matchMedia === "function" && window.matchMedia(DARK_QUERY).matches
  );
}

function resolve(): ResolvedTheme {
  if (forcedLight > 0) return "light";
  const preference = readThemePreference();
  if (preference === "system") return systemIsDark() ? "dark" : "light";
  return preference;
}

function setThemeColor() {
  for (const meta of document.head.querySelectorAll('meta[name="theme-color"]'))
    meta.remove();
  const background = getComputedStyle(document.documentElement)
    .getPropertyValue("--bg")
    .trim();
  if (!background) return;
  const meta = document.createElement("meta");
  meta.name = "theme-color";
  meta.content = background;
  document.head.append(meta);
}

function apply() {
  const resolved = resolve();
  const root = document.documentElement;
  root.classList.toggle("dark", resolved === "dark");
  root.style.colorScheme = resolved;
  setThemeColor();
  for (const listener of listeners) listener();
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  const media =
    typeof window.matchMedia === "function" ? window.matchMedia(DARK_QUERY) : null;
  const onChange = () => {
    if (readThemePreference() === "system") apply();
  };
  media?.addEventListener("change", onChange);
  return () => {
    listeners.delete(listener);
    media?.removeEventListener("change", onChange);
  };
}

/**
 * useResolvedTheme returns the theme on screen and re-renders when the
 * preference, the device setting or a forced light theme changes it.
 */
export function useResolvedTheme(): ResolvedTheme {
  return useSyncExternalStore(subscribe, resolve, () => "light");
}

/**
 * useForcedLightTheme keeps the page light while the calling component is
 * mounted, whatever the preference, and restores the preference on unmount.
 * Screens not yet rebuilt to the deck call it: they were never drawn dark.
 */
export function useForcedLightTheme(): void {
  useEffect(() => {
    forcedLight += 1;
    apply();
    return () => {
      forcedLight -= 1;
      apply();
    };
  }, []);
}
