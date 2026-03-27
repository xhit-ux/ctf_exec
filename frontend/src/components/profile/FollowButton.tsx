import React, { useEffect, useState } from 'react'
import { useAuth } from '../../context/AuthContext'
import { followUser } from '../../utils/profileApi'

interface FollowButtonProps {
  username: string
  isFollowing: boolean | null
  isFollowedBy: boolean | null
  onFollowChange?: (isFollowing: boolean, newCount?: number) => void
  className?: string
}

export const FollowButton: React.FC<FollowButtonProps> = ({
  username,
  isFollowing: initialIsFollowing,
  isFollowedBy,
  onFollowChange,
  className = '',
}) => {
  const { user } = useAuth()
  const [isFollowing, setIsFollowing] = useState(Boolean(initialIsFollowing))
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    setIsFollowing(Boolean(initialIsFollowing))
  }, [initialIsFollowing, username])

  // If viewing own profile or not logged in, don't render or render as disabled
  if (!user || user.username === username) {
    return null
  }

  const isMutual = isFollowing && isFollowedBy

  if (isFollowing) {
    return (
      <span
        className={`inline-flex min-w-[5.75rem] items-center justify-center gap-1 rounded-full px-4 py-2 text-sm font-medium ${
          isMutual ? 'border border-sky-200 bg-sky-50 text-sky-700' : 'bg-slate-100 text-slate-600'
        } ${className}`}
      >
        {isMutual ? '已互关' : '已关注'}
      </span>
    )
  }

  const handleToggleEvent = async () => {
    setLoading(true)
    try {
      const res = await followUser(username)
      setIsFollowing(true)
      onFollowChange?.(true, res.followers_count)
    } catch (err: any) {
      console.error(err.message || '操作失败')
    } finally {
      setLoading(false)
    }
  }

  let btnClass =
    'inline-flex min-w-[5.75rem] items-center justify-center gap-1 rounded-full px-4 py-2 text-sm font-medium transition-all duration-200 '
  let text = '关注'

  btnClass += 'border border-slate-300 bg-white text-slate-900 hover:border-slate-400 hover:bg-slate-50'

  return (
    <button
      type="button"
      onClick={handleToggleEvent}
      disabled={loading}
      className={`${btnClass} ${className} disabled:cursor-not-allowed disabled:opacity-70`}
    >
      {loading ? (
        <svg className="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"></path>
        </svg>
      ) : (
        <>
          {isMutual && (
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
            </svg>
          )}
          <span>{text}</span>
        </>
      )}
    </button>
  )
}
