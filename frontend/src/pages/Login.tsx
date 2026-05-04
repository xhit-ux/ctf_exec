import React, { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { apiUrl } from '../utils/api'
import { apiFetch, setAuthTokens } from '../utils/authClient'
import { fromUnknownError, readSafeErrorMessage } from '../utils/error'
import AuthShell from '../components/auth/AuthShell'

export default function Login() {
    const [username, setUsername] = useState('')
    const [password, setPassword] = useState('')
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState('')
    const navigate = useNavigate()
    const { login } = useAuth()

    const handleLogin = async (e: React.FormEvent) => {
        e.preventDefault()
        setError('')
        setLoading(true)

        try {
            const response = await apiFetch(apiUrl('/api/auth/login'), {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ username, password }),
            }, {
                disableAuth: true,
                disableRetry401: true,
            })

            if (!response.ok) {
                setError(await readSafeErrorMessage(response, '登录失败，请重试'))
                return
            }

            const data = await response.json()

            setAuthTokens(data.access_expires_in_secs)
            // 使用 context 的 login 方法更新全局状态
            login(data.user)

            // 重定向到首页
            navigate('/')
        } catch (err) {
            setError(fromUnknownError(err, '登录失败，请重试'))
        } finally {
            setLoading(false)
        }
    }

    return (
        <AuthShell
            heading="进入你的训练空间"
            title="欢迎回来"
            description="登录后即可查看题目进度、历史提交和队伍状态，继续你的实战节奏。"
            switchText="还没有账号？"
            switchLabel="立即注册"
            switchTo="/register"
        >
            {error && <div className="mb-4 soft-card p-3 text-sm text-red-700 border-red-200">{error}</div>}

            <form onSubmit={handleLogin} className="space-y-4">
                <div className="space-y-2">
                    <label className="text-sm font-medium text-slate-700">用户名</label>
                    <input
                        type="text"
                        className="field-input"
                        value={username}
                        onChange={(e) => setUsername(e.target.value)}
                        disabled={loading}
                        required
                        placeholder="请输入用户名"
                    />
                </div>
                <div className="space-y-2">
                    <div className="flex items-center justify-between gap-2">
                        <label className="text-sm font-medium text-slate-700">密码</label>
                        <Link
                            to="/forgot-password"
                            className="text-sm text-slate-600 hover:text-slate-900 underline decoration-slate-300 hover:decoration-slate-600"
                        >
                            忘记密码？
                        </Link>
                    </div>
                    <input
                        type="password"
                        className="field-input"
                        value={password}
                        onChange={(e) => setPassword(e.target.value)}
                        disabled={loading}
                        required
                        placeholder="请输入密码"
                    />
                </div>
                <button
                    type="submit"
                    className="btn-primary w-full !py-3"
                    disabled={loading}
                >
                    {loading ? '登录中...' : '登录'}
                </button>
            </form>
        </AuthShell>
    )
}
