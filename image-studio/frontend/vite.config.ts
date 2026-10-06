import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import pkg from "./package.json";

const targetPlatform = (process.env.VITE_TARGET_PLATFORM ?? "").trim().toLowerCase();
const isAndroidWebViewTarget = targetPlatform === "android" || targetPlatform === "android-pad";
const explicitBase = (process.env.VITE_BUILD_BASE ?? "").trim();
const explicitOutDir = (process.env.VITE_OUT_DIR ?? "").trim();

function manualChunks(id: string) {
  if (id.includes("/wailsjs/")) return "wails-runtime";
  if (id.includes("/node_modules/")) {
    if (id.includes("/react-konva/") || id.includes("/konva/")) return "canvas-vendor";
    if (id.includes("/react/") || id.includes("/react-dom/") || id.includes("/scheduler/")) return "react-vendor";
    if (id.includes("/lucide-react/")) return "icon-vendor";
    return "vendor";
  }
  return undefined;
}

// https://vitejs.dev/config/
export default defineConfig({
  base: explicitBase || (isAndroidWebViewTarget ? "./" : "/"),
  build: {
    ...(explicitOutDir ? { outDir: explicitOutDir } : {}),
    ...(isAndroidWebViewTarget ? { target: "chrome70" } : {}),
    rollupOptions: {
      output: {
        manualChunks,
      },
    },
  },
  define: {
    "import.meta.env.PACKAGE_VERSION": JSON.stringify(pkg.version),
  },
  plugins: [react(), tailwindcss()],
});
