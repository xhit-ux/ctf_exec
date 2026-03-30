import { API_BASE_URL } from '../../utils/api'
import { UserPreferences } from '../../types/profile'

type BasicProfileLike = {
  username: string
  display_name?: string
  avatar_url?: string
  preferences?: string
}

export function parseProfilePreferences(preferences?: string): UserPreferences {
  if (!preferences) {
    return {
      email_public: false,
      github_url: '',
      social_links: [],
    }
  }

  try {
    const parsed = JSON.parse(preferences) as Partial<UserPreferences>
    return {
      email_public: Boolean(parsed.email_public),
      github_url: parsed.github_url || '',
      social_links: Array.isArray(parsed.social_links) ? parsed.social_links : [],
    }
  } catch {
    return {
      email_public: false,
      github_url: '',
      social_links: [],
    }
  }
}

export function getProfileAvatarUrl(profile: BasicProfileLike): string {
  if (profile.avatar_url) {
    return profile.avatar_url.startsWith('http')
      ? profile.avatar_url
      : `${API_BASE_URL}${profile.avatar_url}`
  }

  const fallbackName = profile.display_name || profile.username || 'User'
  return `https://ui-avatars.com/api/?name=${encodeURIComponent(fallbackName)}&background=E2E8F0&color=0F172A`
}

export function formatProfileDate(date?: string, locale = 'zh-CN'): string {
  if (!date) return '暂无记录'

  const parsed = new Date(date)
  if (Number.isNaN(parsed.getTime())) return '暂无记录'

  return parsed.toLocaleDateString(locale, {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  })
}

/**
 * 将 ISO 时间戳格式化为相对时间。
 * - 24 小时内：X小时Y分钟之前
 * - 24h ~ 7天：N天前
 * - 超过 7 天：7天前
 */
export function formatRelativeTime(date?: string): string {
  if (!date) return '暂无记录'

  const parsed = new Date(date)
  if (Number.isNaN(parsed.getTime())) return '暂无记录'

  const diffMs = Date.now() - parsed.getTime()
  if (diffMs < 0) return '刚刚'

  const totalMinutes = Math.floor(diffMs / 60000)
  const totalHours = Math.floor(diffMs / 3600000)
  const totalDays = Math.floor(diffMs / 86400000)

  if (totalDays >= 7) return '7天前'
  if (totalDays >= 1) return `${totalDays}天前`

  // 24 小时内
  const hours = totalHours
  const minutes = totalMinutes % 60

  if (hours === 0 && minutes <= 15) return '在线'
  if (hours === 0) return `${minutes}分钟内`
  if (minutes === 0) return `${hours}小时内`
  return `${hours}小时内`
}

export function normalizeLink(url: string): string {
  if (!url) return ''
  return url.startsWith('http://') || url.startsWith('https://') ? url : `https://${url}`
}

export function isRecentlyActive(lastLoginAt?: string, windowMinutes = 15): boolean {
  if (!lastLoginAt) return false

  const parsed = new Date(lastLoginAt)
  if (Number.isNaN(parsed.getTime())) return false

  return Date.now() - parsed.getTime() < windowMinutes * 60 * 1000
}
