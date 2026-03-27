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

export function normalizeLink(url: string): string {
  if (!url) return ''
  return url.startsWith('http://') || url.startsWith('https://') ? url : `https://${url}`
}

export function isRecentlyActive(lastLoginAt?: string, windowMinutes = 20): boolean {
  if (!lastLoginAt) return false

  const parsed = new Date(lastLoginAt)
  if (Number.isNaN(parsed.getTime())) return false

  return Date.now() - parsed.getTime() < windowMinutes * 60 * 1000
}
