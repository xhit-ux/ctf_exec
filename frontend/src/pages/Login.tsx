import React, { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { apiUrl } from '../utils/api'
import { fromUnknownError, readSafeErrorMessage } from '../utils/error'

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
            const response = await fetch(apiUrl('/api/auth/login'), {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ username, password }),
            })

            if (!response.ok) {
                setError(await readSafeErrorMessage(response, '登录失败，请重试'))
                return
            }

            const data = await response.json()

            // 使用 context 的 login 方法更新全局状态
            login(data.token, data.user)

            // 重定向到首页
            navigate('/')
        } catch (err) {
            setError(fromUnknownError(err, '登录失败，请重试'))
        } finally {
            setLoading(false)
        }
    }

    return (
        <div className="max-w-5xl mx-auto mt-16 bg-white border border-gray-100 rounded-3xl shadow-xl overflow-hidden flex flex-col md:flex-row">
            {/* 左侧装饰区域 */}
            <div className="hidden md:flex md:w-1/2 bg-gradient-to-br from-pink-300/70 to-rose-200/70 p-12 flex-col justify-between text-gray-900 relative overflow-hidden backdrop-blur-sm">
                <div className="relative z-10">
                    <h1 className="text-4xl font-extrabold mb-4 text-rose-950">开启您的<br/>CTF 学习之旅</h1>
                    <p className="text-rose-900/80 text-lg leading-relaxed font-medium">
                        加入我们的练习平台，挑战丰富的题目，提升安全技能。在真实的对抗环境中磨炼自我，追踪进度并与全球极客一较高下。
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
                    <h2 className="text-3xl font-extrabold text-gray-900 tracking-tight">欢迎回来</h2>
                    <p className="text-gray-500 mt-2">使用您的账号登录到 CTF 练习平台</p>
                </div>

                {error && (
                    <div className="mb-6 p-4 bg-red-50 border border-red-200 text-red-600 rounded-lg text-sm flex items-center gap-2">
                    <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    {error}
                </div>
            )}

            <form onSubmit={handleLogin} className="space-y-6">
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">用户名</label>
                    <input
                        type="text"
                        className="w-full px-4 py-3 rounded-lg bg-gray-50 border border-gray-200 text-gray-900 focus:bg-white focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200 outline-none transition-all"
                        value={username}
                        onChange={(e) => setUsername(e.target.value)}
                        disabled={loading}
                        required
                        placeholder="请输入用户名"
                    />
                </div>
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">密码</label>
                    <input
                        type="password"
                        className="w-full px-4 py-3 rounded-lg bg-gray-50 border border-gray-200 text-gray-900 focus:bg-white focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200 outline-none transition-all"
                        value={password}
                        onChange={(e) => setPassword(e.target.value)}
                        disabled={loading}
                        required
                        placeholder="请输入密码"
                    />
                </div>
                <button
                    type="submit"
                    className="w-full py-3.5 bg-indigo-600 hover:bg-indigo-700 rounded-lg text-white font-bold text-lg shadow-md hover:shadow-lg transition-all disabled:opacity-50 disabled:cursor-not-allowed mt-2"
                    disabled={loading}
                >
                    {loading ? '登录中...' : '登录'}
                </button>
            </form>

            <div className="mt-8 pt-6 border-t border-gray-100 text-center text-gray-500">
                还没有账号？{' '}
                <Link to="/register" className="font-semibold text-indigo-600 hover:text-indigo-500 transition-colors">
                    立即注册
                </Link>
            </div>
            </div>
        </div>
    )
}
