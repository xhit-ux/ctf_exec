import React, { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { apiUrl } from '../utils/api'
import { apiFetch } from '../utils/authClient'
import { fromUnknownError, readSafeErrorMessage } from '../utils/error'
import TurnstileWidget from '../components/auth/TurnstileWidget'
import AuthShell from '../components/auth/AuthShell'

function resolveTurnstileErrorMessage(reason: string): string {
    const host = window.location.hostname || 'unknown-host'
    const normalized = reason.trim().toLowerCase()

    if (!normalized) {
        return '人机验证失败，请刷新页面后重试'
    }

    if (normalized === 'turnstile_script_load_failed') {
        return '人机验证脚本加载失败，请检查网络是否可访问 challenges.cloudflare.com'
    }

    // 常见错误码：110200/110201 通常表示当前域名(含局域网IP)不在 Turnstile 允许列表。
    if (normalized === '110200' || normalized === '110201') {
        return `当前访问域名/IP(${host})未加入 Turnstile 允许列表，请在 Cloudflare Turnstile 控制台添加后重试`
    }

    return `人机验证失败(${reason})，请刷新页面后重试`
}

export default function Register() {
    const [username, setUsername] = useState('')
    const [email, setEmail] = useState('')
    const [emailCode, setEmailCode] = useState('')
    const [password, setPassword] = useState('')
    const [confirmPassword, setConfirmPassword] = useState('')
    const [loading, setLoading] = useState(false)
    const [sendingCode, setSendingCode] = useState(false)
    const [codeCooldown, setCodeCooldown] = useState(0)
    const [error, setError] = useState('')
    const [botToken, setBotToken] = useState('')
    const [showTurnstile, setShowTurnstile] = useState(false)
    const [pendingSendAfterVerify, setPendingSendAfterVerify] = useState(false)
    const [turnstileResetSignal, setTurnstileResetSignal] = useState(0)
    const [captchaError, setCaptchaError] = useState('')

    const navigate = useNavigate()
    const { login } = useAuth()
    const turnstileSiteKey = (import.meta.env.VITE_TURNSTILE_SITE_KEY || '').trim()

    useEffect(() => {
        if (codeCooldown <= 0) return
        const timer = window.setInterval(() => {
            setCodeCooldown((prev) => (prev <= 1 ? 0 : prev - 1))
        }, 1000)

        return () => window.clearInterval(timer)
    }, [codeCooldown])

    const sendCodeRequest = async (token?: string) => {
        const tokenToUse = token || botToken

        setError('')
        setCaptchaError('')

        if (!email) {
            setError('请先输入邮箱')
            return
        }

        if (!turnstileSiteKey) {
            setError('未配置 Turnstile 站点密钥，暂无法发送验证码')
            return
        }

        setSendingCode(true)

        try {
            const response = await apiFetch(apiUrl('/api/auth/send-register-code'), {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ email, bot_token: tokenToUse }),
            }, {
                disableAuth: true,
                disableRetry401: true,
            })

            if (!response.ok) {
                setError(await readSafeErrorMessage(response, '验证码发送失败，请稍后重试'))
                return
            }

            setCodeCooldown(60)
            setShowTurnstile(false)
        } catch (err) {
            setError(fromUnknownError(err, '验证码发送失败，请稍后重试'))
        } finally {
            setSendingCode(false)
            setPendingSendAfterVerify(false)
            setBotToken('')
        }
    }

    const handleSendCode = async () => {
        setError('')
        setCaptchaError('')

        if (!email) {
            setError('请先输入邮箱')
            return
        }

        if (!turnstileSiteKey) {
            setError('未配置 Turnstile 站点密钥，暂无法发送验证码')
            return
        }

        // 每次点击发送验证码都要求重新验证一次。
        setBotToken('')
        setPendingSendAfterVerify(true)
        setShowTurnstile(true)
        setTurnstileResetSignal((prev) => prev + 1)
        setCaptchaError('请先完成验证，再自动发送验证码')
    }

    const handleRegister = async (e: React.FormEvent) => {
        e.preventDefault()
        setError('')

        // 本地验证
        if (!username || !email || !password || !emailCode) {
            setError('请填写所有字段')
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
            const response = await apiFetch(apiUrl('/api/auth/register'), {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ username, email, password, email_code: emailCode }),
            }, {
                disableAuth: true,
                disableRetry401: true,
            })

            if (!response.ok) {
                setError(await readSafeErrorMessage(response, '注册失败，请稍后重试'))
                return
            }

            const data = await response.json()

            // 使用 context 的 login 方法更新全局状态
            login(data.token, data.refresh_token, data.user)

            // 重定向到首页
            navigate('/')
        } catch (err) {
            setError(fromUnknownError(err, '注册失败，请稍后重试'))
        } finally {
            setLoading(false)
        }
    }

    return (
        <AuthShell
            heading="创建账号，进入实战训练"
            title="创建新账号"
            description="注册后可参与题目练习、比赛与团队协作，并持续追踪你的解题成长。"
            switchText="已有账号？"
            switchLabel="立即登录"
            switchTo="/login"
        >
            {error && <div className="mb-4 soft-card p-3 text-sm text-red-700 border-red-200">{error}</div>}

            <form onSubmit={handleRegister} className="space-y-4">
                <div className="space-y-2">
                    <label className="text-sm font-medium text-slate-700">用户名</label>
                    <input
                        type="text"
                        className="field-input"
                        value={username}
                        onChange={(e) => setUsername(e.target.value)}
                        disabled={loading}
                        placeholder="3-32 个字符"
                        required
                    />
                </div>
                <div className="space-y-2">
                    <label className="text-sm font-medium text-slate-700">邮箱</label>
                    <div className="flex gap-2">
                        <input
                            type="email"
                            className="field-input"
                            value={email}
                            onChange={(e) => setEmail(e.target.value)}
                            disabled={loading || sendingCode}
                            placeholder="用于接收通知和找回密码"
                            required
                        />
                        <button
                            type="button"
                            onClick={handleSendCode}
                            disabled={loading || sendingCode || codeCooldown > 0}
                            className="btn-secondary whitespace-nowrap"
                        >
                            {sendingCode ? '发送中' : codeCooldown > 0 ? `${codeCooldown}s` : '发送验证码'}
                        </button>
                    </div>
                </div>
                <div className="space-y-2">
                    <label className="text-sm font-medium text-slate-700">邮箱验证码</label>
                    <input
                        type="text"
                        className="field-input"
                        value={emailCode}
                        onChange={(e) => setEmailCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                        disabled={loading}
                        placeholder="输入 6 位验证码"
                        required
                    />
                </div>
                {showTurnstile && (
                    <div className="soft-card p-3">
                        <div className="text-xs text-gray-500 mb-2">完成人机验证后会自动发送验证码</div>
                        {turnstileSiteKey ? (
                        <TurnstileWidget
                            siteKey={turnstileSiteKey}
                            resetSignal={turnstileResetSignal}
                            onVerify={(token) => {
                                setBotToken(token)
                                setCaptchaError('')
                                if (pendingSendAfterVerify) {
                                    void sendCodeRequest(token)
                                }
                            }}
                            onExpire={() => setBotToken('')}
                            onError={(reason) => {
                                setBotToken('')
                                const message = resolveTurnstileErrorMessage(reason || '')
                                setCaptchaError(message)
                                console.error('[turnstile] verification widget error', {
                                    reason,
                                    host: window.location.hostname,
                                    href: window.location.href,
                                })
                            }}
                        />
                        ) : (
                            <div className="text-sm text-amber-700 bg-amber-50 border border-amber-200 rounded-lg px-3 py-2">
                                Turnstile 未配置，请设置前端环境变量 VITE_TURNSTILE_SITE_KEY
                            </div>
                        )}
                        {captchaError && <div className="text-xs text-red-600 mt-2">{captchaError}</div>}
                    </div>
                )}
                <div className="space-y-2">
                    <label className="text-sm font-medium text-slate-700">密码</label>
                    <input
                        type="password"
                        className="field-input"
                        value={password}
                        onChange={(e) => setPassword(e.target.value)}
                        disabled={loading}
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
                        disabled={loading}
                        placeholder="请再次输入密码"
                        required
                    />
                </div>
                <button
                    type="submit"
                    className="btn-primary w-full !py-3"
                    disabled={loading}
                >
                    {loading ? '注册中...' : '注册'}
                </button>
            </form>
        </AuthShell>
    )
}
