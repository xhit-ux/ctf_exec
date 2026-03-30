import React, { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import ProfileChrome from '../components/profile/ProfileChrome'
import ProfilePanel from '../components/profile/ProfilePanel'
import { FollowButton } from '../components/profile/FollowButton'
import { FollowUser, UserProfile } from '../types/profile'
import { fetchFollowers, fetchFollowing, fetchUserProfile } from '../utils/profileApi'
import { getProfileAvatarUrl } from '../components/profile/profileUtils'

interface FollowListProps {
  type: 'followers' | 'following'
}

export const FollowList: React.FC<FollowListProps> = ({ type }) => {
  const { username } = useParams<{ username: string }>()
  const { user } = useAuth()

  const [profile, setProfile] = useState<UserProfile | null>(null)
  const [users, setUsers] = useState<FollowUser[]>([])
  const [loading, setLoading] = useState(true)
  const [profileLoading, setProfileLoading] = useState(true)
  const [error, setError] = useState('')
  const [profileError, setProfileError] = useState('')
  const [page, setPage] = useState(1)
  const [hasMore, setHasMore] = useState(false)
  const [total, setTotal] = useState(0)

  const title = type === 'followers' ? '粉丝' : '关注'

  useEffect(() => {
    setPage(1)
    setUsers([])
    setHasMore(false)
    setTotal(0)
    setError('')
  }, [type, username])

  useEffect(() => {
    if (!username) return

    let isActive = true
    setProfileLoading(true)
    setProfileError('')
    setProfile(null)

    const loadProfile = async () => {
      try {
        const data = await fetchUserProfile(username)
        if (!isActive) return
        setProfile(data)
      } catch (err: any) {
        if (!isActive) return
        setProfileError(err.message || '加载资料失败')
      } finally {
        if (isActive) {
          setProfileLoading(false)
        }
      }
    }

    loadProfile()

    return () => {
      isActive = false
    }
  }, [username])

  useEffect(() => {
    if (!username) return

    let isActive = true
    setLoading(true)
    setError('')

    const loadUsers = async () => {
      try {
        const fetcher = type === 'followers' ? fetchFollowers : fetchFollowing
        const res = await fetcher(username, page, 20)
        if (!isActive) return

        setUsers((prev) => (page === 1 ? res.items || [] : [...prev, ...(res.items || [])]))
        setTotal(res.total)
        setHasMore(page < res.total_pages)
      } catch (err: any) {
        if (!isActive) return
        setError(err.message || '加载失败')
      } finally {
        if (isActive) {
          setLoading(false)
        }
      }
    }

    loadUsers()

    return () => {
      isActive = false
    }
  }, [page, type, username])

  const handleProfileFollowChange = (isFollowing: boolean, newCount?: number) => {
    setProfile((current) => {
      if (!current) return current
      return {
        ...current,
        is_following: isFollowing,
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

  const handleUserFollowChange = (target: FollowUser, isFollowing: boolean) => {
    setUsers((current) => {
      const next = current.map((item) =>
        item.username === target.username 
          ? { 
              ...item, 
              is_following: isFollowing,
              is_followed_by: isFollowing ? true : item.is_followed_by
            } 
          : item
      )

      if (type === 'following' && user?.username === username && !isFollowing) {
        return next.filter((item) => item.username !== target.username)
      }

      return next
    })

    if (type === 'following' && user?.username === username && !isFollowing) {
      setTotal((current) => Math.max(current - 1, 0))
    }
  }

  const summary = useMemo(() => {
    const mutualCount = users.filter((item) => item.is_following && item.is_followed_by).length
    const withBioCount = users.filter((item) => item.bio).length

    return [
      { label: type === 'followers' ? '粉丝总数' : '关注总数', value: total },
      { label: '互相关注', value: mutualCount },
      { label: '已填写简介', value: withBioCount },
    ]
  }, [total, type, users])

  return (
    <ProfileChrome
      profile={profile}
      currentView={type}
      onFollowChange={handleProfileFollowChange}
      heading={profile ? `${profile.display_name || profile.username} 的${title}` : `加载${title}中`}
    >
      {profileError ? (
        <ProfilePanel title="资料加载失败" description="用户资料没有拿到时，保留 profile 骨架并在内容区展示错误信息。">
          <div className="rounded-[24px] border border-rose-100 bg-rose-50 px-5 py-4 text-sm text-rose-600">{profileError}</div>
        </ProfilePanel>
      ) : profileLoading && !profile ? (
        <div className="space-y-6">
          <div className="grid gap-4 md:grid-cols-3">
            {Array.from({ length: 3 }).map((_, index) => (
              <div key={index} className="h-28 animate-pulse rounded-[24px] border border-slate-200 bg-white" />
            ))}
          </div>
          <div className="h-96 animate-pulse rounded-[28px] border border-slate-200 bg-white" />
        </div>
      ) : (
        <div className="space-y-6">
          <div className="grid gap-4 md:grid-cols-3">
            {summary.map((item) => (
              <div
                key={item.label}
                className="rounded-[24px] border border-slate-200/80 bg-white px-5 py-5 shadow-[0_20px_60px_-45px_rgba(15,23,42,0.45)]"
              >
                <div className="text-xs font-semibold uppercase tracking-[0.22em] text-slate-400">{item.label}</div>
                <div className="mt-3 text-3xl font-semibold text-slate-950">{item.value}</div>
              </div>
            ))}
          </div>

          <ProfilePanel
            title={`${title}列表`}
            action={
              hasMore && !loading ? (
                <button
                  type="button"
                  onClick={() => setPage((current) => current + 1)}
                  className="rounded-full border border-slate-200 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-300 hover:bg-slate-50"
                >
                  加载更多
                </button>
              ) : undefined
            }
          >
            {error ? (
              <div className="rounded-[24px] border border-rose-100 bg-rose-50 px-5 py-4 text-sm text-rose-600">{error}</div>
            ) : users.length === 0 && !loading ? (
              <div className="rounded-[24px] border border-dashed border-slate-200 bg-slate-50 px-4 py-12 text-center text-sm text-slate-500">
                还没有{title}记录
              </div>
            ) : (
              <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
                {users.map((item) => {
                  const avatarUrl = getProfileAvatarUrl(item)
                  const displayName = item.display_name || item.username

                  return (
                    <div
                      key={item.id}
                      className="group flex min-w-0 items-center gap-4 rounded-[24px] border border-slate-200 bg-slate-50/70 px-4 py-4 transition hover:border-slate-300 hover:bg-white"
                    >
                      <Link to={`/profile/${item.username}`} className="shrink-0">
                        <img
                          src={avatarUrl}
                          alt={item.username}
                          className="h-14 w-14 rounded-2xl object-cover ring-1 ring-slate-200 transition group-hover:ring-slate-300"
                        />
                      </Link>

                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <Link
                            to={`/profile/${item.username}`}
                            className="truncate text-base font-semibold tracking-tight text-slate-950 transition hover:text-sky-700"
                          >
                            {displayName}
                          </Link>
                          {item.is_following && item.is_followed_by && (
                            <span className="rounded-full bg-sky-50 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.18em] text-sky-700">
                              friend
                            </span>
                          )}
                        </div>
                        <div className="mt-1 truncate text-sm text-slate-500">@{item.username}</div>
                        {item.bio && <div className="mt-2 text-sm leading-6 text-slate-500">{item.bio}</div>}
                      </div>

                      <div className="shrink-0">
                        <FollowButton
                          username={item.username}
                          isFollowing={item.is_following}
                          isFollowedBy={item.is_followed_by}
                          onFollowChange={(isFollowing) => handleUserFollowChange(item, isFollowing)}
                        />
                      </div>
                    </div>
                  )
                })}
              </div>
            )}

            {loading && (
              <div className="mt-5 flex justify-center">
                <svg className="h-8 w-8 animate-spin text-slate-400" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"></path>
                </svg>
              </div>
            )}
          </ProfilePanel>
        </div>
      )}
    </ProfileChrome>
  )
}

export default FollowList
