import { StrictMode, type ReactElement } from "react";
import { createRoot } from "react-dom/client";
import { DeckScale } from "@/components/ui/deck-scale";
import i18n from "@/lib/i18n";
import { readThemePreference, writeThemePreference } from "@/lib/theme";
import "@/index.css";

type Case = () => ReactElement;
type CaseFile = { cases?: Record<string, Case> };

const FILES = import.meta.glob<CaseFile>("./cases/*.tsx", { eager: true });

const CASES = new Map<string, Case>(
  Object.entries(FILES).flatMap(([path, file]) => {
    const name = path.slice("./cases/".length, -".tsx".length);
    return Object.entries(file.cases ?? {}).map(([key, Case]): [string, Case] => [
      `${name}/${key}`,
      Case,
    ]);
  }),
);

const QUERY = new URLSearchParams(window.location.search);
const NAME = QUERY.get("case");
const WIDTH = Number(QUERY.get("width"));
const Selected = NAME === null ? undefined : CASES.get(NAME);

function CaseList() {
  return (
    <ul className="flex flex-col gap-1 p-6 text-sm">
      {[...CASES.keys()]
        .sort((a, b) => a.localeCompare(b))
        .map((name) => (
          <li key={name}>
            <a className="underline" href={`?case=${encodeURIComponent(name)}`}>
              {name}
            </a>
          </li>
        ))}
    </ul>
  );
}

/**
 * Harness renders the case the address names, under DeckScale and in a
 * container of the width the address gives, or a list of links to every case
 * when it names none.
 */
export function Harness() {
  if (NAME === null) return <CaseList />;
  if (Selected === undefined) {
    return (
      <p role="alert" className="p-6 text-sm">
        No case named {NAME}.
      </p>
    );
  }
  return (
    <DeckScale className="bg-bg text-fg min-h-svh p-6">
      <div data-case={NAME} style={WIDTH > 0 ? { width: WIDTH } : undefined}>
        <Selected />
      </div>
    </DeckScale>
  );
}

writeThemePreference(readThemePreference());
document.documentElement.lang = i18n.language;

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <Harness />
  </StrictMode>,
);
