export type DocumentTheme = 'light' | 'dark'

const FAVICON: Record<DocumentTheme, string> = {
  light: '/favicon-light.svg',
  dark: '/favicon-dark.svg',
}

const THEME_COLOR: Record<DocumentTheme, string> = {
  light: '#f3f4f6',
  dark: '#111827',
}

export function resolveInitialTheme(): DocumentTheme {
  try {
    const saved = localStorage.getItem('theme')
    if (saved === 'light' || saved === 'dark') return saved
  } catch {
    /* ignore private mode / denied */
  }
  if (typeof window !== 'undefined' && window.matchMedia?.('(prefers-color-scheme: dark)').matches) {
    return 'dark'
  }
  return 'light'
}

/** Syncs <html class="dark">, favicon, and theme-color meta with app theme. */
export function applyDocumentTheme(theme: DocumentTheme): void {
  document.documentElement.classList.toggle('dark', theme === 'dark')

  let link = document.getElementById('app-favicon') as HTMLLinkElement | null
  if (!link) {
    link = document.createElement('link')
    link.id = 'app-favicon'
    link.rel = 'icon'
    link.type = 'image/svg+xml'
    document.head.appendChild(link)
  }
  link.href = FAVICON[theme]

  let meta = document.querySelector('meta[name="theme-color"]')
  if (!meta) {
    meta = document.createElement('meta')
    meta.setAttribute('name', 'theme-color')
    document.head.appendChild(meta)
  }
  meta.setAttribute('content', THEME_COLOR[theme])
}
