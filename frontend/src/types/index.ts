// 与后端 v3.0 题目规范对齐的核心类型。
// 详细的 ProblemView/ProblemAttachment/ProblemHint/ProblemPrerequisiteRef/InstanceView
// 以及解析逻辑请参考 utils/problemView.ts。
import type {
  ProblemView,
  ProblemCategory,
  ProblemDifficulty,
  ProblemType,
} from '../utils/problemView'

export type {
  ProblemView,
  ProblemCategory,
  ProblemDifficulty,
  ProblemType,
}

// Problem 是 ProblemView 的别名，保留旧 import 路径不破坏。
export type Problem = ProblemView

export type Submission = {
  problemId: string
  problemTitle?: string
  flag: string
  correct: boolean
  time: string
}
