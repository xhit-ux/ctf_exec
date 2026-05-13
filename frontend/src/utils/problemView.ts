import { staticUrl } from './api'

export type ProblemCategory = 'web' | 'pwn' | 'crypto' | 'reverse' | 'misc'
export type ProblemDifficulty = '简单' | '中等' | '困难' | '专家'
export type ProblemType = 'static' | 'container_web' | 'container_tcp'
export type ProblemStatus = 'draft' | 'ready' | 'hidden'

// 单附件（v2.1：一题一包）。
export type ProblemAttachment = {
  id: string
  name: string
  url: string
  size?: number
  sha256?: string
}

// v3.0 Part 9：提示
export type ProblemHint = {
  id: string
  cost: number
  is_unlocked: boolean
  is_visible?: boolean
  content?: string
}

// v3.0 Part 9：前置依赖
export type ProblemPrerequisiteRef = {
  id: string
  title: string
  is_solved: boolean
}

export type ProblemInstanceStatus = 'idle' | 'pending' | 'running' | 'failed' | 'stopping' | 'destroyed'

export type ProblemInstanceView = {
  status: ProblemInstanceStatus
  access_url?: string | null
  started_at?: string | null
  expires_at?: string | null
  remaining_seconds?: number | null
}

export type ConnectionInfo = {
  http?: string | null
  tcp?: string | null
}

// 用户视角的题目视图 — 与后端 GET /api/problems/:id 字段对齐。
export type ProblemView = {
  id: string
  title: string
  category: ProblemCategory
  type: ProblemType
  difficulty: ProblemDifficulty
  // platform_score 是平台 scope 的固定分值；contest_score（可选）来自 contest scope。
  platform_score: number
  contest_score?: number
  current_score?: number
  is_dynamic_score?: boolean
  initial_score?: number
  min_score?: number
  decay_constant?: number
  solves_count?: number
  solve_count?: number
  tags: string[]
  author?: string | null
  description: string
  attachment?: ProblemAttachment | null
  hints?: ProblemHint[]
  prerequisites?: ProblemPrerequisiteRef[]
  instance?: ProblemInstanceView | null
  connection_info?: ConnectionInfo | null
  is_solved: boolean
  is_locked?: boolean
  status?: ProblemStatus
  /** 兼容字段：等价于 platform_score。仅供旧页面读取，不写入。 */
  points: number
}

const CATEGORY_LABELS: Record<ProblemCategory, string> = {
  web: 'Web',
  pwn: 'Pwn',
  crypto: 'Crypto',
  reverse: 'Reverse',
  misc: 'Misc',
}

const DIFFICULTY_LABELS: Record<ProblemDifficulty, string> = {
  简单: 'Easy',
  中等: 'Medium',
  困难: 'Hard',
  专家: 'Expert',
}

export function categoryLabel(value: ProblemCategory): string {
  return CATEGORY_LABELS[value]
}

export function difficultyLabel(value: ProblemDifficulty): string {
  return DIFFICULTY_LABELS[value]
}

export function normalizeCategory(value: unknown): ProblemCategory {
  const raw = String(value ?? '').trim().toLowerCase()
  if (raw === 'web') return 'web'
  if (raw === 'pwn') return 'pwn'
  if (raw === 'crypto') return 'crypto'
  if (raw === 'reverse' || raw === 'rev') return 'reverse'
  return 'misc'
}

export function normalizeDifficulty(value: unknown): ProblemDifficulty {
  const raw = String(value ?? '').trim().toLowerCase()
  if (raw === 'easy' || raw === '简单') return '简单'
  if (raw === 'medium' || raw === '中等') return '中等'
  if (raw === 'hard' || raw === '困难') return '困难'
  if (raw === 'expert' || raw === '专家') return '专家'
  return '中等'
}

export function normalizeProblemType(value: unknown): ProblemType {
  const raw = String(value ?? '').trim().toLowerCase()
  if (raw === 'container_web') return 'container_web'
  if (raw === 'container_tcp') return 'container_tcp'
  return 'static'
}

function normalizeTags(value: unknown): string[] {
  if (Array.isArray(value)) {
    return Array.from(
      new Set(
        value
          .map((item) => String(item ?? '').trim())
          .filter(Boolean),
      ),
    )
  }

  if (typeof value === 'string') {
    const segments = value
      .split(/[;,\n]/)
      .map((item) => item.trim())
      .filter(Boolean)
    return Array.from(new Set(segments))
  }

  return []
}

function normalizePlatformScore(payload: Record<string, unknown>): number {
  // 优先 platform_score → contest 端的 current_score → contest_score → 旧字段 points/base_score。
  for (const key of ['platform_score', 'current_score', 'contest_score', 'points', 'base_score']) {
    const v = Number((payload as Record<string, unknown>)[key])
    if (Number.isFinite(v)) return v
  }
  return 0
}

function normalizeAttachment(value: unknown): ProblemAttachment | null {
  if (!value || typeof value !== 'object') return null
  const raw = value as Record<string, unknown>
  const rawURL = String(raw.download_url ?? raw.url ?? raw.file_path ?? raw.object_key ?? '').trim()
  if (!rawURL) return null
  const rawName = String(raw.file_name ?? raw.filename ?? raw.name ?? '').trim()
  const sizeValue = Number(raw.file_size ?? raw.size)
  const sha256 = String(raw.sha256 ?? '').trim() || undefined
  return {
    id: String(raw.id ?? 'attachment'),
    name: rawName || rawURL.split('/').pop() || '附件',
    url: staticUrl(rawURL),
    size: Number.isFinite(sizeValue) ? sizeValue : undefined,
    sha256,
  }
}

function normalizeHints(value: unknown): ProblemHint[] | undefined {
  if (!Array.isArray(value)) return undefined
  return value
    .map((item) => {
      if (!item || typeof item !== 'object') return null
      const raw = item as Record<string, unknown>
      return {
        id: String(raw.id ?? ''),
        cost: Number(raw.cost ?? 0) || 0,
        is_unlocked: Boolean(raw.is_unlocked),
        is_visible: raw.is_visible === undefined ? undefined : Boolean(raw.is_visible),
        content: typeof raw.content === 'string' ? raw.content : undefined,
      } as ProblemHint
    })
    .filter((item): item is ProblemHint => Boolean(item))
}

function normalizePrerequisites(value: unknown): ProblemPrerequisiteRef[] | undefined {
  if (!Array.isArray(value)) return undefined
  return value
    .map((item) => {
      if (!item || typeof item !== 'object') return null
      const raw = item as Record<string, unknown>
      return {
        id: String(raw.id ?? ''),
        title: String(raw.title ?? ''),
        is_solved: Boolean(raw.is_solved),
      } as ProblemPrerequisiteRef
    })
    .filter((item): item is ProblemPrerequisiteRef => Boolean(item))
}

function normalizeInstance(value: unknown): ProblemInstanceView | null | undefined {
  if (value === null) return null
  if (!value || typeof value !== 'object') return undefined
  const raw = value as Record<string, unknown>
  const rawStatus = String(raw.status ?? 'idle').toLowerCase()
  const status = (rawStatus === 'destroyed' ? 'stopping' : rawStatus) as ProblemInstanceStatus
  return {
    status,
    access_url: typeof raw.access_url === 'string' ? raw.access_url : null,
    started_at: typeof raw.started_at === 'string' ? raw.started_at : null,
    expires_at: typeof raw.expires_at === 'string' ? raw.expires_at : null,
    remaining_seconds:
      typeof raw.remaining_seconds === 'number' ? raw.remaining_seconds : null,
  }
}

function normalizeConnectionInfo(value: unknown): ConnectionInfo | null | undefined {
  if (value === null) return null
  if (!value || typeof value !== 'object') return undefined
  const raw = value as Record<string, unknown>
  const out: ConnectionInfo = {}
  if (typeof raw.http === 'string') out.http = raw.http
  if (typeof raw.tcp === 'string') out.tcp = raw.tcp
  if (out.http === undefined && out.tcp === undefined) return null
  return out
}

export function normalizeProblem(payload: unknown): ProblemView {
  const item = (payload && typeof payload === 'object' ? payload : {}) as Record<string, unknown>

  const idRaw = item.id
  const id = idRaw === null || idRaw === undefined ? '' : String(idRaw)
  const platformScore = normalizePlatformScore(item)

  const view: ProblemView = {
    id,
    title: String(item.title ?? '未命名题目'),
    category: normalizeCategory(item.category),
    type: normalizeProblemType(item.type),
    difficulty: normalizeDifficulty(item.difficulty),
    platform_score: platformScore,
    points: platformScore,
    tags: normalizeTags(item.tags),
    description: String(item.description ?? ''),
    is_solved: Boolean(item.is_solved),
  }

  if (typeof item.author === 'string') view.author = item.author
  if (Number.isFinite(Number(item.solves_count))) view.solves_count = Number(item.solves_count)
  if (Number.isFinite(Number(item.solve_count))) view.solve_count = Number(item.solve_count)
  if (Number.isFinite(Number(item.contest_score))) view.contest_score = Number(item.contest_score)
  if (Number.isFinite(Number(item.current_score))) view.current_score = Number(item.current_score)
  if (typeof item.is_dynamic_score === 'boolean') view.is_dynamic_score = item.is_dynamic_score
  if (Number.isFinite(Number(item.initial_score))) view.initial_score = Number(item.initial_score)
  if (Number.isFinite(Number(item.min_score))) view.min_score = Number(item.min_score)
  if (Number.isFinite(Number(item.decay_constant))) view.decay_constant = Number(item.decay_constant)
  if (typeof item.is_locked === 'boolean') view.is_locked = item.is_locked
  if (typeof item.status === 'string') view.status = item.status as ProblemStatus

  // 单附件（v2.1）。后端可能返回旧 attachments 数组以做兼容。
  if (item.attachment !== undefined) {
    view.attachment = normalizeAttachment(item.attachment)
  } else if (Array.isArray(item.attachments) && item.attachments.length > 0) {
    view.attachment = normalizeAttachment(item.attachments[0])
  } else {
    view.attachment = null
  }

  view.hints = normalizeHints(item.hints) ?? []
  view.prerequisites = normalizePrerequisites(item.prerequisites) ?? []
  view.instance = normalizeInstance(item.instance) ?? null
  view.connection_info = normalizeConnectionInfo(item.connection_info) ?? null

  return view
}

export function normalizeProblemList(payload: unknown): ProblemView[] {
  if (!Array.isArray(payload)) return []
  return payload.map((item) => normalizeProblem(item))
}
