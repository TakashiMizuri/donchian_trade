import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig, type Plugin } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

const root = path.dirname(fileURLToPath(import.meta.url));

function localInstancesPlugin(): Plugin {
  return {
    name: "donchian-local-instances",
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        if (req.url?.split("?")[0] === "/api/instances") {
          res.setHeader("Content-Type", "application/json");
          res.end(
            JSON.stringify({
              instances: [
                { id: "a", name: "local", prefix: "/api/a" },
                { id: "b", name: "local-b", prefix: "/api/b" },
              ],
            }),
          );
          return;
        }
        next();
      });
    },
  };
}

export default defineConfig({
  plugins: [react(), tailwindcss(), localInstancesPlugin()],
  resolve: {
    alias: {
      "@": path.resolve(root, "./src"),
    },
  },
  server: {
    port: 5173,
    proxy: {
      // Dual-prefix UI; locally both map to one bot on :8080.
      "/api/a": {
        target: "http://127.0.0.1:8080",
        rewrite: (p) => p.replace(/^\/api\/a/, "/api"),
      },
      "/api/b": {
        target: "http://127.0.0.1:8080",
        rewrite: (p) => p.replace(/^\/api\/b/, "/api"),
      },
      "/api": "http://127.0.0.1:8080",
    },
  },
});
