import React, { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'

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
            const response = await fetch('http://localhost:8080/api/auth/login', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ username, password }),
            })

            if (!response.ok) {
                const data = await response.json()
                throw new Error(data.message || '登陆失败')
            }

            const data = await response.json()

            // 使用 context 的 login 方法更新全局状态
            login(data.token, data.user)

            // 重定向到首页
            navigate('/')
        } catch (err) {
            setError(err instanceof Error ? err.message : '登陆失败，请重试')
        } finally {
            setLoading(false)
        }
    }

    return (
        <div className="max-w-md mx-auto mt-10 p-6 bg-gray-800 rounded-lg shadow-xl">
            <h2 className="text-2xl font-bold mb-6 text-center text-blue-400">登录</h2>

            {error && (
                <div className="mb-4 p-3 bg-red-600 text-white rounded">
                    {error}
                </div>
            )}

            <form onSubmit={handleLogin} className="space-y-4">
                <div>
                    <label className="block text-gray-300 mb-1">用户名</label>
                    <input
                        type="text"
                        className="w-full p-2 rounded bg-gray-700 text-white border border-gray-600 focus:border-blue-500 outline-none"
                        value={username}
                        onChange={(e) => setUsername(e.target.value)}
                        disabled={loading}
                        required
                    />
                </div>
                <div>
                    <label className="block text-gray-300 mb-1">密码</label>
                    <input
                        type="password"
                        className="w-full p-2 rounded bg-gray-700 text-white border border-gray-600 focus:border-blue-500 outline-none"
                        value={password}
                        onChange={(e) => setPassword(e.target.value)}
                        disabled={loading}
                        required
                    />
                </div>
                <button
                    type="submit"
                    className="w-full py-2 bg-blue-600 hover:bg-blue-700 rounded text-white font-semibold transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                    disabled={loading}
                >
                    {loading ? '登录中...' : '登录'}
                </button>
            </form>

            <div className="mt-4 text-center text-gray-400">
                还没有账号？{' '}
                <Link to="/register" className="text-blue-400 hover:text-blue-300">
                    注册
                </Link>
            </div>
        </div>
    )
}
