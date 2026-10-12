import path from 'path'
import {defineConfig} from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

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
        target: "http://127.0.0.1:4545",
        changeOrigin: true,
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
