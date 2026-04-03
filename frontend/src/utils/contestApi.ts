import { apiUrl } from './api'
import { apiFetch } from './authClient'
import { readSafeErrorMessage } from './error'

export type Contest = {
  id: string
  title: string
  description: string
  cover_image?: string
  visibility: 'public' | 'invite_only' | 'private'
  start_time?: string | null
  end_time?: string | null
  scoring_type?: string
  created_at?: string
  updated_at?: string
}

export type ContestChallenge = {
  id: number
  title: string
  category?: string
  difficulty?: string
  points?: number
  score_override?: number | null
  is_solved?: boolean
}

export type ContestScoreboardEntry = {
  rank: number
  user_id?: string
  team_id?: string
  username?: string
  display_name?: string
  name?: string
  score: number
  solved_count: number
  last_solve_time?: string | null
}

export type ContestParticipantItem = {
  id: string
  contest_id: string
  user_id?: string
  team_id?: string
  username?: string
  display_name?: string
  team_name?: string
  status: string
  registered_at: string
}

export type ContestSubmissionItem = {
  id: string
  user_id?: string
  team_id?: string
  problem_id: number
  contest_id?: string
  is_correct: boolean
  ip_address?: string
  created_at: string
  username?: string
  display_name?: string
  team_name?: string
  problem_title?: string
}

export type CreateAdminProblemPayload = {
  title: string
  category: string
  type?: 'static' | 'dynamic'
  points: number
  difficulty: string
  description: string
  tags: string[]
  flag: string
  flag_rule?: string
  flag_example?: string
  docker_image?: string
  run_command?: string
  scope?: 'platform' | 'contest'
  contest_id?: string
  status?: 'draft' | 'review' | 'published' | 'archived'
  visible_from?: string | null
  visible_to?: string | null
}

export type AdminProblem = {
  id: number
  title: string
  category: string
  type: 'static' | 'dynamic'
  points: number
  difficulty: string
  description: string
  tags: string[]
  scope: 'platform' | 'contest'
  contest_id?: string
  status: 'draft' | 'review' | 'published' | 'archived'
  flag_rule?: string
  flag_example?: string
  docker_image?: string
  run_command?: string
  visible_from?: string | null
  visible_to?: string | null
}

export type ContestAdmin = {
  id: string
  contest_id: string
  user_id: string
  username: string
  role: 'admin' | 'observer'
  granted_by?: string
  granted_at: string
}

export type PagedResult<T> = {
  items: T[]
  page: number
  page_size: number
  total: number
  total_pages: number
}

function asArray<T>(payload: unknown): T[] {
  if (Array.isArray(payload)) return payload as T[]
  if (payload && typeof payload === 'object') {
    const dict = payload as Record<string, unknown>
    if (Array.isArray(dict.items)) return dict.items as T[]
    if (Array.isArray(dict.data)) return dict.data as T[]
    if (Array.isArray(dict.list)) return dict.list as T[]
  }
  return []
}

async function mustOK(response: Response, fallback: string): Promise<void> {
  if (response.ok) return
  const message = await readSafeErrorMessage(response.clone(), fallback)
  throw new Error(message)
}

export async function fetchContestDetail(contestID: string): Promise<Contest> {
  const response = await apiFetch(apiUrl(`/api/contests/${contestID}`), undefined, {
    disableAuth: true,
    disableRetry401: true,
  })
  await mustOK(response, '获取赛事详情失败')
  return await response.json() as Contest
}

export async function fetchContestChallenges(contestID: string): Promise<ContestChallenge[]> {
  const response = await apiFetch(apiUrl(`/api/contests/${contestID}/challenges`))
  await mustOK(response, '获取赛事题目失败')
  return asArray<ContestChallenge>(await response.json())
}

export async function fetchContestScoreboard(contestID: string, type: 'user' | 'team'): Promise<ContestScoreboardEntry[]> {
  const response = await apiFetch(apiUrl(`/api/contests/${contestID}/scoreboard?type=${type}&limit=50`), undefined, {
    disableAuth: true,
    disableRetry401: true,
  })
  await mustOK(response, '获取赛事榜单失败')
  return asArray<ContestScoreboardEntry>(await response.json())
}

export async function joinContest(contestID: string, payload?: { team_id?: string }): Promise<void> {
  const response = await apiFetch(apiUrl(`/api/contests/${contestID}/join`), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload || {}),
  })
  await mustOK(response, '报名失败')
}

export async function leaveContest(contestID: string, payload?: { team_id?: string }): Promise<void> {
  const response = await apiFetch(apiUrl(`/api/contests/${contestID}/leave`), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload || {}),
  })
  await mustOK(response, '退赛失败')
}

export async function fetchContestParticipants(contestID: string, page = 1, pageSize = 20): Promise<PagedResult<ContestParticipantItem>> {
  const response = await apiFetch(apiUrl(`/api/contests/${contestID}/participants?page=${page}&page_size=${pageSize}`))
  await mustOK(response, '获取参赛者失败')
  return await response.json() as PagedResult<ContestParticipantItem>
}

export async function fetchContestMySubmissions(contestID: string, page = 1, pageSize = 20): Promise<PagedResult<ContestSubmissionItem>> {
  const response = await apiFetch(apiUrl(`/api/contests/${contestID}/my-submissions?page=${page}&page_size=${pageSize}`))
  await mustOK(response, '获取我的提交失败')
  return await response.json() as PagedResult<ContestSubmissionItem>
}

export async function fetchContestAdmins(contestID: string): Promise<ContestAdmin[]> {
  const response = await apiFetch(apiUrl(`/api/admin/contests/${contestID}/admins`))
  await mustOK(response, '获取赛事管理员失败')
  return asArray<ContestAdmin>(await response.json())
}

export async function updateContest(contestID: string, payload: {
  title: string
  description: string
  cover_image: string
  visibility: 'public' | 'invite_only' | 'private'
  start_time: string | null
  end_time: string | null
  scoring_type: string
}): Promise<void> {
  const response = await apiFetch(apiUrl(`/api/admin/contests/${contestID}`), {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  await mustOK(response, '更新赛事失败')
}

export async function addContestChallenge(contestID: string, challengeID: number, scoreOverride?: number): Promise<void> {
  const response = await apiFetch(apiUrl(`/api/admin/contests/${contestID}/challenges`), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ challenge_id: challengeID, score_override: Number.isFinite(scoreOverride) ? scoreOverride : undefined }),
  })
  await mustOK(response, '添加赛事题目失败')
}

export async function createAdminProblem(payload: CreateAdminProblemPayload): Promise<void> {
  const response = await apiFetch(apiUrl('/api/admin/problems'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  await mustOK(response, '发布题目失败')
}

export async function updateAdminProblem(problemID: number, payload: CreateAdminProblemPayload): Promise<void> {
  const response = await apiFetch(apiUrl(`/api/admin/problems/${problemID}`), {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  await mustOK(response, '更新题目失败')
}

export async function fetchAdminProblem(problemID: number): Promise<AdminProblem> {
  const response = await apiFetch(apiUrl(`/api/admin/problems/${problemID}`))
  await mustOK(response, '获取题目详情失败')
  return await response.json() as AdminProblem
}

export async function fetchAdminProblems(params?: {
  keyword?: string
  scope?: 'platform' | 'contest' | ''
  status?: 'draft' | 'review' | 'published' | 'archived' | ''
}): Promise<AdminProblem[]> {
  const search = new URLSearchParams()
  if (params?.keyword) search.set('keyword', params.keyword)
  if (params?.scope) search.set('scope', params.scope)
  if (params?.status) search.set('status', params.status)

  const suffix = search.toString() ? `?${search.toString()}` : ''
  const response = await apiFetch(apiUrl(`/api/admin/problems${suffix}`))
  await mustOK(response, '获取管理端题目列表失败')
  const body = await response.json() as { items?: AdminProblem[] }
  return Array.isArray(body?.items) ? body.items : []
}

export async function removeContestChallenge(contestID: string, challengeID: number): Promise<void> {
  const response = await apiFetch(apiUrl(`/api/admin/contests/${contestID}/challenges/${challengeID}`), {
    method: 'DELETE',
  })
  await mustOK(response, '移除赛事题目失败')
}

export async function addContestAdmin(contestID: string, payload: { username: string; role: 'admin' | 'observer' }): Promise<void> {
  const response = await apiFetch(apiUrl(`/api/admin/contests/${contestID}/admins`), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  await mustOK(response, '添加赛事管理员失败')
}

export async function removeContestAdmin(contestID: string, username: string): Promise<void> {
  const response = await apiFetch(apiUrl(`/api/admin/contests/${contestID}/admins/${encodeURIComponent(username)}`), {
    method: 'DELETE',
  })
  await mustOK(response, '移除赛事管理员失败')
}

export async function fetchAdminContests(params?: {
  page?: number
  page_size?: number
  keyword?: string
  visibility?: 'public' | 'invite_only' | 'private' | ''
}): Promise<PagedResult<Contest>> {
  const search = new URLSearchParams()
  if (params?.page) search.set('page', String(params.page))
  if (params?.page_size) search.set('page_size', String(params.page_size))
  if (params?.keyword) search.set('keyword', params.keyword)
  if (params?.visibility) search.set('visibility', params.visibility)

  const suffix = search.toString() ? `?${search.toString()}` : ''
  const response = await apiFetch(apiUrl(`/api/admin/contests${suffix}`))
  await mustOK(response, '获取赛事管理列表失败')
  return await response.json() as PagedResult<Contest>
}

export async function createContest(payload: {
  title: string
  description: string
  cover_image: string
  visibility: 'public' | 'invite_only' | 'private'
  start_time: string | null
  end_time: string | null
  scoring_type: string
}): Promise<Contest> {
  const response = await apiFetch(apiUrl('/api/admin/contests'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  await mustOK(response, '创建赛事失败')
  return await response.json() as Contest
}

export async function fetchContestSubmissions(contestID: string, params?: {
  page?: number
  page_size?: number
  user_id?: string
  problem_id?: number
  result?: 'correct' | 'wrong' | ''
}): Promise<PagedResult<ContestSubmissionItem>> {
  const search = new URLSearchParams()
  if (params?.page) search.set('page', String(params.page))
  if (params?.page_size) search.set('page_size', String(params.page_size))
  if (params?.user_id) search.set('user_id', params.user_id)
  if (params?.problem_id) search.set('problem_id', String(params.problem_id))
  if (params?.result) search.set('result', params.result)

  const suffix = search.toString() ? `?${search.toString()}` : ''
  const response = await apiFetch(apiUrl(`/api/admin/contests/${contestID}/submissions${suffix}`))
  await mustOK(response, '获取赛事提交审计失败')
  return await response.json() as PagedResult<ContestSubmissionItem>
}
