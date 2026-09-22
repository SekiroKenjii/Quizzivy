import { createRoot } from "react-dom/client";
import { ContentView } from "@/components/shared/content/ContentView";
import "@/lib/i18n";
import "@/index.css";
import { formattingSample, tableSample } from "./fixtures";

createRoot(document.getElementById("root")!).render(
  <main className="mx-auto max-w-3xl space-y-8 p-6">
    <ContentView document={formattingSample} />
    <ContentView document={tableSample} />
  </main>,
);
