import React, { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'

export default function Register() {
    const [username, setUsername] = useState('')
    const [email, setEmail] = useState('')
    const [password, setPassword] = useState('')
    const [confirmPassword, setConfirmPassword] = useState('')
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState('')
    const navigate = useNavigate()
    const { login } = useAuth()

    const handleRegister = async (e: React.FormEvent) => {
        e.preventDefault()
        setError('')

        // 本地验证
        if (!username || !email || !password) {
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
            const response = await fetch('http://localhost:8080/api/auth/register', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ username, email, password }),
            })

            if (!response.ok) {
                const data = await response.json()
                throw new Error(data.message || '注册失败')
            }

            const data = await response.json()

            // 使用 context 的 login 方法更新全局状态
            login(data.token, data.user)

            // 重定向到首页
            navigate('/')
        } catch (err) {
            setError(err instanceof Error ? err.message : '注册失败，请重试')
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
                    <input
                        type="email"
                        className="w-full px-4 py-3 rounded-lg bg-gray-50 border border-gray-200 text-gray-900 focus:bg-white focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200 outline-none transition-all"
                        value={email}
                        onChange={(e) => setEmail(e.target.value)}
                        disabled={loading}
                        placeholder="用于接收通知和找回密码"
                        required
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
