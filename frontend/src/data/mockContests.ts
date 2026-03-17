export type MockContestItem = {
  id: string
  title: string
  description: string
  cover_image?: string
  start_time?: string | null
  end_time?: string | null
  scoring_type?: string
}

export type MockContestChallenge = {
  id: number
  title: string
  category?: string
  difficulty?: string
  points?: number
  score_override?: number | null
  is_solved?: boolean
}

export type MockContestScoreUser = {
  rank: number
  user_id: string
  username: string
  display_name: string
  score: number
  solved_count: number
  last_solve_time?: string | null
}

export type MockContestScoreTeam = {
  rank: number
  team_id: string
  name: string
  score: number
  solved_count: number
  last_solve_time?: string | null
}

export const MOCK_CONTESTS: MockContestItem[] = [
  {
    id: 'mock-2026-spring',
    title: '春季校内选拔赛 2026',
    description: '面向新生与进阶选手的综合练习赛，覆盖 Web/Pwn/Crypto/Reverse。',
    start_time: '2026-03-10T10:00:00+08:00',
    end_time: '2026-04-20T18:00:00+08:00',
    scoring_type: 'jeopardy',
  },
  {
    id: 'mock-weekly-01',
    title: 'Weekly Practice #01',
    description: '每周训练赛，主打快速反馈和团队协作。',
    start_time: '2026-03-20T19:30:00+08:00',
    end_time: '2026-03-20T22:30:00+08:00',
    scoring_type: 'jeopardy',
  },
  {
    id: 'mock-rev-month',
    title: 'Reverse 专项月赛',
    description: '专项逆向训练，包含简单到中等难度样本。',
    start_time: '2026-02-01T09:00:00+08:00',
    end_time: '2026-02-28T23:00:00+08:00',
    scoring_type: 'jeopardy',
  },
]

export const MOCK_CONTEST_CHALLENGES: Record<string, MockContestChallenge[]> = {
  'mock-2026-spring': [
    { id: 1, title: 'Baby SQLi', category: 'web', difficulty: '简单', points: 100, is_solved: true },
    { id: 2, title: 'Format Hell', category: 'pwn', difficulty: '中等', points: 300, score_override: 350, is_solved: false },
    { id: 3, title: 'RSA Intro', category: 'crypto', difficulty: '简单', points: 150, is_solved: true },
    { id: 4, title: 'ELF Walkthrough', category: 'reverse', difficulty: '中等', points: 250, is_solved: false },
  ],
  'mock-weekly-01': [
    { id: 5, title: 'JWT Playground', category: 'web', difficulty: '中等', points: 200, is_solved: false },
    { id: 6, title: 'Simple Overflow', category: 'pwn', difficulty: '简单', points: 120, is_solved: false },
  ],
  'mock-rev-month': [
    { id: 7, title: 'CrackMe 101', category: 'reverse', difficulty: '简单', points: 100, is_solved: true },
    { id: 8, title: 'VM Maze', category: 'reverse', difficulty: '困难', points: 500, is_solved: false },
  ],
}

export const MOCK_CONTEST_USER_SCOREBOARD: Record<string, MockContestScoreUser[]> = {
  'mock-2026-spring': [
    { rank: 1, user_id: 'u1', username: 'alice', display_name: 'Alice', score: 860, solved_count: 5, last_solve_time: '2026-03-16T21:10:00+08:00' },
    { rank: 2, user_id: 'u2', username: 'bob', display_name: 'Bob', score: 780, solved_count: 4, last_solve_time: '2026-03-16T20:22:00+08:00' },
    { rank: 3, user_id: 'u3', username: 'charlie', display_name: 'Charlie', score: 620, solved_count: 4, last_solve_time: '2026-03-15T19:02:00+08:00' },
  ],
  'mock-weekly-01': [
    { rank: 1, user_id: 'u4', username: 'delta', display_name: 'Delta', score: 200, solved_count: 1, last_solve_time: '2026-03-20T20:00:00+08:00' },
  ],
  'mock-rev-month': [
    { rank: 1, user_id: 'u3', username: 'charlie', display_name: 'Charlie', score: 600, solved_count: 2, last_solve_time: '2026-02-20T21:10:00+08:00' },
  ],
}

export const MOCK_CONTEST_TEAM_SCOREBOARD: Record<string, MockContestScoreTeam[]> = {
  'mock-2026-spring': [
    { rank: 1, team_id: 't1', name: '0xAAA', score: 1400, solved_count: 8, last_solve_time: '2026-03-16T21:11:00+08:00' },
    { rank: 2, team_id: 't2', name: 'StackSmashers', score: 1220, solved_count: 7, last_solve_time: '2026-03-16T20:00:00+08:00' },
    { rank: 3, team_id: 't3', name: 'BlueShell', score: 940, solved_count: 5, last_solve_time: '2026-03-15T18:32:00+08:00' },
  ],
  'mock-weekly-01': [
    { rank: 1, team_id: 't2', name: 'StackSmashers', score: 320, solved_count: 2, last_solve_time: '2026-03-20T21:30:00+08:00' },
  ],
  'mock-rev-month': [
    { rank: 1, team_id: 't4', name: 'Reversers', score: 860, solved_count: 3, last_solve_time: '2026-02-26T22:00:00+08:00' },
  ],
}

export const MOCK_MY_TEAM = {
  id: 't1',
  name: '0xAAA',
}
