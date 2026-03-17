const rawApiBaseUrl = (import.meta.env.VITE_API_BASE_URL || '').trim()

export const API_BASE_URL = rawApiBaseUrl.replace(/\/+$/, '')

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
