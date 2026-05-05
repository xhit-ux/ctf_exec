import React, { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { apiUrl } from '../utils/api'
import { apiFetch, setAuthTokens } from '../utils/authClient'
import { fromUnknownError, readSafeErrorMessage } from '../utils/error'
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

const usernamePattern = /^[A-Za-z0-9_-]+$/
const usernamePatternHint = '用户名仅允许包含字母、数字、短线(-)和下划线(_)'

export default function Register() {
  const [username, setUsername] = useState('')
  const [email, setEmail] = useState('')
  const [emailCode, setEmailCode] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [rememberMe, setRememberMe] = useState(true)
  const [loading, setLoading] = useState(false)
  const [sendingCode, setSendingCode] = useState(false)
  const [codeCooldown, setCodeCooldown] = useState(0)
  const [error, setError] = useState('')
  const [hints, setHints] = useState<RegistrationHints | null>(null)
  const [hintsLoaded, setHintsLoaded] = useState(false)
  const [hintsLoadError, setHintsLoadError] = useState('')

  const [sendPanelOpen, setSendPanelOpen] = useState(false)
  const [sendResetSignal, setSendResetSignal] = useState(0)
  const [captchaErrorSend, setCaptchaErrorSend] = useState('')
  const sendCodeInFlightRef = useRef(false)

  const navigate = useNavigate()
  const { login } = useAuth()
  const turnstileSiteKey = (import.meta.env.VITE_TURNSTILE_SITE_KEY || '').trim()

  const registrationEnabled = hints?.registration_enabled !== false
  const emailVerificationRequired = hints?.email_verification_required === true
  const botProtectionEnabled = hints?.bot_protection_enabled === true
  /** 仅「邮箱验证码 + 人机」路径依赖前端站点密钥；未启用邮箱验证时注册不经过 Turnstile，不因缺少密钥锁死整页 */
  const sendCodeNeedsTurnstileUI = emailVerificationRequired && botProtectionEnabled
  const turnstileMisconfigured = sendCodeNeedsTurnstileUI && !turnstileSiteKey

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
        })
        if (!response.ok) {
          const msg = await readSafeErrorMessage(response, '无法加载注册策略，请稍后重试')
          if (!cancelled) setHintsLoadError(msg)
          return
        }
        const data = (await response.json()) as RegistrationHints
        if (!cancelled) setHints(data)
      } catch (err) {
        if (!cancelled) setHintsLoadError(fromUnknownError(err, '无法加载注册策略，请稍后重试'))
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

      const trimmedEmail = email.trim()

      if (!trimmedEmail) {
        setError('请先输入邮箱')
        return
      }

      if (!registrationEnabled) {
        setError('当前未开放注册')
        return
      }

      if (turnstileMisconfigured) {
        setError('服务端已启用人机验证，但未配置前端 VITE_TURNSTILE_SITE_KEY')
        return
      }

      const botToken = botProtectionEnabled ? (token ?? '').trim() : ''
      if (botProtectionEnabled && !botToken) {
        setCaptchaErrorSend('请先完成上方人机验证')
        return
      }

      if (sendCodeInFlightRef.current) {
        return
      }
      sendCodeInFlightRef.current = true
      setSendingCode(true)

      try {
        const response = await apiFetch(
          apiUrl('/api/auth/send-register-code'),
          {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              email: trimmedEmail,
              ...(botProtectionEnabled ? { bot_token: botToken } : {}),
            }),
          },
          { disableAuth: true, disableRetry401: true }
        )

        if (!response.ok) {
          setError(await readSafeErrorMessage(response, '验证码发送失败，请稍后重试'))
          if (botProtectionEnabled) {
            setSendResetSignal((s) => s + 1)
          }
          return
        }

        setCodeCooldown(60)
        setSendPanelOpen(false)
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
    [
      email,
      registrationEnabled,
      turnstileMisconfigured,
      botProtectionEnabled,
    ]
  )

  const handleSendCode = useCallback(async () => {
    setError('')
    setCaptchaErrorSend('')

    const trimmedEmail = email.trim()

    if (!hintsLoaded) {
      setError('注册策略加载中，请稍候')
      return
    }
    if (!registrationEnabled) {
      setError('当前未开放注册')
      return
    }
    if (!emailVerificationRequired) {
      setError('当前站点未启用邮箱验证码，请直接填写密码并提交注册')
      return
    }
    if (!trimmedEmail) {
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

    await sendCodeRequest()
  }, [
    hintsLoaded,
    registrationEnabled,
    emailVerificationRequired,
    email,
    turnstileMisconfigured,
    botProtectionEnabled,
    sendCodeRequest,
  ])

  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    const trimmedUsername = username.trim()
    const trimmedEmail = email.trim()

    if (!hintsLoaded) {
      setError('注册策略加载中，请稍候')
      return
    }
    if (!registrationEnabled) {
      setError('当前未开放注册')
      return
    }

    if (!trimmedUsername || !trimmedEmail || !password) {
      setError('请填写用户名、邮箱和密码')
      return
    }

    if (!usernamePattern.test(trimmedUsername)) {
      setError(usernamePatternHint)
      return
    }

    if (emailVerificationRequired && emailCode.length !== 6) {
      setError('请输入 6 位邮箱验证码')
      return
    }

    if (password !== confirmPassword) {
      setError('两次输入的密码不一致')
      return
    }

    if (password.length < 6) {
      setError('密码长度至少 6 个字符')
      return
    }

    setLoading(true)

    try {
      const response = await apiFetch(
        apiUrl('/api/auth/register'),
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            username: trimmedUsername,
            email: trimmedEmail,
            password,
            email_code: emailVerificationRequired ? emailCode : '',
            remember_me: rememberMe,
          }),
        },
        { disableAuth: true, disableRetry401: true }
      )

      if (!response.ok) {
        setError(await readSafeErrorMessage(response, '注册失败，请稍后重试'))
        return
      }

      const data = await response.json()

      setAuthTokens(data.access_expires_in_secs)
      login(data.user)

      navigate('/')
    } catch (err) {
      setError(fromUnknownError(err, '注册失败，请稍后重试'))
    } finally {
      setLoading(false)
    }
  }

  const disabledForm = loading || !hintsLoaded || !registrationEnabled || !!hintsLoadError || turnstileMisconfigured

  return (
    <AuthShell
      heading="创建账号，进入实战训练"
      title="创建新账号"
      description="注册后可参与题目练习、比赛与团队协作，并持续追踪你的解题成长。"
      switchText="已有账号？"
      switchLabel="立即登录"
      switchTo="/login"
    >
      {!hintsLoaded && (
        <div className="mb-4 soft-card p-3 text-sm text-slate-600 border-slate-200">正在加载注册策略…</div>
      )}
      {hintsLoadError && (
        <div className="mb-4 soft-card p-3 text-sm text-red-700 border-red-200">{hintsLoadError}</div>
      )}
      {hintsLoaded && hints && !registrationEnabled && (
        <div className="mb-4 soft-card p-3 text-sm text-amber-900 bg-amber-50 border-amber-200">
          管理员已关闭新用户注册。如需开通请联系站点管理员。
        </div>
      )}
      {turnstileMisconfigured && registrationEnabled && (
        <div className="mb-4 soft-card p-3 text-sm text-amber-800 bg-amber-50 border-amber-200">
          当前启用了邮箱验证码与人机验证，但前端未配置 <code className="text-xs">VITE_TURNSTILE_SITE_KEY</code>
          ，无法发送验证码。请在构建/部署环境中配置该变量。
        </div>
      )}
      {error && <div className="mb-4 soft-card p-3 text-sm text-red-700 border-red-200">{error}</div>}

      <form onSubmit={handleRegister} className="space-y-4">
        <div className="space-y-2">
          <label className="text-sm font-medium text-slate-700">用户名</label>
          <input
            type="text"
            className="field-input"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            disabled={disabledForm}
            placeholder="3-32 位：字母、数字、下划线"
            required
          />
          <p className="text-xs text-slate-500">{usernamePatternHint}</p>
        </div>
        <div className="space-y-2">
          <label className="text-sm font-medium text-slate-700">邮箱</label>
          <div className="flex gap-2">
            <input
              type="email"
              className="field-input"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              disabled={disabledForm || sendingCode}
              placeholder="用于账号联系与通知"
              required
            />
            {emailVerificationRequired && (
              <button
                type="button"
                onClick={handleSendCode}
                disabled={disabledForm || sendingCode || codeCooldown > 0 || !registrationEnabled}
                className="btn-secondary whitespace-nowrap"
              >
                {sendingCode ? '发送中' : codeCooldown > 0 ? `${codeCooldown}s` : '发送验证码'}
              </button>
            )}
          </div>
          {!emailVerificationRequired && hintsLoaded && registrationEnabled && (
            <p className="text-xs text-slate-500">当前站点未强制邮箱验证码，填写邮箱与密码后直接注册即可。</p>
          )}
        </div>

        {emailVerificationRequired && sendPanelOpen && botProtectionEnabled && turnstileSiteKey && (
          <div className="soft-card p-3 space-y-2">
            <div className="text-xs text-slate-600 font-medium">发送验证码前 · 人机验证</div>
            <p className="text-xs text-slate-500">
              验证通过后将自动发送邮件；令牌一次性有效。冷却结束后如需重新发信，请再次点击「发送验证码」并完成验证。
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
                console.error('[turnstile] send-code widget error', {
                  reason,
                  host: window.location.hostname,
                  href: window.location.href,
                })
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

        {emailVerificationRequired && (
          <div className="space-y-2">
            <label className="text-sm font-medium text-slate-700">邮箱验证码</label>
            <input
              type="text"
              className="field-input"
              value={emailCode}
              onChange={(e) => setEmailCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
              disabled={disabledForm}
              placeholder="输入 6 位验证码"
              required={emailVerificationRequired}
            />
          </div>
        )}

        <div className="space-y-2">
          <label className="text-sm font-medium text-slate-700">密码</label>
          <input
            type="password"
            className="field-input"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            disabled={disabledForm}
            placeholder="至少 6 个字符"
            required
          />
        </div>
        <div className="space-y-2">
          <label className="text-sm font-medium text-slate-700">确认密码</label>
          <input
            type="password"
            className="field-input"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            disabled={disabledForm}
            placeholder="请再次输入密码"
            required
          />
        </div>
        <label className="flex items-center gap-2 text-sm text-slate-600 select-none">
          <input
            type="checkbox"
            className="h-4 w-4 rounded border-slate-300 text-slate-700 focus:ring-slate-500"
            checked={rememberMe}
            onChange={(e) => setRememberMe(e.target.checked)}
            disabled={disabledForm}
          />
          <span>记住我（30 天内自动保持登录）</span>
        </label>
        <button type="submit" className="btn-primary w-full !py-3" disabled={disabledForm || loading}>
          {loading ? '注册中...' : '注册'}
        </button>
      </form>
    </AuthShell>
  )
}
