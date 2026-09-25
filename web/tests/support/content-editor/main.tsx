import { useState } from "react";
import { createRoot } from "react-dom/client";
import { useTranslation } from "react-i18next";
import { FilePenLine, Eye } from "lucide-react";
import { ContentEditor } from "@/components/shared/content/editor/ContentEditor";
import { ContentView } from "@/components/shared/content/ContentView";
import type { SemanticContent } from "@/components/shared/content/model";
import { cn } from "@/lib/utils";
import "@/lib/i18n";
import "@/index.css";
import { formattingSample, tableSample } from "./fixtures";

/** EditorPrototype exercises isolated local edits using synthetic content only. */
export function EditorPrototype() {
  const { t } = useTranslation();
  const [selected, setSelected] = useState("sample1");
  const [documents, setDocuments] = useState<Record<string, SemanticContent>>({
    sample1: formattingSample,
    sample2: tableSample,
  });
  const document = documents[selected]!;
  return (
    <main className="bg-muted/25 min-h-screen px-4 py-6 sm:px-8 lg:px-12">
      <header className="mb-8 border-b pb-6">
        <p className="text-muted-foreground mb-2 text-xs font-medium tracking-wide">
          {t("contentEditor.spikeLabel")}
        </p>
        <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">
          {t("contentEditor.spikeTitle")}
        </h1>
        <p className="text-muted-foreground mt-3 max-w-3xl text-sm leading-6">
          {t("contentEditor.spikeDescription")}
        </p>
      </header>
      <div
        className="mb-6 flex flex-wrap gap-2"
        role="group"
        aria-label={t("contentEditor.edit")}
      >
        {["sample1", "sample2"].map((id) => (
          <button
            key={id}
            type="button"
            aria-pressed={selected === id}
            onClick={() => setSelected(id)}
            className={cn(
              "bg-card focus-visible:outline-ring rounded-lg border px-4 py-2.5 text-sm shadow-sm transition-colors focus-visible:outline-2 motion-reduce:transition-none",
              selected === id
                ? "border-primary bg-primary text-primary-foreground"
                : "hover:bg-muted",
            )}
          >
            {t(`contentEditor.${id}`)}
          </button>
        ))}
      </div>
      <div className="grid min-w-0 items-start gap-6 xl:grid-cols-2">
        <section className="min-w-0">
          <div className="mb-3 flex items-center gap-2">
            <FilePenLine size={17} aria-hidden="true" />
            <h2 className="text-sm font-medium">{t("contentEditor.edit")}</h2>
          </div>
          <ContentEditor
            key={selected}
            initialContent={document}
            label={t("contentEditor.edit")}
            onChange={(content) =>
              setDocuments((current) => ({ ...current, [selected]: content }))
            }
          />
          <p role="status" className="text-muted-foreground mt-3 text-xs">
            {t("contentEditor.sessionOnly")}
          </p>
        </section>
        <section className="min-w-0" aria-label={t("contentEditor.preview")}>
          <div className="mb-3 flex items-center gap-2">
            <Eye size={17} aria-hidden="true" />
            <h2 className="text-sm font-medium">{t("contentEditor.preview")}</h2>
          </div>
          <div className="bg-card rounded-lg border p-5 shadow-sm">
            <ContentView document={document} />
          </div>
          <p className="text-muted-foreground mt-3 text-xs">
            {t("contentEditor.previewHelp")}
          </p>
        </section>
      </div>
    </main>
  );
}

createRoot(document.getElementById("root")!).render(<EditorPrototype />);
