import React, { useEffect, useRef } from 'react'

type TurnstileWidgetProps = {
  siteKey: string
  resetSignal?: number
  onVerify: (token: string) => void
  onExpire?: () => void
  onError?: (reason?: string) => void
}

declare global {
  interface Window {
    turnstile?: {
      render: (
        container: HTMLElement,
        options: {
          sitekey: string
          callback: (token: string) => void
          'expired-callback'?: () => void
          'error-callback'?: (errorCode?: string) => void
          theme?: 'light' | 'dark' | 'auto'
        }
      ) => string
      remove: (widgetId: string) => void
    }
  }
}

const SCRIPT_ID = 'cf-turnstile-script'
const DEFAULT_SCRIPT_SRC = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit'
const SCRIPT_SRC = (import.meta.env.VITE_TURNSTILE_SCRIPT_URL || DEFAULT_SCRIPT_SRC).trim()

function loadTurnstileScript(): Promise<void> {
  const existing = document.getElementById(SCRIPT_ID) as HTMLScriptElement | null
  if (existing) {
    if (window.turnstile) return Promise.resolve()
    return new Promise((resolve, reject) => {
      existing.addEventListener('load', () => resolve(), { once: true })
      existing.addEventListener('error', () => reject(new Error('Turnstile script load failed')), { once: true })
    })
  }

  return new Promise((resolve, reject) => {
    const script = document.createElement('script')
    script.id = SCRIPT_ID
    script.src = SCRIPT_SRC
    script.async = true
    script.defer = true
    script.onload = () => resolve()
    script.onerror = () => reject(new Error('turnstile_script_load_failed'))
    document.head.appendChild(script)
  })
}

/**
 * Cloudflare Turnstile：用 ref 持有回调，避免因父组件重渲染反复 remove/render 导致验证状态闪烁或丢失。
 */
export default function TurnstileWidget({
  siteKey,
  resetSignal,
  onVerify,
  onExpire,
  onError,
}: TurnstileWidgetProps) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const widgetIdRef = useRef<string | null>(null)
  const onVerifyRef = useRef(onVerify)
  const onExpireRef = useRef(onExpire)
  const onErrorRef = useRef(onError)

  onVerifyRef.current = onVerify
  onExpireRef.current = onExpire
  onErrorRef.current = onError

  useEffect(() => {
    let disposed = false

    async function renderWidget() {
      if (!siteKey || !containerRef.current) return
      await loadTurnstileScript()

      if (disposed || !containerRef.current || !window.turnstile) return

      if (widgetIdRef.current) {
        window.turnstile.remove(widgetIdRef.current)
        widgetIdRef.current = null
      }

      widgetIdRef.current = window.turnstile.render(containerRef.current, {
        sitekey: siteKey,
        callback: (token: string) => onVerifyRef.current?.(token),
        'expired-callback': () => onExpireRef.current?.(),
        'error-callback': (errorCode?: string) =>
          onErrorRef.current?.(errorCode || 'turnstile_render_error'),
        theme: 'light',
      })
    }

    renderWidget().catch(() => {
      onErrorRef.current?.('turnstile_script_load_failed')
    })

    return () => {
      disposed = true
      if (widgetIdRef.current && window.turnstile) {
        window.turnstile.remove(widgetIdRef.current)
        widgetIdRef.current = null
      }
    }
  }, [siteKey, resetSignal])

  return <div ref={containerRef} />
}
