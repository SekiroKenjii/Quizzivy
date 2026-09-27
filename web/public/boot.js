(function () {
  var root = document.documentElement;
  var theme = "light";
  var locale = null;
  try {
    var stored = localStorage.getItem("quizzivy.theme");
    if (stored === "dark" || stored === "system") theme = stored;
    locale = localStorage.getItem("quizzivy.locale");
  } catch {
    theme = "light";
  }
  var dark =
    theme === "dark" ||
    (theme === "system" && window.matchMedia("(prefers-color-scheme: dark)").matches);
  root.classList.toggle("dark", dark);
  root.style.colorScheme = dark ? "dark" : "light";
  if (locale === "vi" || locale === "en") root.lang = locale;
  function color(content, scheme) {
    var meta = document.createElement("meta");
    meta.name = "theme-color";
    meta.content = content;
    if (scheme) meta.media = "(prefers-color-scheme: " + scheme + ")";
    document.head.appendChild(meta);
  }
  if (theme === "system") {
    color("#ffffff", "light");
    color("#0e1213", "dark");
  } else {
    color(dark ? "#0e1213" : "#ffffff");
  }
})();
