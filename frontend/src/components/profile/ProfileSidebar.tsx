import React from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import { UserProfile } from '../../types/profile'
import { FollowButton } from './FollowButton'
import { formatProfileDate, formatRelativeTime, getProfileAvatarUrl, isRecentlyActive, normalizeLink, parseProfilePreferences } from './profileUtils'

interface ProfileSidebarProps {
  profile: UserProfile | null
  onFollowChange?: (isFollowing: boolean, newCount?: number) => void
  showOwnEditButton?: boolean
}

export const ProfileSidebar: React.FC<ProfileSidebarProps> = ({
  profile,
  onFollowChange,
  showOwnEditButton = true,
}) => {
  const navigate = useNavigate()
  const { user } = useAuth()

  if (!profile) {
    return (
      <aside className="space-y-4 xl:sticky xl:top-6">
        <div className="animate-pulse overflow-hidden rounded-[28px] border border-slate-200 bg-white">
          <div className="h-28 bg-slate-100" />
          <div className="px-5 pb-6">
            <div className="-mt-12 h-24 w-24 rounded-3xl border-4 border-white bg-slate-200" />
            <div className="mt-5 h-7 w-40 rounded-full bg-slate-200" />
            <div className="mt-3 h-4 w-28 rounded-full bg-slate-100" />
            <div className="mt-5 h-10 rounded-2xl bg-slate-100" />
          </div>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <div className="h-24 animate-pulse rounded-[24px] border border-slate-200 bg-white" />
          <div className="h-24 animate-pulse rounded-[24px] border border-slate-200 bg-white" />
        </div>
      </aside>
    )
  }

  const avatarUrl = getProfileAvatarUrl(profile)
  const preferences = parseProfilePreferences(profile.preferences)
  const displayName = profile.display_name || profile.username
  const effectiveLastLoginAt = profile.is_own_profile ? (user?.last_login_at || profile.last_login_at) : profile.last_login_at
  const isOnline = profile.is_own_profile ? isRecentlyActive(effectiveLastLoginAt, 20) : profile.is_online
  const socialLinks = [
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

  return (
    <aside className="space-y-4 xl:sticky xl:top-6">
      <div className="overflow-hidden rounded-[28px] border border-slate-200/80 bg-white shadow-[0_20px_60px_-45px_rgba(15,23,42,0.45)]">
        <div className="h-24 bg-[linear-gradient(135deg,rgba(14,165,233,0.16),rgba(249,115,22,0.18),rgba(15,23,42,0.08))]" />

        <div className="px-5 pb-6">
          <div className="-mt-12 flex items-end gap-4">
            <div className="relative">
              <div className="h-24 w-24 overflow-hidden rounded-[28px] border-4 border-white bg-slate-100 shadow-lg ring-1 ring-slate-200/80">
                <img src={avatarUrl} alt={profile.username} className="h-full w-full object-cover" />
              </div>
              <div
                className={`absolute -bottom-2 -right-2 rounded-full border-4 border-white p-1 text-white shadow-md ${
                  isOnline ? 'bg-emerald-500' : 'bg-slate-400'
                }`}
                title={isOnline ? '在线' : '离线'}
                aria-label={isOnline ? '在线' : '离线'}
              >
                {isOnline ? (
                  <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M5 13l4 4L19 7" />
                  </svg>
                ) : (
                  <span className="block h-4 w-4 rounded-full bg-white/80" />
                )}
              </div>
            </div>
          </div>
          <div className="mt-5 space-y-4">
            <div className="space-y-2">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <h2 className="text-2xl font-semibold tracking-tight text-slate-950">{displayName}</h2>
                    {profile.role !== 'user' && (
                      <span className="rounded-full bg-slate-100 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-600">
                        {profile.role}
                      </span>
                    )}
                  </div>
                  <p className="text-sm font-medium text-slate-500">@{profile.username}</p>
                </div>

                {profile.is_own_profile && showOwnEditButton && (
                  <button
                    type="button"
                    onClick={() => navigate(`/profile/${profile.username}/edit`)}
                    className="shrink-0 rounded-full border border-slate-200 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-300 hover:bg-slate-50"
                  >
                    编辑资料
                  </button>
                )}
              </div>
            </div>

            {profile.bio && <p className="rounded-2xl bg-slate-50 px-4 py-4 text-sm leading-7 text-slate-600">{profile.bio}</p>}

            {!profile.is_own_profile && (
              <FollowButton
                className="w-full"
                username={profile.username}
                isFollowing={profile.is_following}
                isFollowedBy={profile.is_followed_by}
                onFollowChange={onFollowChange}
              />
            )}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <button
          type="button"
          onClick={() => navigate(`/profile/${profile.username}/followers`)}
          className="rounded-[24px] border border-slate-200/80 bg-white px-4 py-4 text-left shadow-[0_20px_60px_-45px_rgba(15,23,42,0.45)] transition hover:border-sky-200 hover:bg-sky-50/40"
        >
          <div className="text-xs uppercase tracking-[0.22em] text-slate-400">粉丝</div>
          <div className="mt-2 text-2xl font-semibold text-slate-950">{profile.stats.followers_count}</div>
        </button>
        <button
          type="button"
          onClick={() => navigate(`/profile/${profile.username}/following`)}
          className="rounded-[24px] border border-slate-200/80 bg-white px-4 py-4 text-left shadow-[0_20px_60px_-45px_rgba(15,23,42,0.45)] transition hover:border-orange-200 hover:bg-orange-50/40"
        >
          <div className="text-xs uppercase tracking-[0.22em] text-slate-400">关注中</div>
          <div className="mt-2 text-2xl font-semibold text-slate-950">{profile.stats.following_count}</div>
        </button>
      </div>

      <div className="rounded-[28px] border border-slate-200/80 bg-white p-5 shadow-[0_20px_60px_-45px_rgba(15,23,42,0.45)]">
        <h3 className="text-sm font-semibold uppercase tracking-[0.22em] text-slate-400">资料概览</h3>
        <ul className="mt-4 space-y-3 text-sm text-slate-600">
          {profile.organization && (
            <li className="flex items-start gap-3 rounded-2xl bg-slate-50 px-4 py-3">
              <span className="mt-0.5 text-slate-400">机构</span>
              <span className="min-w-0 flex-1 font-medium text-slate-900">{profile.organization}</span>
            </li>
          )}
          {profile.country && (
            <li className="flex items-start gap-3 rounded-2xl bg-slate-50 px-4 py-3">
              <span className="mt-0.5 text-slate-400">地区</span>
              <span className="min-w-0 flex-1 font-medium text-slate-900">{profile.country}</span>
            </li>
          )}
          {profile.email && (profile.is_own_profile || preferences.email_public) && (
            <li className="flex items-start gap-3 rounded-2xl bg-slate-50 px-4 py-3">
              <span className="mt-0.5 text-slate-400">邮箱</span>
              <a href={`mailto:${profile.email}`} className="min-w-0 flex-1 truncate font-medium text-slate-900 transition hover:text-sky-700">
                {profile.email}
              </a>
            </li>
          )}
          <li className="flex items-start gap-3 rounded-2xl bg-slate-50 px-4 py-3">
            <span className="mt-0.5 text-slate-400">注册于</span>
            <span className="min-w-0 flex-1 font-medium text-slate-900">{formatProfileDate(profile.created_at)}</span>
          </li>
          <li className="flex items-start gap-3 rounded-2xl bg-slate-50 px-4 py-3">
            <span className="mt-0.5 text-slate-400">最近登录</span>
            <span className="min-w-0 flex-1 font-medium text-slate-900">{formatRelativeTime(profile.last_login_at)}</span>
          </li>
        </ul>
      </div>

      {socialLinks.length > 0 && (
        <div className="rounded-[28px] border border-slate-200/80 bg-white p-5 shadow-[0_20px_60px_-45px_rgba(15,23,42,0.45)]">
          <h3 className="text-sm font-semibold uppercase tracking-[0.22em] text-slate-400">公开链接</h3>
          <div className="mt-4 space-y-3">
            {socialLinks.map((item) => (
              <a
                key={`${item.label}-${item.href}`}
                href={item.href}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center justify-between gap-3 rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-700 transition hover:border-slate-300 hover:bg-white"
              >
                <div className="min-w-0">
                  <div className="font-medium text-slate-900">{item.label}</div>
                  <div className="truncate text-slate-500">{item.value}</div>
                </div>
                <svg className="h-4 w-4 shrink-0 text-slate-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14 5h5m0 0v5m0-5L10 14m-1 5H5a2 2 0 01-2-2v-4" />
                </svg>
              </a>
            ))}
          </div>
        </div>
      )}
    </aside>
  )
}
