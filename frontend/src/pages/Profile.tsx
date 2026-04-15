import React, { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import ProfileChrome from '../components/profile/ProfileChrome'
import ProfilePanel from '../components/profile/ProfilePanel'
import SubmissionHeatmap from '../components/profile/SubmissionHeatmap'
import ContestHistory from '../components/profile/ContestHistory'
import { UserProfile } from '../types/profile'
import { fetchUserProfile } from '../utils/profileApi'
import { formatRelativeTime, normalizeLink, parseProfilePreferences } from '../components/profile/profileUtils'

export const Profile: React.FC = () => {
  const { username } = useParams<{ username?: string }>()
  const { user } = useAuth()
  const navigate = useNavigate()

  const [profile, setProfile] = useState<UserProfile | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const targetUsername = username || user?.username

  useEffect(() => {
    if (!targetUsername) {
      if (!user) {
        navigate('/login')
      }
      return
    }

    let isActive = true
    setLoading(true)
    setError('')
    setProfile(null)

    const loadData = async () => {
      try {
        const data = await fetchUserProfile(targetUsername)
        if (!isActive) return
        setProfile(data)
      } catch (err: any) {
        if (!isActive) return
        setError(err.message || '加载资料失败')
      } finally {
        if (isActive) {
          setLoading(false)
        }
      }
    }

    loadData()

    return () => {
      isActive = false
    }
  }, [navigate, targetUsername, user])

  const handleFollowChange = (isFollowing: boolean, newCount?: number) => {
    setProfile((current) => {
      if (!current) return current
      return {
        ...current,
        is_following: isFollowing,
        is_followed_by: isFollowing ? true : current.is_followed_by,
        stats: {
          ...current.stats,
          followers_count:
            newCount !== undefined
              ? newCount
              : Math.max(current.stats.followers_count + (isFollowing ? 1 : -1), 0),
        },
      }
    })
  }

  const preferences = useMemo(() => parseProfilePreferences(profile?.preferences), [profile?.preferences])

  const publicLinks = useMemo(() => {
    if (!profile) return []

    return [
      profile.website_url
        ? { label: '个人网站', value: profile.website_url, href: normalizeLink(profile.website_url) }
        : null,
      preferences.github_url
        ? { label: 'GitHub', value: preferences.github_url, href: normalizeLink(preferences.github_url) }
        : null,
      ...(preferences.social_links || [])
        .filter((item) => item?.label && item?.url)
        .map((item) => ({
          label: item.label,
          value: item.url,
          href: normalizeLink(item.url),
        })),
    ].filter((item): item is { label: string; value: string; href: string } => Boolean(item))
  }, [preferences.github_url, preferences.social_links, profile])

  return (
    <ProfileChrome
      profile={profile}
      currentView="profile"
      onFollowChange={handleFollowChange}
    >
      {error ? (
        <ProfilePanel title="资料加载失败">
          <div className="rounded-[24px] border border-rose-100 bg-rose-50 px-5 py-4 text-sm text-rose-600">{error}</div>
        </ProfilePanel>
      ) : loading && !profile ? (
        <div className="space-y-6">
          <div className="h-[26rem] animate-pulse rounded-[28px] glass-card" />
          <div className="grid gap-6 lg:grid-cols-2">
            <div className="h-64 animate-pulse rounded-[28px] glass-card" />
            <div className="h-64 animate-pulse rounded-[28px] glass-card" />
          </div>
        </div>
      ) : (
        <div className="space-y-6">
          <ProfilePanel title="资料总览">
            <div className="space-y-6">
              <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                <div className="rounded-2xl glass-soft px-4 py-4">
                  <div className="text-xs font-semibold uppercase tracking-[0.22em] text-slate-400">总积分</div>
                  <div className="mt-2 text-3xl font-semibold text-slate-950">{profile?.score ?? 0}</div>
                </div>
                <div className="rounded-2xl glass-soft px-4 py-4">
                  <div className="text-xs font-semibold uppercase tracking-[0.22em] text-slate-400">全站排名</div>
                  <div className="mt-2 text-3xl font-semibold text-sky-700">{profile?.rank_num ? `#${profile.rank_num}` : '-'}</div>
                </div>
                <div className="rounded-2xl glass-soft px-4 py-4">
                  <div className="text-xs font-semibold uppercase tracking-[0.22em] text-slate-400">总解题</div>
                  <div className="mt-2 text-3xl font-semibold text-slate-950">{profile?.solved_count ?? 0}</div>
                </div>
                <div className="rounded-2xl glass-soft px-4 py-4">
                  <div className="text-xs font-semibold uppercase tracking-[0.22em] text-slate-400">赛事参与</div>
                  <div className="mt-2 text-3xl font-semibold text-orange-600">{profile?.stats.contests_participated ?? 0}</div>
                </div>
              </div>

              <div className="grid gap-6 xl:grid-cols-[minmax(0,1.2fr),minmax(0,0.8fr)]">
                <section className="space-y-4">
                  <h3 className="text-sm font-semibold uppercase tracking-[0.22em] text-slate-400">资料摘要</h3>
                  {profile?.bio ? (
                    <div className="rounded-2xl glass-soft px-4 py-4 text-sm leading-7 text-slate-600">{profile.bio}</div>
                  ) : (
                    <div className="rounded-2xl border border-dashed border-slate-200/90 glass-soft px-4 py-6 text-sm text-slate-500">暂无个人简介</div>
                  )}

                  <div className="grid gap-3 sm:grid-cols-2">
                    <div className="rounded-2xl glass-soft px-4 py-4">
                      <div className="text-xs uppercase tracking-[0.22em] text-slate-400">注册时间</div>
                      <div className="mt-2 text-base font-semibold text-slate-950">{profile?.created_at ? new Date(profile.created_at).toLocaleDateString('zh-CN') : '暂无记录'}</div>
                    </div>
                    <div className="rounded-2xl glass-soft px-4 py-4">
                      <div className="text-xs uppercase tracking-[0.22em] text-slate-400">最近登录</div>
                      <div className="mt-2 text-base font-semibold text-slate-950">{formatRelativeTime(profile?.last_login_at)}</div>
                    </div>
                  </div>
                </section>

                <section className="space-y-4">
                  <h3 className="text-sm font-semibold uppercase tracking-[0.22em] text-slate-400">公开资料</h3>
                  {publicLinks.length > 0 ? (
                    <div className="space-y-3">
                      {publicLinks.map((item) => (
                        <a
                          key={`${item.label}-${item.href}`}
                          href={item.href}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="flex items-center justify-between gap-3 rounded-2xl glass-soft px-4 py-4 text-sm transition hover:border-slate-300 hover:bg-white"
                        >
                          <div className="min-w-0">
                            <div className="font-medium text-slate-950">{item.label}</div>
                            <div className="truncate text-slate-500">{item.value}</div>
                          </div>
                          <svg className="h-4 w-4 shrink-0 text-slate-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14 5h5m0 0v5m0-5L10 14m-1 5H5a2 2 0 01-2-2v-4" />
                          </svg>
                        </a>
                      ))}
                    </div>
                  ) : (
                    <div className="rounded-[24px] border border-dashed border-slate-200/90 glass-soft px-4 py-8 text-center text-sm text-slate-500">
                      暂无公开链接
                    </div>
                  )}
                </section>
              </div>
            </div>
          </ProfilePanel>

          <SubmissionHeatmap />

          <ProfilePanel title="赛事履历">
            <ContestHistory embedded />
          </ProfilePanel>
        </div>
      )}
    </ProfileChrome>
  )
}

export default Profile
