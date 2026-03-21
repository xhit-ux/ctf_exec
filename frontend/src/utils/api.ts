const rawApiBaseUrl = (import.meta.env.VITE_API_BASE_URL || '').trim()
const rawServerHost = (import.meta.env.SERVER_HOST || import.meta.env.VITE_SERVER_HOST || '').trim()
const rawServerPort = (import.meta.env.SERVER_PORT || import.meta.env.VITE_SERVER_PORT || '').trim()
const rawApiProtocol = (import.meta.env.VITE_API_PROTOCOL || 'http').trim()

function resolveApiBaseUrl(): string {
  if (rawApiBaseUrl) {
    return rawApiBaseUrl.replace(/\/+$/, '')
  }

  if (!rawServerHost && !rawServerPort) {
    return ''
  }

  if (/^https?:\/\//i.test(rawServerHost)) {
    return rawServerHost.replace(/\/+$/, '')
  }

  const host = rawServerHost || 'localhost'
  const protocol = /^https?$/i.test(rawApiProtocol) ? rawApiProtocol : 'http'
  const portSegment = rawServerPort ? `:${rawServerPort}` : ''

  return `${protocol}://${host}${portSegment}`.replace(/\/+$/, '')
}

export const API_BASE_URL = resolveApiBaseUrl()

export function apiUrl(path: string): string {
  if (/^https?:\/\//i.test(path)) return path

  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  return API_BASE_URL ? `${API_BASE_URL}${normalizedPath}` : normalizedPath
}

export function staticUrl(path?: string): string {
  if (!path) return ''
  if (/^https?:\/\//i.test(path)) return path

  const normalizedPath = path
    .replace(/\\/g, '/')
    .replace(/^\.\//, '')
    .replace(/^\.\.\//, '')
    .replace(/^\/+/, '')

  return apiUrl(`/${normalizedPath}`)
}
