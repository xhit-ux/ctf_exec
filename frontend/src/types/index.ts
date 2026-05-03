export type Problem = {
    id: number
    title: string
    category: 'web' | 'pwn' | 'crypto' | 'reverse' | 'misc'
    points: number
    difficulty: '简单' | '中等' | '困难'
    tags?: string[]
    description: string
    is_solved: boolean
    // Flag is hidden from frontend in API response
}

export type Submission = {
    problemId: number | string
    problemTitle?: string
    flag: string
    correct: boolean
    time: string
}
