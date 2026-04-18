export type ProblemCategory = 'Web' | 'Pwn' | 'Crypto' | 'Reverse' | 'Misc';
export type ProblemType = 'static' | 'container';
export type Difficulty = 'Easy' | 'Medium' | 'Hard' | 'Expert';
export type FlagType = 'static' | 'dynamic';
export type DeployMode = 'shared' | 'per_user';
export type AccessType = 'ingress' | 'nodeport';

export interface ProblemPublishState {
  problemId?: string; // Generated after Step 1
  category?: ProblemCategory;
  type?: ProblemType;
  
  // Basic info (Step 2)
  title: string;
  difficulty: Difficulty;
  baseScore: number;
  isDynamicScore: boolean;
  minScore: number;
  decayDec: number;
  description: string;
  tags: string[];
  status: 'draft' | 'published' | 'hidden';

  // Flag Config (Step 3)
  flagType: FlagType;
  staticFlag?: string; // Used for static, plain text (hashed on submit backend, but frontend can preview hash)
  staticFlagMatchMode?: 'exact' | 'regex' | 'ignore_case';
  dynamicFlagInjection?: 'env' | 'file';
  dynamicFlagEnvName?: string;
  dynamicFlagFilePath?: string;
  dynamicFlagPrefix?: string;

  // Attachments (Step 4)
  attachments: Array<{
    id: string;
    filename: string;
    size: number;
    sha256: string;
    status: 'uploading' | 'success' | 'error';
  }>;

  // Container Env (Step 5)
  imageTaskId?: string;
  imageTaskStatus?: 'uploading' | 'extracting' | 'loading' | 'tagging' | 'pushing' | 'success' | 'failed';
  imageTaskProgress?: number;
  imageTaskError?: string;
  registryImage?: string;
  
  internalPort?: number;
  accessType?: AccessType;
  
  cpuLimit?: number; // millicores
  memoryLimit?: number; // MiB
  
  deployMode?: DeployMode;
  enableSidecar?: boolean;
}

export const initialProblemState: ProblemPublishState = {
  enableSidecar: true,
  title: '',
  difficulty: 'Easy',
  baseScore: 1000,
  isDynamicScore: false,
  minScore: 100,
  decayDec: 10,
  description: '',
  tags: [],
  status: 'draft',
  
  flagType: 'static',
  staticFlagMatchMode: 'exact',
  
  attachments: [],
  
  cpuLimit: 500,
  memoryLimit: 512,
};
