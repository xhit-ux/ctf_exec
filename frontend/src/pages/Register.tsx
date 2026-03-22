import React, { useEffect, useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { apiUrl } from '../utils/api'
import { fromUnknownError, readSafeErrorMessage } from '../utils/error'
import TurnstileWidget from '../components/auth/TurnstileWidget'

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
            const response = await fetch(apiUrl('/api/auth/send-register-code'), {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ email, bot_token: tokenToUse }),
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
            const response = await fetch(apiUrl('/api/auth/register'), {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ username, email, password, email_code: emailCode }),
            })

            if (!response.ok) {
                setError(await readSafeErrorMessage(response, '注册失败，请稍后重试'))
                return
            }

            const data = await response.json()

            // 使用 context 的 login 方法更新全局状态
            login(data.token, data.user)

            // 重定向到首页
            navigate('/')
        } catch (err) {
            setError(fromUnknownError(err, '注册失败，请稍后重试'))
        } finally {
            setLoading(false)
        }
    }

    return (
        <div className="max-w-5xl mx-auto mt-16 bg-white border border-gray-100 rounded-3xl shadow-xl overflow-hidden flex flex-col md:flex-row">
            {/* 左侧装饰区域 */}
            <div className="hidden md:flex md:w-1/2 bg-gradient-to-br from-pink-300/70 to-rose-200/70 p-12 flex-col justify-between text-gray-900 relative overflow-hidden backdrop-blur-sm">
                <div className="relative z-10">
                    <h1 className="text-4xl font-extrabold mb-4 text-rose-950">注册账号<br/>成为我们的用户</h1>
                    <p className="text-rose-900/80 text-lg leading-relaxed font-medium">
                        从入门到精通，CTF 学习平台为您提供各种难度阶梯的真题实战。注册账号，开启属于您的极客之路。
                    </p>
                </div>
                <div className="relative z-10 text-sm text-rose-800/60 font-medium">
                    &copy; 2026 CTF Platform. All rights reserved.
                </div>
                
                {/* 装饰用光效和几何图形 */}
                <div className="absolute -top-24 -right-24 w-96 h-96 bg-white opacity-40 rounded-full blur-3xl"></div>
                <div className="absolute -bottom-24 -left-24 w-72 h-72 bg-pink-400 opacity-20 rounded-full blur-2xl"></div>
            </div>

            {/* 右侧表单区域 */}
            <div className="w-full md:w-1/2 p-10 md:p-14 flex flex-col justify-center bg-white">
                <div className="mb-10">
                    <h2 className="text-3xl font-extrabold text-gray-900 tracking-tight">创建新账号</h2>
                    <p className="text-gray-500 mt-2">加入 CTF 练习平台，开始您的学习之旅</p>
                </div>

                {error && (
                    <div className="mb-6 p-4 bg-red-50 border border-red-200 text-red-600 rounded-lg text-sm flex items-center gap-2">
                    <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    {error}
                </div>
            )}

            <form onSubmit={handleRegister} className="space-y-5">
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">用户名</label>
                    <input
                        type="text"
                        className="w-full px-4 py-3 rounded-lg bg-gray-50 border border-gray-200 text-gray-900 focus:bg-white focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200 outline-none transition-all"
                        value={username}
                        onChange={(e) => setUsername(e.target.value)}
                        disabled={loading}
                        placeholder="3-32 个字符"
                        required
                    />
                </div>
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">邮箱</label>
                    <div className="flex gap-2">
                        <input
                            type="email"
                            className="flex-1 px-4 py-3 rounded-lg bg-gray-50 border border-gray-200 text-gray-900 focus:bg-white focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200 outline-none transition-all"
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
                            className="px-4 py-3 rounded-lg border border-indigo-200 text-indigo-600 font-semibold bg-indigo-50 hover:bg-indigo-100 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
                        >
                            {sendingCode ? '发送中' : codeCooldown > 0 ? `${codeCooldown}s` : '发送验证码'}
                        </button>
                    </div>
                </div>
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">邮箱验证码</label>
                    <input
                        type="text"
                        className="w-full px-4 py-3 rounded-lg bg-gray-50 border border-gray-200 text-gray-900 focus:bg-white focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200 outline-none transition-all"
                        value={emailCode}
                        onChange={(e) => setEmailCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                        disabled={loading}
                        placeholder="输入 6 位验证码"
                        required
                    />
                </div>
                {showTurnstile && (
                    <div className="rounded-lg border border-gray-200 bg-gray-50 p-3">
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
                                Turnstile 未配置，请设置前端环境变量 `VITE_TURNSTILE_SITE_KEY`
                            </div>
                        )}
                        {captchaError && <div className="text-xs text-red-600 mt-2">{captchaError}</div>}
                    </div>
                )}
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">密码</label>
                    <input
                        type="password"
                        className="w-full px-4 py-3 rounded-lg bg-gray-50 border border-gray-200 text-gray-900 focus:bg-white focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200 outline-none transition-all"
                        value={password}
                        onChange={(e) => setPassword(e.target.value)}
                        disabled={loading}
                        placeholder="至少 6 个字符"
                        required
                    />
                </div>
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">确认密码</label>
                    <input
                        type="password"
                        className="w-full px-4 py-3 rounded-lg bg-gray-50 border border-gray-200 text-gray-900 focus:bg-white focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200 outline-none transition-all"
                        value={confirmPassword}
                        onChange={(e) => setConfirmPassword(e.target.value)}
                        disabled={loading}
                        placeholder="请再次输入密码"
                        required
                    />
                </div>
                <button
                    type="submit"
                    className="w-full py-3.5 bg-indigo-600 hover:bg-indigo-700 rounded-lg text-white font-bold text-lg shadow-md hover:shadow-lg transition-all disabled:opacity-50 disabled:cursor-not-allowed mt-2"
                    disabled={loading}
                >
                    {loading ? '注册中...' : '注册'}
                </button>
            </form>

            <div className="mt-8 pt-6 border-t border-gray-100 text-center text-gray-500">
                已有账号？{' '}
                <Link to="/login" className="font-semibold text-indigo-600 hover:text-indigo-500 transition-colors">
                    立即登录
                </Link>
            </div>
            </div>
        </div>
    )
}
