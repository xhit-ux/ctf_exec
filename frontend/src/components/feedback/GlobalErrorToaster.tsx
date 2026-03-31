import React, { PropsWithChildren, useEffect, useMemo, useRef, useState } from 'react'
import { APP_HTTP_ERROR_EVENT, GlobalHttpErrorPayload } from '../../utils/globalErrorBus'

type ToastLevel = 'warn' | 'error'

type ToastItem = {
  id: number
  message: string
  level: ToastLevel
}

const MAX_TOASTS = 4
const TOAST_DURATION_MS = 3800
const DUPLICATE_WINDOW_MS = 1200

function levelByStatus(status: number): ToastLevel {
  if (status >= 500 || status === 0) return 'error'
  return 'warn'
}

export default function GlobalErrorToaster({ children }: PropsWithChildren) {
  const [toasts, setToasts] = useState<ToastItem[]>([])
  const idRef = useRef(1)
  const recentRef = useRef<Map<string, number>>(new Map())

  const pushToast = (message: string, level: ToastLevel) => {
    const key = `${level}:${message}`
    const now = Date.now()
    const recentAt = recentRef.current.get(key)
    if (recentAt && now-recentAt < DUPLICATE_WINDOW_MS) {
      return
    }

    recentRef.current.set(key, now)
    const id = idRef.current++
    setToasts((prev) => [{ id, message, level }, ...prev].slice(0, MAX_TOASTS))

    window.setTimeout(() => {
      setToasts((prev) => prev.filter((item) => item.id !== id))
    }, TOAST_DURATION_MS)
  }

  useEffect(() => {
    const onHttpError = (event: Event) => {
      const customEvent = event as CustomEvent<GlobalHttpErrorPayload>
      const payload = customEvent.detail
      if (!payload) return

      // 401 的登录态失效由 auth 流程统一处理（刷新或跳转登录），这里不重复弹。
      if (payload.status === 401) return

      const message = (payload.message || '').trim()
      if (!message) return

      pushToast(message, levelByStatus(payload.status))
    }

    window.addEventListener(APP_HTTP_ERROR_EVENT, onHttpError as EventListener)
    return () => {
      window.removeEventListener(APP_HTTP_ERROR_EVENT, onHttpError as EventListener)
    }
  }, [])

  const toastNodes = useMemo(() => toasts.map((toast) => (
    <div
      key={toast.id}
      className={`global-toast global-toast--${toast.level}`}
      role="status"
      aria-live="polite"
    >
      {toast.message}
    </div>
  )), [toasts])

  return (
    <>
      {children}
      <div className="global-toast-stack" aria-live="polite" aria-atomic="false">
        {toastNodes}
      </div>
    </>
  )
}
