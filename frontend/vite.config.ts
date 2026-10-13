import path from 'path'
import fs from 'fs'
import os from 'os'
import {defineConfig} from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

function getBackendPort(): number {
  if (process.env.BACKEND_PORT) {
    const envPort = parseInt(process.env.BACKEND_PORT, 10);
    if (!isNaN(envPort) && envPort > 0) return envPort;
  }
  try {
    const configPath = path.join(os.homedir(), '.config', 'EposProxy', 'config.json');
    if (fs.existsSync(configPath)) {
      const data = JSON.parse(fs.readFileSync(configPath, 'utf8'));
      if (typeof data.port === 'number' && data.port > 0) {
        return data.port;
      }
    }
  } catch {}
  return 4545;
}

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [tailwindcss(), react()],
  server: {
    host: "127.0.0.1",
    port: 5173,
    strictPort: true,
    cors: true,
    fs: {
      allow: [".."],
    },
    proxy: {
      "/api": {
        target: `http://127.0.0.1:${getBackendPort()}`,
        changeOrigin: true,
        router: () => {
          return `http://127.0.0.1:${getBackendPort()}`;
        },
        configure: (proxy) => {
          proxy.on("error", (err, _req, res) => {
            if ((err as any).code === "ECONNREFUSED") {
              if (res && "writeHead" in res && !res.headersSent) {
                res.writeHead(503, { "Content-Type": "application/json" });
                res.end(JSON.stringify({ error: "Backend server starting..." }));
              }
              return;
            }
            console.error("[vite proxy error]", err.message);
          });
        },
      },
    },
    headers: {
      "Permissions-Policy": "local-network-access=*, private-network-access=*, local-network=*, loopback-network=*",
    },
  },
  resolve: {
    alias: {
      "@build": path.resolve(__dirname, "../build"),
    },
  },
})
