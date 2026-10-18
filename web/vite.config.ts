import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: { "/api": "http://localhost:8088" },
  },
  build: {
    rolldownOptions: {
      output: {
        // Long-lived vendor chunks cache across app deploys; pages load lazily.
        codeSplitting: {
          groups: [
            { name: "react", test: /node_modules[\\/](react|react-dom|scheduler|react-router|react-router-dom|@remix-run)[\\/]/, priority: 20 },
            { name: "mantine", test: /node_modules[\\/](@mantine|@floating-ui)[\\/]/, priority: 10 },
            { name: "icons", test: /node_modules[\\/]@tabler[\\/]/, priority: 10 },
          ],
        },
      },
    },
  },
});
