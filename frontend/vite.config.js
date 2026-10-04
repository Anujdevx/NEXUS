import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

/* The backend is reached through the Traefik gateway (default http://localhost:8000; scripts/start-all.sh
   sets NEXUS_GATEWAY when it had to pick another port). Proxying avoids CORS in development and preview. */
const gateway = process.env.NEXUS_GATEWAY || 'http://localhost:8000';
const proxy = {
  '/api': { target: gateway, changeOrigin: true },
  '/svc': { target: gateway, changeOrigin: true },
  '/ws': { target: gateway.replace(/^http/, 'ws'), ws: true, changeOrigin: true }
};

export default defineConfig({ plugins: [react()], server: { port: 5173, open: true, host: true, proxy }, preview: { proxy } });
