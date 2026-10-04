import { defineConfig } from "@playwright/test";
export default defineConfig({ testDir: "tests", timeout: 240000, workers: 2, use: { viewport: { width: 1536, height: 960 } } });
