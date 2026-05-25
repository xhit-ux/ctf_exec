import { apiUrl } from './api'
import { apiFetch } from './authClient'
import { readSafeErrorMessage } from './error'

export type ChallengeInstance = {
  id: string
  user_id: string
  problem_id: string
  pod_name?: string
  namespace?: string
  access_url?: string
  node_port?: number
  status: 'pending' | 'running' | 'stopping' | 'destroyed' | 'failed'
  started_at?: string
  expired_at?: string
  destroyed_at?: string
  remaining_seconds?: number | null
}

type InstanceResponse = {
  status: string
  instance: ChallengeInstance | null
  message?: string
}

async function ensureOk(response: Response, fallback: string): Promise<Response> {
  if (response.ok) return response
  const message = await readSafeErrorMessage(response.clone(), fallback)
  throw new Error(message)
}

export async function fetchProblemInstance(problemID: string): Promise<InstanceResponse> {
  const response = await apiFetch(apiUrl(`/api/problems/${encodeURIComponent(problemID)}/instance`))
  await ensureOk(response, '获取实例状态失败')
  return response.json()
}

export async function startProblemInstance(problemID: string): Promise<InstanceResponse> {
  const response = await apiFetch(apiUrl(`/api/problems/${encodeURIComponent(problemID)}/instance/start`), {
    method: 'POST',
  })
  await ensureOk(response, '启动实例失败')
  return response.json()
}

export async function destroyProblemInstance(problemID: string): Promise<InstanceResponse> {
  const response = await apiFetch(apiUrl(`/api/problems/${encodeURIComponent(problemID)}/instance`), {
    method: 'DELETE',
  })
  await ensureOk(response, '销毁实例失败')
  return response.json()
}
