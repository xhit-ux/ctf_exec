import type { CSSProperties } from 'react'

export type TabAccent = {
  bg: string
  border: string
  text: string
  glow: string
}

export const TAB_ACCENT_PALETTE: TabAccent[] = [
  { bg: 'rgba(14, 165, 233, 0.2)', border: '#0284c7', text: '#000000', glow: 'rgba(14, 165, 233, 0.32)' },
  { bg: 'rgba(20, 184, 166, 0.2)', border: '#0f766e', text: '#000000', glow: 'rgba(20, 184, 166, 0.32)' },
  { bg: 'rgba(34, 197, 94, 0.2)', border: '#15803d', text: '#000000', glow: 'rgba(34, 197, 94, 0.30)' },
  { bg: 'rgba(245, 158, 11, 0.2)', border: '#b45309', text: '#000000', glow: 'rgba(245, 158, 11, 0.30)' },
  { bg: 'rgba(249, 115, 22, 0.2)', border: '#c2410c', text: '#000000', glow: 'rgba(249, 115, 22, 0.30)' },
  { bg: 'rgba(244, 63, 94, 0.2)', border: '#be123c', text: '#000000', glow: 'rgba(244, 63, 94, 0.30)' },
]

export function randomTabAccent(current?: TabAccent): TabAccent {
  if (TAB_ACCENT_PALETTE.length < 2) {
    return TAB_ACCENT_PALETTE[0]
  }

  let next = TAB_ACCENT_PALETTE[Math.floor(Math.random() * TAB_ACCENT_PALETTE.length)]
  while (current && next.bg === current.bg) {
    next = TAB_ACCENT_PALETTE[Math.floor(Math.random() * TAB_ACCENT_PALETTE.length)]
  }
  return next
}

export function tabAccentStyle(accent: TabAccent): CSSProperties {
  return {
    '--tab-active-bg': accent.bg,
    '--tab-active-border': accent.border,
    '--tab-active-text': accent.text,
    '--tab-active-glow': accent.glow,
  } as CSSProperties
}
