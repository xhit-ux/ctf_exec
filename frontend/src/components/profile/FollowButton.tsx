import React, { useEffect, useState } from 'react'
import { useAuth } from '../../context/AuthContext'
import { followUser, unfollowUser } from '../../utils/profileApi'

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

  // If viewing own profile or not logged in, don't render
  if (!user || user.username === username) {
    return null
  }

  const isMutual = isFollowing && isFollowedBy

  const handleToggle = async () => {
    setLoading(true)
    try {
      if (isFollowing) {
        // 取消关注
        const res = await unfollowUser(username)
        setIsFollowing(false)
        onFollowChange?.(false, res.followers_count)
      } else {
        // 关注
        const res = await followUser(username)
        setIsFollowing(true)
        onFollowChange?.(true, res.followers_count)
      }
    } catch (err: any) {
      console.error(err.message || '操作失败')
    } finally {
      setLoading(false)
    }
  }

  // 按钮文字
  let buttonText: string
  if (isFollowing) {
    buttonText = isMutual ? '已互关' : '取消关注'
  } else {
    buttonText = '关注'
  }

  // 按钮样式
  let btnClass =
    'inline-flex min-w-[5.75rem] items-center justify-center gap-1 rounded-full px-4 py-2 text-sm font-medium transition-all duration-200 '

  if (isFollowing) {
    if (isMutual) {
      // 已互关：绿色
      btnClass += 'border border-green-200 bg-green-50 text-green-600 hover:border-red-300 hover:bg-red-50 hover:text-red-600'
    } else {
      // 单向已关注：红色
      btnClass += 'border border-red-200 bg-red-50 text-red-600 hover:border-red-300 hover:bg-red-100'
    }
  } else {
    // 未关注：蓝色
    btnClass += 'border border-blue-300 bg-white text-blue-600 hover:border-blue-400 hover:bg-blue-50'
  }

  return (
    <button
      type="button"
      onClick={handleToggle}
      disabled={loading}
      className={`${btnClass} ${className} disabled:cursor-not-allowed disabled:opacity-70`}
    >
      {loading ? (
        <svg className="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"></path>
        </svg>
      ) : (
        <span>{buttonText}</span>
      )}
    </button>
  )
}

