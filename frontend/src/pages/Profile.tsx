import React, { useState, useEffect } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import AvatarUpload from '../components/upload/AvatarUpload'
import SubmissionHeatmap from '../components/profile/SubmissionHeatmap'
import { apiUrl, staticUrl } from '../utils/api'
import { fromUnknownError, readSafeErrorMessage } from '../utils/error'


function getLevelInfo(score: number = 0) {
  if (score < 1000) {
    return { title: '新手', color: 'text-gray-500' }
  }
  if (score < 3000) {
    return { title: '入门', color: 'text-green-500' }
  }
  if (score < 4500) {
    return { title: '进阶', color: 'text-cyan-500' }
  }
  if (score < 5500) {
    return { title: '熟练', color: 'text-blue-500' }
  }
  if (score < 6000) {
    return { title: '专家', color: 'text-purple-500' }
  }
  return { title: '大师', color: 'text-red-500' }
}

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
    bio: '',
    email_public: false
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
          const headers: HeadersInit = {}
          if (token) {
            headers['Authorization'] = `Bearer ${token}`
          }
          const response = await fetch(apiUrl(`/api/users/public?username=${urlUsername}`), {
            headers
          })
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
  }, [urlUsername, currentUser, isOwnProfile, token])

  // 初始化表单数据
  useEffect(() => {
    if (currentUser && isOwnProfile) {
      // 解析用户偏好设置
      let emailPublic = false;
      if (currentUser.preferences) {
        try {
          const prefs = JSON.parse(currentUser.preferences);
          // 处理整数1或字符串"true"或布尔值true的情况
          emailPublic = !!prefs.email_public;
        } catch (e) {
          // 忽略解析错误
        }
      }
      
      setFormData({
        display_name: currentUser.display_name || '',
        email: currentUser.email || '',
        bio: currentUser.bio || '',
        email_public: emailPublic
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
      const response = await fetch(apiUrl('/api/users/profile'), {
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
        setMessage({ type: 'error', text: await readSafeErrorMessage(response, '更新失败，请稍后重试') })
      }
    } catch (err) {
      setMessage({ type: 'error', text: fromUnknownError(err, '网络错误，请稍后再试') })
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

  const avatarUrl = staticUrl(targetUser.avatar_url)

  
  const score = targetUser.score ?? 0
  const level = getLevelInfo(score)

  return (
    <div className="w-full max-w-6xl mx-auto p-4 lg:p-8 min-w-0">
      <div className="bg-white rounded-2xl shadow-sm border border-gray-100 overflow-hidden mb-8">
        {/* 背景装饰 */}
        <div className="h-32 bg-gradient-to-r from-pink-300/70 to-rose-200/70"></div>

        <div className="p-6 md:px-8 md:pb-8">
          <div className="relative flex flex-col sm:flex-row sm:justify-between items-center sm:items-end -mt-16 sm:-mt-12 mb-6 gap-4 sm:gap-0">
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
                  <div className="pt-4 border-t border-gray-100">
                    <div className="flex items-center justify-between">
                      <div>
                        <label className="block text-sm font-semibold text-gray-700 mb-1">隐私设置</label>
                        <p className="text-sm text-gray-500">对外展示</p>
                      </div>
                      <div className="flex items-center">
                        <button
                          type="button"
                          onClick={() => setFormData({ ...formData, email_public: !formData.email_public })}
                          className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 ${formData.email_public ? 'bg-blue-600' : 'bg-gray-200'}`}
                        >
                          <span className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${formData.email_public ? 'translate-x-6' : 'translate-x-1'}`} />
                        </button>
                        <span className="ml-3 text-sm font-medium text-gray-700">
                          {formData.email_public ? '公开' : '隐藏'}
                        </span>
                      </div>
                    </div>
                    <p className="mt-2 text-sm text-gray-500">
                      {formData.email_public 
                        ? '其他登录用户可以看到你的邮箱地址' 
                        : '你的邮箱地址仅对你自己可见，其他用户无法查看'}
                    </p>
                  </div>
                </form>
              ) : (
                <div className="space-y-8 min-w-0">
                  <div className={`text-xs ${level.color} min-w-0`}>
                      <h1 className="text-3xl font-extrabold mb-2 break-words truncate">{targetUser.display_name || targetUser.username}</h1>
                      <h2 className="text-1xl font-extrabold mb-2">
                        <div>
                          等级 · {level.title}
                        </div>
                      </h2>
                    {/* <p className="text-gray-500 font-medium">@{targetUser.username}</p> */}
                  </div>

                  <div>
                    <h3 className="text-sm font-bold text-gray-400 uppercase tracking-wider mb-3">个人简介</h3>
                    <p className="text-gray-700 leading-relaxed text-lg">
                      {targetUser.bio || '这个人很懒，什么都没留下...'}
                    </p>
                  </div>

                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-6 pt-4 w-full min-w-0">
                    <div className="p-4 bg-gray-50 rounded-xl border border-gray-100 flex flex-col justify-center min-w-0">
                      <div className="flex justify-between items-center mb-1 gap-2">
                        <p className="text-sm text-gray-500">电子邮箱</p>
                        {isOwnProfile && targetUser.email ? (
                          <span className={`text-[10px] px-1.5 py-0.5 rounded font-bold uppercase ${targetUser.is_verified ? 'bg-green-100 text-green-700' : 'bg-yellow-100 text-yellow-700'
                            }`}>
                            {targetUser.is_verified ? '已验证' : '未验证'}
                          </span>
                        ) : targetUser.email ? (
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
                      <p className="font-semibold text-gray-900 text-sm truncate w-full" title={targetUser.email || '私密信息'}>
                        {targetUser.email || 'ta不想让你看ta的邮箱！'}
                      </p>
                    </div>
                    <div className="p-4 bg-gray-50 rounded-xl border border-gray-100 flex flex-col justify-center min-w-0">
                      <p className="text-sm text-gray-500 mb-1">角色</p>
                      <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-bold uppercase tracking-wider ${(targetUser.role === 'root' || targetUser.role === 'site_admin') ? 'bg-purple-100 text-purple-700' : 'bg-blue-100 text-blue-700'
                        }`}>
                        {targetUser.role}
                      </span>
                    </div>
                  </div>
                </div>
              )}
            </div>

            <div className="space-y-6">
              <div className="bg-gradient-to-br from-pink-50 to-rose-50 rounded-2xl p-6 text-gray-900 shadow-lg border border-pink-100">
                <h3 className="text-gray-500 text-xs font-bold uppercase tracking-wider mb-6">成就统计</h3>
                <div className="space-y-6">
                  <div>
                    <p className="text-3xl font-black text-rose-950 mb-1">{targetUser.score}</p>
                    <p className="text-rose-800/60 text-xs font-medium">总积分</p>
                  </div>
                  <div className="pt-4 border-t border-pink-200/50">
                    <p className="text-2xl font-bold text-rose-950 mb-1">{targetUser.solved_count}</p>
                    <p className="text-rose-800/60 text-xs font-medium">解题数</p>
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
      
      {/* 提交热力图区域 */}
      <div className="mt-8 w-full min-w-0 overflow-hidden">
         <SubmissionHeatmap />
      </div>
    </div>
  )
}
