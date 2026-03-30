import { apiFetch } from './authClient'
import { apiUrl } from './api'
import { UserProfile, UpdateProfileRequest, FollowListResponse } from '../types/profile'

export async function fetchUserProfile(username: string): Promise<UserProfile> {
  const res = await apiFetch(apiUrl(`/api/users/profile/${username}`))
  if (!res.ok) throw new Error('获取用户资料失败')
  return res.json()
}

export async function updateProfile(data: UpdateProfileRequest): Promise<void> {
  const res = await apiFetch(apiUrl('/api/users/profile'), {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!res.ok) {
    const err = await res.json()
    throw new Error(err.error || '更新失败')
  }
}

export async function followUser(username: string): Promise<{ followers_count: number; is_following: boolean }> {
  const res = await apiFetch(apiUrl(`/api/users/${username}/follow`), { method: 'POST' })
  if (!res.ok) throw new Error('关注失败')
  return res.json()
}

export async function unfollowUser(username: string): Promise<{ followers_count: number; is_following: boolean }> {
  const res = await apiFetch(apiUrl(`/api/users/${username}/follow`), { method: 'DELETE' })
  if (!res.ok) throw new Error('取消关注失败')
  return res.json()
}

export async function fetchFollowers(username: string, page = 1, pageSize = 20, keyword = ''): Promise<FollowListResponse> {
  const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) })
  if (keyword) params.set('keyword', keyword)
  const res = await apiFetch(apiUrl(`/api/users/${username}/followers?${params}`), undefined, {
    disableAuth: true,
  })
  if (!res.ok) throw new Error('获取粉丝列表失败')
  return res.json()
}

export async function fetchFollowing(username: string, page = 1, pageSize = 20, keyword = ''): Promise<FollowListResponse> {
  const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) })
  if (keyword) params.set('keyword', keyword)
  const res = await apiFetch(apiUrl(`/api/users/${username}/following?${params}`), undefined, {
    disableAuth: true,
  })
  if (!res.ok) throw new Error('获取关注列表失败')
  return res.json()
}
