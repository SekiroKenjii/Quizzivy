import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";

const root = path.resolve(import.meta.dirname, "../../..");
export default defineConfig({
  root,
  plugins: [react(), tailwindcss()],
  envDir: false,
  define: {
    __APP_BUILD__: JSON.stringify("deck-harness"),
    __APP_VERSION__: JSON.stringify("0.0.0"),
  },
  resolve: { alias: { "@": path.join(root, "src") } },
  build: {
    outDir: "dist/deck-harness",
    rolldownOptions: {
      input: { harness: path.join(import.meta.dirname, "index.html") },
    },
  },
  preview: { host: "localhost", port: 4175, strictPort: true },
});
