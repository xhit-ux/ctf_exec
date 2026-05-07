import { apiUrl } from './api'
import { apiFetch } from './authClient'
import { readSafeErrorMessage } from './error'
import type { ProblemHint } from './problemView'

async function ensureOk(response: Response, fallback: string): Promise<Response> {
  if (response.ok) return response
  const message = await readSafeErrorMessage(response.clone(), fallback)
  throw new Error(message)
}

function normalizeHint(item: unknown): ProblemHint {
  const raw = (item && typeof item === 'object' ? item : {}) as Record<string, unknown>
  return {
    id: String(raw.id ?? ''),
    cost: Number(raw.cost ?? 0) || 0,
    is_unlocked: Boolean(raw.is_unlocked),
    is_visible: raw.is_visible === undefined ? undefined : Boolean(raw.is_visible),
    content: typeof raw.content === 'string' ? raw.content : undefined,
  }
}

export async function listProblemHints(problemID: string): Promise<ProblemHint[]> {
  const res = await apiFetch(apiUrl(`/api/problems/${encodeURIComponent(problemID)}/hints`))
  await ensureOk(res, '获取提示列表失败')
  const data = await res.json()
  if (!Array.isArray(data)) return []
  return data.map(normalizeHint)
}

export async function unlockProblemHint(hintID: string): Promise<ProblemHint> {
  const res = await apiFetch(apiUrl(`/api/problems/hints/${encodeURIComponent(hintID)}/unlock`), {
    method: 'POST',
  })
  await ensureOk(res, '解锁提示失败')
  const data = await res.json()
  const payload =
    data && typeof data === 'object' && 'hint' in data
      ? (data as Record<string, unknown>).hint
      : data
  return normalizeHint(payload)
}
