import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

const configDirectory = path.dirname(fileURLToPath(import.meta.url))
const projectDirectory = path.resolve(configDirectory, '..', '..')

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, projectDirectory, '')
  return {
    envDir: projectDirectory,
    plugins: [react()],
    base: env.VITE_ASSET_BASE_URL || '/',
    server: {
      port: 5173,
      proxy: {
        '/api': {
          target: 'http://localhost:8080',
          changeOrigin: true,
          ws: true,
          headers: env.API_KEY ? { 'X-API-Key': env.API_KEY } : undefined,
        },
        '/admin': {
          target: 'http://localhost:8081',
          changeOrigin: true,
          rewrite: (path) => path.replace(/^\/admin/, ''),
        },
      },
    },
  }
})
