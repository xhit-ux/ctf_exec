import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig(({ mode }) => {
  const envDir = '../'
  const env = loadEnv(mode, envDir, '')

  const serverHost = (env.SERVER_HOST || env.VITE_SERVER_HOST || '127.0.0.1').trim()
  const serverPort = (env.SERVER_PORT || env.VITE_SERVER_PORT || '8080').trim()
  const serverProtocol = (env.VITE_API_PROTOCOL || 'http').trim().toLowerCase() === 'https' ? 'https' : 'http'
  const backendTarget = `${serverProtocol}://${serverHost}:${serverPort}`

  return {
    envDir,
    envPrefix: ['VITE_'],
    plugins: [react()],
    server: {
      host: '0.0.0.0',
      port: 5173,
      proxy: {
        '/api': {
          target: backendTarget,
          changeOrigin: true,
          ws: true,
          configure: (proxy) => {
            proxy.on('proxyReq', (proxyReq, req) => {
              const remote = req.socket?.remoteAddress
              if (!remote) return
              const existing = req.headers['x-forwarded-for']
              if (typeof existing === 'string' && existing.trim() !== '') {
                proxyReq.setHeader('X-Forwarded-For', `${existing}, ${remote}`)
              } else if (Array.isArray(existing) && existing[0]) {
                proxyReq.setHeader('X-Forwarded-For', `${existing[0]}, ${remote}`)
              } else {
                proxyReq.setHeader('X-Forwarded-For', remote)
              }
            })
          },
        },
        '/static': {
          target: backendTarget,
          changeOrigin: true,
        },
      },
    },
  }
})
