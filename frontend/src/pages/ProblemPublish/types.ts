export type ProblemCategory = 'Web' | 'Pwn' | 'Crypto' | 'Reverse' | 'Misc';
export type Difficulty = 'Easy' | 'Medium' | 'Hard' | 'Expert';
// v2.1：flag_type / flag_rule / flag_inject 三个正交维度。
export type FlagType = 'static' | 'dynamic';
export type FlagRule = 'exact' | 'case_insensitive' | 'regex' | 'contains';
export type FlagInject = 'env' | 'sidecar';
export type DeployMode = 'shared' | 'per_user';
export type AccessType = 'ingress' | 'nodeport';
export type BuildUploadType = 'docker_save' | 'compose_bundle';

export interface Attachment {
  id: string;
  filename: string;
  size: number;
  sha256: string;
  status: 'uploading' | 'success' | 'error';
}

// 难度对应平台分值区间（v2.1 Part 3.1）。
export const DIFFICULTY_SCORE_RANGE: Record<Difficulty, { min: number; max: number }> = {
  Easy: { min: 1, max: 200 },
  Medium: { min: 200, max: 500 },
  Hard: { min: 500, max: 1000 },
  Expert: { min: 1000, max: 2000 },
};

export interface ProblemPublishState {
  problemId?: string;
  category?: ProblemCategory;
  useContainer: boolean;

  // Step 1 基础信息（platform scope 固定分值；contest_score 在赛事侧配置）。
  title: string;
  author?: string;
  difficulty: Difficulty;
  platformScore: number;
  description: string;
  tags: string[];
  status: 'draft' | 'published' | 'hidden';

  // Step 2 Flag 配置
  flagType: FlagType;
  flagRule: FlagRule;
  /** static + (exact|case_insensitive) → 明文；static + (regex|contains) → pattern；dynamic 时为空 */
  flagValue?: string;
  flagInject: FlagInject;
  flagEnvKey: string;
  flagExample?: string;
  flagSidecarPattern?: string;

  // Step 3 单附件（v2.1）
  attachment?: Attachment | null;

  // Step 4 容器环境
  imageTaskId?: string;
  imageTaskStatus?: 'pending' | 'running' | 'uploaded' | 'extracting' | 'loading' | 'tagging' | 'building' | 'pushing' | 'starting' | 'success' | 'failed';
  imageTaskProgress?: number;
  imageTaskError?: string;
  uploadType?: BuildUploadType;
  hostPortMin?: number;
  hostPortMax?: number;
  registryImage?: string;

  internalPort?: number;
  accessType?: AccessType;
  ingressHostTpl?: string;

  cpuLimit?: number; // millicores
  memoryLimit?: number; // MiB
  ttlSeconds?: number;

  deployMode?: DeployMode;
  /** sidecar 可选：未启用时 sidecar_image / sidecar_port 一并置空。 */
  enableSidecar: boolean;
  sidecarImage?: string;
  sidecarPort?: number;
}

export const initialProblemState: ProblemPublishState = {
  useContainer: true,
  enableSidecar: false,
  title: '',
  difficulty: 'Easy',
  platformScore: 100,
  description: '',
  tags: [],
  status: 'draft',

  flagType: 'static',
  flagRule: 'exact',
  flagInject: 'env',
  flagEnvKey: 'EXEC_FLAG',

  attachment: null,

  cpuLimit: 500,
  memoryLimit: 512,
  ttlSeconds: 3600,
  deployMode: 'per_user',
  accessType: 'ingress',
  uploadType: 'docker_save',
  hostPortMin: 20000,
  hostPortMax: 30000,
};
