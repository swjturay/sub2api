import { defineConfig } from "vitest/config";
import { loadEnv } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const proxyTarget = env.INSIGHTS_API_PROXY_TARGET || "http://127.0.0.1:8080";
  return {
    base: "/insights/",
    plugins: [react(), tailwindcss()],
    server: {
      port: 4178,
      proxy: {
        // Keep native login, callbacks and assets on the same development origin.
        "^/(?!insights(?:[/?#]|$))": {
          target: proxyTarget,
          changeOrigin: true,
          secure: env.INSIGHTS_API_PROXY_VERIFY_TLS !== "false",
          ws: true,
        },
      },
    },
    build: { outDir: "dist", sourcemap: false },
    test: {
      environment: "jsdom",
      setupFiles: ["./src/test/setup.ts"],
      css: true,
    },
  };
});
