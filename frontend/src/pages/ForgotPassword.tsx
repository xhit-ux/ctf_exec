import React, { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { apiUrl } from '../utils/api'
import { apiFetch } from '../utils/authClient'
import { fromUnknownError, readSafeErrorMessage } from '../utils/error'
import { PASSWORD_POLICY_HINT, validatePasswordStrength } from '../utils/passwordPolicy'
import TurnstileWidget from '../components/auth/TurnstileWidget'
import AuthShell from '../components/auth/AuthShell'

type RegistrationHints = {
  registration_enabled: boolean
  email_verification_required: boolean
  bot_protection_enabled: boolean
}

function resolveTurnstileErrorMessage(reason: string): string {
  const host = window.location.hostname || 'unknown-host'
  const normalized = reason.trim().toLowerCase()

  if (!normalized) {
    return '人机验证失败，请刷新页面后重试'
  }

  if (normalized === 'turnstile_script_load_failed') {
    return '人机验证脚本加载失败，请检查网络是否可访问 challenges.cloudflare.com'
  }

  if (normalized === '110200' || normalized === '110201') {
    return `当前访问域名/IP(${host})未加入 Turnstile 允许列表，请在 Cloudflare Turnstile 控制台添加后重试`
  }

  return `人机验证失败(${reason})，请刷新页面后重试`
}

type Phase = 'request_code' | 'new_password' | 'done'

export default function ForgotPassword() {
  const navigate = useNavigate()
  const [phase, setPhase] = useState<Phase>('request_code')
  const [email, setEmail] = useState('')
  const [emailCode, setEmailCode] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [sendingCode, setSendingCode] = useState(false)
  const [codeCooldown, setCodeCooldown] = useState(0)
  const [error, setError] = useState('')
  const [sendHint, setSendHint] = useState('')

  const [hints, setHints] = useState<RegistrationHints | null>(null)
  const [hintsLoaded, setHintsLoaded] = useState(false)
  const [hintsLoadError, setHintsLoadError] = useState('')

  const [sendPanelOpen, setSendPanelOpen] = useState(false)
  const [sendResetSignal, setSendResetSignal] = useState(0)
  const [captchaErrorSend, setCaptchaErrorSend] = useState('')
  /** 防止 Turnstile 回调 / StrictMode 触发重复 POST，导致 timeout-or-duplicate */
  const sendCodeInFlightRef = useRef(false)

  const turnstileSiteKey = (import.meta.env.VITE_TURNSTILE_SITE_KEY || '').trim()
  const botProtectionEnabled = hints?.bot_protection_enabled === true
  const sendNeedsTurnstile = botProtectionEnabled
  const turnstileMisconfigured = sendNeedsTurnstile && !turnstileSiteKey

  useEffect(() => {
    if (codeCooldown <= 0) return
    const timer = window.setInterval(() => {
      setCodeCooldown((prev) => (prev <= 1 ? 0 : prev - 1))
    }, 1000)
    return () => window.clearInterval(timer)
  }, [codeCooldown])

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const response = await apiFetch(apiUrl('/api/auth/registration-hints'), { method: 'GET' }, {
          disableAuth: true,
          disableRetry401: true,
          disableGlobalError: true,
        })
        if (!response.ok) {
          const msg = await readSafeErrorMessage(response, '无法加载页面策略，请稍后重试')
          if (!cancelled) setHintsLoadError(msg)
          return
        }
        const data = (await response.json()) as RegistrationHints
        if (!cancelled) setHints(data)
      } catch (err) {
        if (!cancelled) setHintsLoadError(fromUnknownError(err, '无法加载页面策略，请稍后重试'))
      } finally {
        if (!cancelled) setHintsLoaded(true)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  const sendCodeRequest = useCallback(
    async (token?: string) => {
      setError('')
      setCaptchaErrorSend('')
      setSendHint('')

      if (!email.trim()) {
        setError('请先输入邮箱')
        return
      }

      if (turnstileMisconfigured) {
        setError('服务端已启用人机验证，但未配置前端 VITE_TURNSTILE_SITE_KEY')
        return
      }

      const botToken = botProtectionEnabled ? (token ?? '').trim() : ''
      if (botProtectionEnabled && !botToken) {
        setCaptchaErrorSend('请先完成人机验证')
        return
      }

      if (sendCodeInFlightRef.current) {
        return
      }
      sendCodeInFlightRef.current = true
      setSendingCode(true)

      try {
        const response = await apiFetch(
          apiUrl('/api/auth/send-password-reset-code'),
          {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              email,
              ...(botProtectionEnabled ? { bot_token: botToken } : {}),
            }),
          },
          { disableAuth: true, disableRetry401: true, disableGlobalError: true }
        )

        if (!response.ok) {
          setError(await readSafeErrorMessage(response, '验证码发送失败，请稍后重试'))
          if (botProtectionEnabled) {
            setSendResetSignal((s) => s + 1)
          }
          return
        }

        const data = (await response.json()) as { message?: string }
        setSendHint(data.message || '若该邮箱已绑定账号，你将收到一封包含验证码的邮件，请查收')
        setCodeCooldown(60)
        setSendPanelOpen(false)
        setPhase('new_password')
        setEmailCode('')
        setNewPassword('')
        setConfirmPassword('')
      } catch (err) {
        setError(fromUnknownError(err, '验证码发送失败，请稍后重试'))
        if (botProtectionEnabled) {
          setSendResetSignal((s) => s + 1)
        }
      } finally {
        setSendingCode(false)
        sendCodeInFlightRef.current = false
      }
    },
    [email, turnstileMisconfigured, botProtectionEnabled]
  )

  const handleSendCode = useCallback(() => {
    setError('')
    setCaptchaErrorSend('')
    setSendHint('')

    if (!hintsLoaded) {
      setError('页面加载中，请稍候')
      return
    }
    if (!email.trim()) {
      setError('请先输入邮箱')
      return
    }
    if (turnstileMisconfigured) {
      setError('服务端已启用人机验证，但未配置前端 VITE_TURNSTILE_SITE_KEY')
      return
    }

    if (botProtectionEnabled) {
      setSendPanelOpen(true)
      setSendResetSignal((s) => s + 1)
      setCaptchaErrorSend('请完成人机验证后自动发送验证码')
      return
    }

    void sendCodeRequest()
  }, [hintsLoaded, email, turnstileMisconfigured, botProtectionEnabled, sendCodeRequest])

  const goBackToEmail = () => {
    setPhase('request_code')
    setEmailCode('')
    setNewPassword('')
    setConfirmPassword('')
    setError('')
    setSendHint('')
    setSendPanelOpen(false)
    setCaptchaErrorSend('')
  }

  const handleResetPassword = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    if (emailCode.length !== 6) {
      setError('请输入 6 位邮箱验证码')
      return
    }
    if (newPassword !== confirmPassword) {
      setError('两次输入的密码不一致')
      return
    }
    const passwordError = validatePasswordStrength(newPassword)
    if (passwordError) {
      setError(passwordError)
      return
    }

    setLoading(true)
    try {
      const response = await apiFetch(
        apiUrl('/api/auth/reset-password'),
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            email,
            email_code: emailCode,
            new_password: newPassword,
          }),
        },
        { disableAuth: true, disableRetry401: true, disableGlobalError: true }
      )

      if (!response.ok) {
        setError(await readSafeErrorMessage(response, '重置失败，请稍后重试'))
        return
      }

      setPhase('done')
    } catch (err) {
      setError(fromUnknownError(err, '重置失败，请稍后重试'))
    } finally {
      setLoading(false)
    }
  }

  const disabledRequest =
    loading || !hintsLoaded || !!hintsLoadError || turnstileMisconfigured || sendingCode || codeCooldown > 0

  return (
    <AuthShell
      heading="安全找回你的账号"
      title="重置密码"
      description="通过绑定邮箱接收验证码，设置新密码。完成后请使用新密码登录。"
      switchText="想起密码了？"
      switchLabel="返回登录"
      switchTo="/login"
    >
      {!hintsLoaded && (
        <div className="mb-4 soft-card p-3 text-sm text-slate-600 border-slate-200">正在加载…</div>
      )}
      {hintsLoadError && (
        <div className="mb-4 soft-card p-3 text-sm text-red-700 border-red-200">{hintsLoadError}</div>
      )}
      {turnstileMisconfigured && hintsLoaded && (
        <div className="mb-4 soft-card p-3 text-sm text-amber-800 bg-amber-50 border-amber-200">
          服务端已启用人机验证，但前端未配置 <code className="text-xs">VITE_TURNSTILE_SITE_KEY</code>
          ，无法发送验证码。
        </div>
      )}
      {error && <div className="mb-4 soft-card p-3 text-sm text-red-700 border-red-200">{error}</div>}
      {sendHint && phase === 'new_password' && (
        <div className="mb-4 soft-card p-3 text-sm text-slate-700 border-slate-200 bg-slate-50">{sendHint}</div>
      )}

      {phase === 'done' ? (
        <div className="space-y-4">
          <div className="soft-card p-4 text-center border-emerald-200 bg-emerald-50 text-emerald-900">
            <p className="font-medium">重置成功</p>
            <p className="text-sm mt-2 text-emerald-800">请使用新密码登录。其它设备上的登录状态已失效。</p>
          </div>
          <button type="button" className="btn-primary w-full !py-3" onClick={() => navigate('/login')}>
            返回登录
          </button>
        </div>
      ) : phase === 'request_code' ? (
        <div className="space-y-4">
          <div className="space-y-2">
            <label className="text-sm font-medium text-slate-700">注册邮箱</label>
            <input
              type="email"
              className="field-input"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              disabled={disabledRequest || loading}
              placeholder="请输入账号绑定的邮箱"
              required
            />
          </div>

          {sendPanelOpen && botProtectionEnabled && turnstileSiteKey && (
            <div className="soft-card p-3 space-y-2">
              <div className="text-xs text-slate-600 font-medium">发送验证码前 · 人机验证</div>
              <p className="text-xs text-slate-500">
                验证通过后自动发信；令牌一次性有效。冷却结束后再次发送需重新验证。
              </p>
              <TurnstileWidget
                siteKey={turnstileSiteKey}
                resetSignal={sendResetSignal}
                onVerify={(token) => {
                  setCaptchaErrorSend('')
                  void sendCodeRequest(token)
                }}
                onExpire={() => {
                  setCaptchaErrorSend('人机验证已过期，请再次点击「发送验证码」')
                }}
                onError={(reason) => {
                  const message = resolveTurnstileErrorMessage(reason || '')
                  setCaptchaErrorSend(message)
                  console.error('[turnstile] forgot-password send-code', { reason })
                }}
              />
              {captchaErrorSend && <div className="text-xs text-red-600">{captchaErrorSend}</div>}
              <button
                type="button"
                className="text-xs text-slate-500 hover:text-slate-700 underline"
                onClick={() => {
                  setSendPanelOpen(false)
                  setCaptchaErrorSend('')
                }}
              >
                取消
              </button>
            </div>
          )}

          <button
            type="button"
            className="btn-secondary w-full !py-3"
            onClick={handleSendCode}
            disabled={disabledRequest || loading}
          >
            {sendingCode ? '发送中…' : codeCooldown > 0 ? `${codeCooldown}s 后可重发` : '发送验证码'}
          </button>
        </div>
      ) : (
        <form onSubmit={handleResetPassword} className="space-y-4">
          <div className="space-y-2">
            <label className="text-sm font-medium text-slate-700">邮箱</label>
            <input type="email" className="field-input bg-slate-50 text-slate-600" value={email} readOnly disabled />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium text-slate-700">邮箱验证码</label>
            <input
              type="text"
              className="field-input"
              value={emailCode}
              onChange={(e) => setEmailCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
              disabled={loading}
              placeholder="6 位数字"
              required
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium text-slate-700">新密码</label>
            <input
              type="password"
              className="field-input"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              disabled={loading}
              placeholder={PASSWORD_POLICY_HINT}
              required
            />
            <p className="text-xs text-slate-500">{PASSWORD_POLICY_HINT}</p>
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium text-slate-700">确认新密码</label>
            <input
              type="password"
              className="field-input"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              disabled={loading}
              placeholder="再次输入新密码"
              required
            />
          </div>
          <button type="submit" className="btn-primary w-full !py-3" disabled={loading || !hintsLoaded}>
            {loading ? '提交中…' : '确认重置密码'}
          </button>
          <button type="button" className="btn-secondary w-full !py-3" disabled={loading} onClick={goBackToEmail}>
            使用其它邮箱
          </button>
        </form>
      )}
    </AuthShell>
  )
}
