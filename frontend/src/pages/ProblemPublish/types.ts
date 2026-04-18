export type ProblemCategory = 'Web' | 'Pwn' | 'Crypto' | 'Reverse' | 'Misc';
export type Difficulty = 'Easy' | 'Medium' | 'Hard' | 'Expert';
export type FlagType = 'static' | 'dynamic';
export type DeployMode = 'shared' | 'per_user';
export type AccessType = 'ingress' | 'nodeport';

export interface Attachment {
  id: string;
  filename: string;
  size: number;
  sha256: string;
  status: 'uploading' | 'success' | 'error';
}

export interface ProblemPublishState {
  problemId?: string;
  category?: ProblemCategory;
  useContainer: boolean;
  
  // Step 2 Basic Info
  title: string;
  difficulty: Difficulty;
  baseScore: number;
  isDynamicScore: boolean;
  minScore: number;
  decayDec: number; // How much it decreases per solve relative (or fixed step)
  description: string;
  tags: string[];
  status: 'draft' | 'published' | 'hidden';

  // Step 3 Flag Config
  flagType: FlagType;
  staticFlag?: string; // used when static
  staticFlagMatchMode?: 'exact' | 'regex' | 'ignore_case';
  dynamicFlagInjection?: 'env' | 'file';
  dynamicFlagEnvName?: string;
  dynamicFlagFilePath?: string;
  dynamicFlagPrefix?: string;

  // Step 4 Attachments
  attachments: Attachment[];

  // Step 5 Container Env
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
  useContainer: true,
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
  deployMode: 'per_user'
};
