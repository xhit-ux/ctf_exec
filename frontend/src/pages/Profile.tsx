import React, { useState, useEffect } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import AvatarUpload from '../components/AvatarUpload'

export default function Profile() {
  const { username: urlUsername } = useParams<{ username?: string }>()
  const { user: currentUser, token, refreshUser } = useAuth()

  const [targetUser, setTargetUser] = useState<any>(null)
  const [loading, setLoading] = useState(true)
  const [showUpload, setShowUpload] = useState(false)
  const [isEditing, setIsEditing] = useState(false)

  // 表单状态
  const [formData, setFormData] = useState({
    display_name: '',
    email: '',
    bio: ''
  })
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState({ type: '', text: '' })

  // 判断是否是当前用户自己的主页
  const isOwnProfile = !urlUsername || (currentUser && currentUser.username === urlUsername)

  // 获取目标用户信息
  useEffect(() => {
    const fetchUser = async () => {
      setLoading(true)
      if (isOwnProfile) {
        setTargetUser(currentUser)
        setLoading(false)
      } else {
        try {
          const response = await fetch(`http://localhost:8080/api/users/public?username=${urlUsername}`)
          if (response.ok) {
            const data = await response.json()
            setTargetUser(data)
          } else {
            setTargetUser(null)
          }
        } catch (err) {
          console.error('Failed to fetch public profile:', err)
        } finally {
          setLoading(false)
        }
      }
    }
    fetchUser()
  }, [urlUsername, currentUser, isOwnProfile])

  // 初始化表单数据
  useEffect(() => {
    if (currentUser && isOwnProfile) {
      setFormData({
        display_name: currentUser.display_name || '',
        email: currentUser.email || '',
        bio: currentUser.bio || ''
      })
    }
  }, [currentUser, isOwnProfile])

  const handleUploadSuccess = async (newUrl: string) => {
    if (refreshUser) {
      await refreshUser()
    }
    setShowUpload(false)
  }

  const handleSaveProfile = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    setMessage({ type: '', text: '' })

    try {
      const response = await fetch('http://localhost:8080/api/users/profile', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify(formData)
      })

      if (response.ok) {
        if (refreshUser) {
          await refreshUser()
        }
        setIsEditing(false)
        setMessage({ type: 'success', text: '个人资料已更新' })
      } else {
        const errorData = await response.json()
        setMessage({ type: 'error', text: errorData.error || '更新失败' })
      }
    } catch (err) {
      setMessage({ type: 'error', text: '网络错误，请稍后再试' })
    } finally {
      setSaving(false)
    }
  }

  if (loading) return <div className="text-center text-gray-500 mt-10">加载中...</div>

  if (!targetUser) {
    return (
      <div className="text-center text-gray-500 mt-10">
        <p className="mb-4">未找到该用户</p>
        <Link to="/" className="text-blue-500 hover:underline">返回首页</Link>
      </div>
    )
  }

  const avatarUrl = targetUser.avatar_url?.startsWith('static/')
    ? `http://localhost:8080/${targetUser.avatar_url}`
    : targetUser.avatar_url

  return (
    <div className="max-w-4xl mx-auto p-4">
      <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
        {/* 背景装饰 */}
        <div className="h-32 bg-gradient-to-r from-blue-500 to-indigo-600"></div>

        <div className="px-8 pb-8">
          <div className="relative flex justify-between items-end -mt-12 mb-6">
            <div className="relative">
              <div className="w-32 h-32 rounded-2xl border-4 border-white overflow-hidden bg-gray-100 shadow-md">
                {targetUser.avatar_url ? (
                  <img
                    src={avatarUrl}
                    alt={targetUser.username}
                    className="w-full h-full object-cover"
                  />
                ) : (
                  <div className="w-full h-full flex items-center justify-center text-4xl font-bold text-blue-500">
                    {targetUser.username[0].toUpperCase()}
                  </div>
                )}
              </div>
              {isOwnProfile && (
                <button
                  onClick={() => setShowUpload(!showUpload)}
                  className="absolute bottom-2 right-2 p-2 bg-white rounded-lg shadow-lg text-gray-600 hover:text-blue-500 transition-colors border border-gray-100"
                  title="更换头像"
                >
                  <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z" />
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M15 13a3 3 0 11-6 0 3 3 0 016 0z" />
                  </svg>
                </button>
              )}
            </div>

            <div className="flex gap-3">
              {isOwnProfile && (
                !isEditing ? (
                  <button
                    onClick={() => setIsEditing(true)}
                    className="px-4 py-2 bg-gray-50 text-gray-700 rounded-lg hover:bg-gray-100 transition-colors font-medium border border-gray-200"
                  >
                    编辑资料
                  </button>
                ) : (
                  <div className="flex gap-2">
                    <button
                      onClick={() => setIsEditing(false)}
                      className="px-4 py-2 text-gray-600 hover:text-gray-800 font-medium"
                    >
                      取消
                    </button>
                    <button
                      onClick={handleSaveProfile}
                      disabled={saving}
                      className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors font-medium shadow-sm disabled:opacity-50"
                    >
                      {saving ? '保存中...' : '提交保存'}
                    </button>
                  </div>
                )
              )}
            </div>
          </div>

          {showUpload && isOwnProfile && (
            <div className="mb-8 p-6 bg-blue-50 rounded-xl border border-blue-100">
              <AvatarUpload currentAvatarUrl={targetUser.avatar_url} onUploadSuccess={handleUploadSuccess} />
            </div>
          )}

          {message.text && (
            <div className={`mb-6 p-4 rounded-lg flex items-center gap-3 ${message.type === 'success' ? 'bg-green-50 text-green-700 border border-green-100' : 'bg-red-50 text-red-700 border border-red-100'
              }`}>
              <svg className="w-5 h-5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                {message.type === 'success' ? (
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                ) : (
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                )}
              </svg>
              {message.text}
            </div>
          )}

          <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
            <div className="md:col-span-2">
              {isEditing && isOwnProfile ? (
                <form onSubmit={handleSaveProfile} className="space-y-6">
                  <div>
                    <label className="block text-sm font-semibold text-gray-700 mb-2">显示名称</label>
                    <input
                      type="text"
                      value={formData.display_name}
                      onChange={(e) => setFormData({ ...formData, display_name: e.target.value })}
                      className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none transition-all"
                      placeholder="你的昵称"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-semibold text-gray-700 mb-2">电子邮箱</label>
                    <input
                      type="email"
                      value={formData.email}
                      onChange={(e) => setFormData({ ...formData, email: e.target.value })}
                      className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none transition-all"
                      placeholder="email@example.com"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-semibold text-gray-700 mb-2">个人简介</label>
                    <textarea
                      rows={4}
                      value={formData.bio}
                      onChange={(e) => setFormData({ ...formData, bio: e.target.value })}
                      className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none transition-all resize-none"
                      placeholder="介绍一下你自己..."
                    />
                  </div>
                </form>
              ) : (
                <div className="space-y-8">
                  <div>
                    <h1 className="text-3xl font-extrabold text-gray-900 mb-2">{targetUser.display_name || targetUser.username}</h1>
                    <p className="text-gray-500 font-medium">@{targetUser.username}</p>
                  </div>

                  <div>
                    <h3 className="text-sm font-bold text-gray-400 uppercase tracking-wider mb-3">个人简介</h3>
                    <p className="text-gray-700 leading-relaxed text-lg">
                      {targetUser.bio || '这个人很懒，什么都没留下...'}
                    </p>
                  </div>

                  <div className="grid grid-cols-2 gap-6 pt-4">
                    <div className="p-4 bg-gray-50 rounded-xl border border-gray-100 flex flex-col justify-center">
                      <div className="flex justify-between items-center mb-1">
                        <p className="text-sm text-gray-500">电子邮箱</p>
                        {targetUser.email ? (
                          <span className={`text-[10px] px-1.5 py-0.5 rounded font-bold uppercase ${targetUser.is_verified ? 'bg-green-100 text-green-700' : 'bg-yellow-100 text-yellow-700'
                            }`}>
                            {targetUser.is_verified ? '已验证' : '未验证'}
                          </span>
                        ) : (
                          <span className="text-[10px] px-1.5 py-0.5 rounded font-bold uppercase bg-gray-100 text-gray-500">
                            已隐藏
                          </span>
                        )}
                      </div>
                      <p className="font-semibold text-gray-900 text-sm truncate" title={targetUser.email || '私密信息'}>
                        {targetUser.email || '********@***.***'}
                      </p>
                    </div>
                    <div className="p-4 bg-gray-50 rounded-xl border border-gray-100">
                      <p className="text-sm text-gray-500 mb-1">角色</p>
                      <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-bold uppercase tracking-wider ${targetUser.role === 'admin' ? 'bg-purple-100 text-purple-700' : 'bg-blue-100 text-blue-700'
                        }`}>
                        {targetUser.role}
                      </span>
                    </div>
                  </div>
                </div>
              )}
            </div>

            <div className="space-y-6">
              <div className="bg-gradient-to-br from-gray-900 to-gray-800 rounded-2xl p-6 text-white shadow-lg">
                <h3 className="text-gray-400 text-xs font-bold uppercase tracking-wider mb-6">成就统计</h3>
                <div className="space-y-6">
                  <div>
                    <p className="text-3xl font-black text-white mb-1">{targetUser.score}</p>
                    <p className="text-gray-400 text-xs font-medium">总积分</p>
                  </div>
                  <div className="pt-4 border-t border-gray-700">
                    <p className="text-2xl font-bold text-white mb-1">{targetUser.solved_count}</p>
                    <p className="text-gray-400 text-xs font-medium">解题数</p>
                  </div>
                </div>
              </div>

              <div className="p-4 bg-white rounded-xl border border-gray-100 flex items-center gap-3">
                <div className="p-2 bg-blue-50 rounded-lg text-blue-600">
                  <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
                  </svg>
                </div>
                <div>
                  <p className="text-xs text-gray-500 font-medium">加入时间</p>
                  <p className="text-sm font-bold text-gray-900">
                    {new Date(targetUser.created_at).toLocaleDateString()}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
