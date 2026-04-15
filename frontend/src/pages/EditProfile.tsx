import React, { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import ProfileChrome from '../components/profile/ProfileChrome'
import ProfilePanel from '../components/profile/ProfilePanel'
import AvatarUpload from '../components/upload/AvatarUpload'
import { UpdateProfileRequest, UserProfile } from '../types/profile'
import { fetchUserProfile, updateProfile } from '../utils/profileApi'
import { parseProfilePreferences } from '../components/profile/profileUtils'

type FormField = HTMLInputElement | HTMLTextAreaElement

const INPUT_CLASS =
  'glass-input w-full rounded-2xl px-4 py-3 text-sm text-slate-900 placeholder:text-slate-400'

export const EditProfile: React.FC = () => {
  const { user, refreshUser } = useAuth()
  const navigate = useNavigate()

  const [profile, setProfile] = useState<UserProfile | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [showAvatarModal, setShowAvatarModal] = useState(false)
  const [error, setError] = useState('')

  const [formData, setFormData] = useState<UpdateProfileRequest>({
    display_name: '',
    email: '',
    bio: '',
    website_url: '',
    organization: '',
    country: '',
    email_public: false,
    github_url: '',
    social_links: [],
  })

  useEffect(() => {
    if (!user) {
      navigate('/login')
      return
    }

    let isActive = true
    setLoading(true)
    setError('')

    const init = async () => {
      try {
        const nextProfile = await fetchUserProfile(user.username)
        if (!isActive) return

        const prefs = parseProfilePreferences(nextProfile.preferences)
        setProfile(nextProfile)
        setFormData({
          display_name: nextProfile.display_name || '',
          email: nextProfile.email || '',
          bio: nextProfile.bio || '',
          website_url: nextProfile.website_url || '',
          organization: nextProfile.organization || '',
          country: nextProfile.country || '',
          email_public: prefs.email_public || false,
          github_url: prefs.github_url || '',
          social_links: prefs.social_links || [],
        })
      } catch (err: any) {
        if (!isActive) return
        setError(err.message || '无法加载个人资料')
      } finally {
        if (isActive) {
          setLoading(false)
        }
      }
    }

    init()

    return () => {
      isActive = false
    }
  }, [navigate, user])

  const handleChange = (event: React.ChangeEvent<FormField>) => {
    const { name, value } = event.target
    setFormData((current) => ({ ...current, [name]: value }))
  }

  const handleToggle = (event: React.ChangeEvent<HTMLInputElement>) => {
    const { name, checked } = event.target
    setFormData((current) => ({ ...current, [name]: checked }))
  }

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault()
    setSaving(true)
    setError('')

    try {
      await updateProfile(formData)
      await refreshUser?.()
      navigate(`/profile/${user?.username}`)
    } catch (err: any) {
      setError(err.message || '保存失败')
      setSaving(false)
    }
  }

  const handleAvatarUploaded = async (newAvatarUrl: string) => {
    setProfile((current) => {
      if (!current) return current
      return {
        ...current,
        avatar_url: newAvatarUrl,
      }
    })

    await refreshUser?.()
  }

  const completionStats = useMemo(() => {
    const filled = [
      formData.display_name,
      formData.email,
      formData.bio,
      formData.website_url,
      formData.organization,
      formData.country,
      formData.github_url,
    ].filter((item) => item && item.trim()).length

    return {
      filled,
      total: 7,
    }
  }, [formData])

  useEffect(() => {
    if (!showAvatarModal) return

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setShowAvatarModal(false)
      }
    }

    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [showAvatarModal])

  return (
    <ProfileChrome
      profile={profile}
      currentView="edit"
      onAvatarEdit={() => setShowAvatarModal(true)}
      heading="编辑个人资料"
      description="编辑页现在和主页共用同一套 profile 骨架，返回主页、查看关注页时都能保持熟悉的上下文和视觉结构。"
    >
      {!profile && error ? (
        <ProfilePanel
          title="资料加载失败"
          description="没有拿到当前用户资料时，不再展示空白表单，而是保留统一页面壳子并明确提示异常状态。"
        >
          <div className="rounded-[24px] border border-rose-100 bg-rose-50 px-5 py-4 text-sm text-rose-600">{error}</div>
        </ProfilePanel>
      ) : loading && !profile ? (
        <div className="space-y-6">
          <div className="grid gap-4 md:grid-cols-3">
            {Array.from({ length: 3 }).map((_, index) => (
              <div key={index} className="h-28 animate-pulse rounded-[24px] glass-card" />
            ))}
          </div>
          <div className="h-[36rem] animate-pulse rounded-[28px] glass-card" />
        </div>
      ) : (
        <>
        <form onSubmit={handleSubmit} className="space-y-6">
          <div className="grid gap-4 md:grid-cols-3">
            <div className="rounded-[24px] glass-card px-5 py-5">
              <div className="text-xs font-semibold uppercase tracking-[0.22em] text-slate-400">资料完成度</div>
              <div className="mt-3 text-3xl font-semibold text-slate-950">
                {completionStats.filled}/{completionStats.total}
              </div>
              <div className="mt-2 text-sm text-slate-500">已填写的基础资料字段</div>
            </div>
            <div className="rounded-[24px] glass-card px-5 py-5">
              <div className="text-xs font-semibold uppercase tracking-[0.22em] text-slate-400">粉丝可见</div>
              <div className="mt-3 text-3xl font-semibold text-slate-950">{formData.email_public ? '开启' : '关闭'}</div>
              <div className="mt-2 text-sm text-slate-500">邮箱公开状态会同步影响主页资料区</div>
            </div>
            <div className="rounded-[24px] glass-card px-5 py-5">
              <div className="text-xs font-semibold uppercase tracking-[0.22em] text-slate-400">保存去向</div>
              <div className="mt-3 text-3xl font-semibold text-slate-950">@{user?.username}</div>
              <div className="mt-2 text-sm text-slate-500">保存后会直接回到你的个人主页</div>
            </div>
          </div>

          {error && (
            <div className="rounded-[24px] border border-rose-100 bg-rose-50 px-5 py-4 text-sm text-rose-600">{error}</div>
          )}

          <ProfilePanel
            title="基础资料"
          >
            <div className="grid gap-5 lg:grid-cols-2">
              <div className="space-y-2">
                <label className="text-sm font-medium text-slate-700">显示名称</label>
                <input
                  type="text"
                  name="display_name"
                  value={formData.display_name}
                  onChange={handleChange}
                  required
                  className={INPUT_CLASS}
                  placeholder="想让别人如何称呼你"
                />
              </div>

              <div className="space-y-2">
                <label className="text-sm font-medium text-slate-700">电子邮箱</label>
                <input
                  type="email"
                  name="email"
                  value={formData.email}
                  onChange={handleChange}
                  required
                  className={INPUT_CLASS}
                  placeholder="you@example.com"
                />
              </div>

              <div className="space-y-2 lg:col-span-2">
                <label className="text-sm font-medium text-slate-700">个人简介</label>
                <textarea
                  name="bio"
                  value={formData.bio}
                  onChange={handleChange}
                  rows={5}
                  maxLength={200}
                  className={`${INPUT_CLASS} resize-none`}
                  placeholder="介绍一下你的方向、兴趣或最近在做什么"
                />
                <div className="text-right text-xs text-slate-400">{formData.bio.length} / 200</div>
              </div>
            </div>
          </ProfilePanel>

          <ProfilePanel
            title="补充信息"
            description="这些内容会出现在 sidebar 和概览面板里，帮助资料页更完整。"
          >
            <div className="grid gap-5 lg:grid-cols-2">
              <div className="space-y-2">
                <label className="text-sm font-medium text-slate-700">组织 / 学校</label>
                <input
                  type="text"
                  name="organization"
                  value={formData.organization}
                  onChange={handleChange}
                  className={INPUT_CLASS}
                  placeholder="Example University"
                />
              </div>

              <div className="space-y-2">
                <label className="text-sm font-medium text-slate-700">所在地区</label>
                <input
                  type="text"
                  name="country"
                  value={formData.country}
                  onChange={handleChange}
                  className={INPUT_CLASS}
                  placeholder="CN / US / Singapore"
                />
              </div>

              <div className="space-y-2">
                <label className="text-sm font-medium text-slate-700">个人网站</label>
                <input
                  type="url"
                  name="website_url"
                  value={formData.website_url}
                  onChange={handleChange}
                  className={INPUT_CLASS}
                  placeholder="https://..."
                />
              </div>

              <div className="space-y-2">
                <label className="text-sm font-medium text-slate-700">GitHub</label>
                <input
                  type="url"
                  name="github_url"
                  value={formData.github_url}
                  onChange={handleChange}
                  className={INPUT_CLASS}
                  placeholder="https://github.com/..."
                />
              </div>
            </div>
          </ProfilePanel>

          <ProfilePanel
            title="隐私设置"
            description="当前只保留邮箱公开设置，后续若补更多 profile 偏好也可以直接接在这一块。"
          >
            <label className="flex cursor-pointer items-start gap-4 rounded-[24px] glass-soft px-4 py-4 transition hover:border-slate-300 hover:bg-white">
              <div className="relative mt-0.5">
                <input
                  type="checkbox"
                  name="email_public"
                  checked={formData.email_public}
                  onChange={handleToggle}
                  className="sr-only"
                />
                <div className={`h-6 w-11 rounded-full transition ${formData.email_public ? 'bg-slate-950' : 'bg-slate-300'}`}></div>
                <div
                  className={`absolute left-1 top-1 h-4 w-4 rounded-full bg-white transition ${
                    formData.email_public ? 'translate-x-5' : ''
                  }`}
                ></div>
              </div>
              <div>
                <div className="text-sm font-medium text-slate-950">公开邮箱地址</div>
                <div className="mt-1 text-sm leading-6 text-slate-500">
                  开启后，其他用户可以在你的资料侧栏中看到邮箱信息。
                </div>
              </div>
            </label>
          </ProfilePanel>

          <div className="sticky bottom-4 z-10 flex justify-end">
            <div className="flex w-full items-center justify-between gap-4 rounded-[24px] glass-card px-5 py-4 sm:w-auto">
              <button
                type="button"
                onClick={() => navigate(`/profile/${user?.username}`)}
                disabled={saving}
                className="btn-secondary rounded-full disabled:opacity-60"
              >
                取消
              </button>
              <button
                type="submit"
                disabled={saving}
                className="btn-primary rounded-full disabled:cursor-not-allowed disabled:opacity-70"
              >
                {saving ? '保存中...' : '保存更改'}
              </button>
            </div>
          </div>
        </form>

        {showAvatarModal && (
          <div className="fixed inset-0 z-[140] flex items-center justify-center p-4">
            <button
              type="button"
              className="absolute inset-0 bg-slate-900/45 backdrop-blur-[2px]"
              onClick={() => setShowAvatarModal(false)}
              aria-label="关闭头像编辑弹窗"
            />
            <div className="relative w-full max-w-xl rounded-2xl border border-amber-100/80 bg-white/95 p-4 shadow-2xl">
              <div className="mb-3 flex items-center justify-between">
                <h3 className="text-base font-semibold text-slate-900">修改头像</h3>
                <button type="button" onClick={() => setShowAvatarModal(false)} className="btn-secondary !px-3 !py-2 text-xs">关闭</button>
              </div>

              <AvatarUpload
                currentAvatarUrl={profile?.avatar_url || user?.avatar_url}
                onUploadSuccess={(nextUrl) => {
                  void handleAvatarUploaded(nextUrl)
                  setShowAvatarModal(false)
                }}
              />
            </div>
          </div>
        )}
        </>
      )}
    </ProfileChrome>
  )
}

export default EditProfile
