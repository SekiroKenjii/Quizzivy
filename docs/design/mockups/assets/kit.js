/* Deck chrome only — nothing here describes product behaviour. */

const STORE_KEY = "quizzivy-deck-prefs";

function readPrefs() {
  try {
    return JSON.parse(localStorage.getItem(STORE_KEY) || "{}");
  } catch {
    return {};
  }
}

function writePrefs(patch) {
  try {
    localStorage.setItem(STORE_KEY, JSON.stringify({ ...readPrefs(), ...patch }));
  } catch {
    /* private windows and file:// both throw; the deck still works without it */
  }
}

function applyTheme(theme) {
  document.documentElement.classList.toggle("dark", theme === "dark");
  document.querySelectorAll("[data-theme-label]").forEach((el) => {
    el.textContent = theme === "dark" ? "Sáng" : "Tối";
  });
}

function applyZoom(zoom) {
  document.documentElement.style.setProperty("--frame-zoom", String(zoom));
  document.querySelectorAll(".frame-desktop, .frame-wide, .frame-tablet").forEach((el) => {
    el.style.zoom = String(zoom);
  });
  document.querySelectorAll("[data-zoom-label]").forEach((el) => {
    el.textContent = `${Math.round(zoom * 100)}%`;
  });
}

function initDeck() {
  const prefs = readPrefs();
  applyTheme(prefs.theme || "light");
  applyZoom(prefs.zoom || 1);

  document.querySelectorAll("[data-theme-toggle]").forEach((btn) => {
    btn.addEventListener("click", () => {
      const next = document.documentElement.classList.contains("dark") ? "light" : "dark";
      applyTheme(next);
      writePrefs({ theme: next });
    });
  });

  document.querySelectorAll("[data-zoom]").forEach((btn) => {
    btn.addEventListener("click", () => {
      const step = Number(btn.dataset.zoom);
      const current = Number(getComputedStyle(document.documentElement).getPropertyValue("--frame-zoom")) || 1;
      const next = Math.min(1, Math.max(0.5, Math.round((current + step) * 20) / 20));
      applyZoom(next);
      writePrefs({ zoom: next });
    });
  });
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", initDeck);
} else {
  initDeck();
}
