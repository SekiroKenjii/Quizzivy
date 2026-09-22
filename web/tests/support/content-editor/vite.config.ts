import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";

const root = path.resolve(import.meta.dirname, "../../..");
export default defineConfig({
  root,
  plugins: [react(), tailwindcss()],
  envDir: false,
  resolve: { alias: { "@": path.join(root, "src") } },
  build: {
    outDir: "dist/content-editor",
    manifest: true,
    rolldownOptions: {
      input: {
        editor: path.join(import.meta.dirname, "index.html"),
        reader: path.join(import.meta.dirname, "reader.html"),
      },
    },
  },
  preview: { host: "localhost", port: 4174, strictPort: true },
});
