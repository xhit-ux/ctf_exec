export interface UserStats {
  followers_count: number
  following_count: number
  contests_participated: number
  rank_num: number | null
}

export interface SocialLink {
  label: string
  url: string
}

export interface UserPreferences {
  email_public: boolean
  github_url?: string
  social_links?: SocialLink[]
}

export interface UserProfile {
  id: string
  username: string
  email: string
  role: string
  display_name: string
  avatar_url?: string
  bio?: string
  website_url?: string
  organization?: string
  country?: string
  score: number
  solved_count: number
  rank_num?: number
  is_active: boolean
  is_verified: boolean
  created_at: string
  updated_at: string
  last_login_at?: string
  last_active_at?: string
  preferences?: string

  stats: UserStats
  is_own_profile: boolean
  is_following: boolean | null
  is_followed_by: boolean | null
  is_online: boolean
}

export interface FollowUser {
  id: string
  username: string
  display_name: string
  avatar_url: string
  bio: string
  is_following: boolean | null
  is_followed_by: boolean | null
}

export interface FollowListResponse {
  items: FollowUser[]
  page: number
  page_size: number
  total: number
  total_pages: number
}

export interface UpdateProfileRequest {
  display_name: string
  email: string
  bio: string
  website_url: string
  organization: string
  country: string
  email_public: boolean
  github_url: string
  social_links: SocialLink[]
}
