import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./index.css";
// Side-effect import: configures i18next before any component renders.
import "./lib/i18n";
import { AppProviders } from "@/app/providers";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <AppProviders />
  </StrictMode>,
);
