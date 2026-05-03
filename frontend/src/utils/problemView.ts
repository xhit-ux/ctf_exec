import { staticUrl } from './api'

export type ProblemCategory = 'web' | 'pwn' | 'crypto' | 'reverse' | 'misc'
export type ProblemDifficulty = '简单' | '中等' | '困难' | '专家'
export type ProblemType = 'static' | 'container_web' | 'container_tcp'

export type ProblemAttachment = {
  id: string
  name: string
  url: string
  size?: number
}

export type ProblemView = {
  id: string
  title: string
  category: ProblemCategory
  type: ProblemType
  difficulty: ProblemDifficulty
  points: number
  tags: string[]
  attachments: ProblemAttachment[]
  description: string
  is_solved: boolean
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

function normalizePoints(payload: Record<string, unknown>): number {
  const points = Number(payload.points)
  if (Number.isFinite(points)) return points

  const baseScore = Number(payload.base_score)
  if (Number.isFinite(baseScore)) return baseScore

  return 0
}

function normalizeAttachment(item: unknown, index: number): ProblemAttachment | null {
  if (typeof item === 'string') {
    const cleaned = item.trim()
    if (!cleaned) return null
    return {
      id: `att-${index}`,
      name: cleaned.split('/').pop() || `附件 ${index + 1}`,
      url: staticUrl(cleaned),
    }
  }

  if (item && typeof item === 'object') {
    const raw = item as Record<string, unknown>
    const rawURL = String(raw.download_url ?? raw.url ?? raw.file_path ?? '').trim()
    if (!rawURL) return null

    const rawName = String(raw.file_name ?? raw.filename ?? raw.name ?? '').trim()
    const fallbackName = rawURL.split('/').pop() || `附件 ${index + 1}`
    const sizeValue = Number(raw.file_size ?? raw.size)

    return {
      id: String(raw.id ?? `att-${index}`),
      name: rawName || fallbackName,
      url: staticUrl(rawURL),
      size: Number.isFinite(sizeValue) ? sizeValue : undefined,
    }
  }

  return null
}

function normalizeAttachments(value: unknown): ProblemAttachment[] {
  if (!Array.isArray(value)) return []
  return value
    .map((item, index) => normalizeAttachment(item, index))
    .filter((item): item is ProblemAttachment => Boolean(item))
}

export function normalizeProblem(payload: unknown): ProblemView {
  const item = (payload && typeof payload === 'object' ? payload : {}) as Record<string, unknown>

  const idRaw = item.id
  const id = idRaw === null || idRaw === undefined ? '' : String(idRaw)

  return {
    id,
    title: String(item.title ?? '未命名题目'),
    category: normalizeCategory(item.category),
    type: normalizeProblemType(item.type),
    difficulty: normalizeDifficulty(item.difficulty),
    points: normalizePoints(item),
    tags: normalizeTags(item.tags),
    attachments: normalizeAttachments(item.attachments),
    description: String(item.description ?? ''),
    is_solved: Boolean(item.is_solved),
  }
}

export function normalizeProblemList(payload: unknown): ProblemView[] {
  if (!Array.isArray(payload)) return []
  return payload.map((item) => normalizeProblem(item))
}
