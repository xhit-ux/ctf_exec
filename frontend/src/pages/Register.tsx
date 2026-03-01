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
        <div className="max-w-md mx-auto mt-10 p-6 bg-gray-800 rounded-lg shadow-xl">
            <h2 className="text-2xl font-bold mb-6 text-center text-green-400">注册</h2>

            {error && (
                <div className="mb-4 p-3 bg-red-600 text-white rounded">
                    {error}
                </div>
            )}

            <form onSubmit={handleRegister} className="space-y-4">
                <div>
                    <label className="block text-gray-300 mb-1">用户名</label>
                    <input
                        type="text"
                        className="w-full p-2 rounded bg-gray-700 text-white border border-gray-600 focus:border-green-500 outline-none"
                        value={username}
                        onChange={(e) => setUsername(e.target.value)}
                        disabled={loading}
                        placeholder="3-32 个字符"
                        required
                    />
                </div>
                <div>
                    <label className="block text-gray-300 mb-1">邮箱</label>
                    <input
                        type="email"
                        className="w-full p-2 rounded bg-gray-700 text-white border border-gray-600 focus:border-green-500 outline-none"
                        value={email}
                        onChange={(e) => setEmail(e.target.value)}
                        disabled={loading}
                        required
                    />
                </div>
                <div>
                    <label className="block text-gray-300 mb-1">密码</label>
                    <input
                        type="password"
                        className="w-full p-2 rounded bg-gray-700 text-white border border-gray-600 focus:border-green-500 outline-none"
                        value={password}
                        onChange={(e) => setPassword(e.target.value)}
                        disabled={loading}
                        placeholder="至少 6 个字符"
                        required
                    />
                </div>
                <div>
                    <label className="block text-gray-300 mb-1">确认密码</label>
                    <input
                        type="password"
                        className="w-full p-2 rounded bg-gray-700 text-white border border-gray-600 focus:border-green-500 outline-none"
                        value={confirmPassword}
                        onChange={(e) => setConfirmPassword(e.target.value)}
                        disabled={loading}
                        required
                    />
                </div>
                <button
                    type="submit"
                    className="w-full py-2 bg-green-600 hover:bg-green-700 rounded text-white font-semibold transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                    disabled={loading}
                >
                    {loading ? '注册中...' : '注册'}
                </button>
            </form>

            <div className="mt-4 text-center text-gray-400">
                已有账号？{' '}
                <Link to="/login" className="text-green-400 hover:text-green-300">
                    登录
                </Link>
            </div>
        </div>
    )
}
