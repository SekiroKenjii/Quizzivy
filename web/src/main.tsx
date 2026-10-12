import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./index.css";
import { localeReady } from "./lib/i18n";
import { AppProviders } from "@/app/providers";

const root = createRoot(document.getElementById("root")!);

function render() {
  root.render(
    <StrictMode>
      <AppProviders />
    </StrictMode>,
  );
}

if (localeReady === null) render();
else void localeReady.then(render);
