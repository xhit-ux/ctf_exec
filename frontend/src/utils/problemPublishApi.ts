import { apiUrl } from './api'
import { apiFetch } from './authClient'
import { readSafeErrorMessage } from './error'

export type BuildUploadType = 'docker_save' | 'compose_bundle'

export type BuildTaskView = {
  id: string
  problem_id: string
  file_path: string
  file_size: number
  status: string
  current_step: string
  image_repo?: string
  image_tag?: string
  error_log?: string
}

export type PublishProblemResult = {
  published: boolean
  building: boolean
  message: string
}

async function mustOK(response: Response, fallback: string): Promise<void> {
  if (response.ok) return
  const message = await readSafeErrorMessage(response.clone(), fallback)
  throw new Error(message)
}

export async function createProblemDraft(payload: { category: string; type: 'static' | 'container_web' | 'container_tcp' }): Promise<string> {
  const response = await apiFetch(apiUrl('/api/admin/problems'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  await mustOK(response, '创建题目草稿失败')
  const body = await response.json() as { problem_id?: string }
  if (!body.problem_id) {
    throw new Error('创建题目草稿失败：未返回 problem_id')
  }
  return body.problem_id
}

export async function saveProblemBasic(problemID: string, payload: {
  title: string
  category: string
  type: 'static' | 'container_web' | 'container_tcp'
  description: string
  base_score: number
  difficulty: 'easy' | 'medium' | 'hard' | 'expert'
  deploy_mode: 'shared' | 'per_user'
  use_kata: boolean
}): Promise<void> {
  const response = await apiFetch(apiUrl(`/api/admin/problems/${problemID}/basic`), {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  await mustOK(response, '保存基础信息失败')
}

export async function saveProblemFlag(problemID: string, payload: {
  flag_plain: string
  flag_rule: 'exact' | 'case_insensitive'
  flag_type: 'static' | 'dynamic'
  flag_inject: 'env' | 'sidecar'
  flag_env_key: string
  flag_example?: string
  flag_sidecar_pattern?: string
}): Promise<void> {
  const response = await apiFetch(apiUrl(`/api/admin/problems/${problemID}/flag`), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  await mustOK(response, '保存 Flag 配置失败')
}

export async function saveProblemContainer(problemID: string, payload: {
  image_name: string
  image_tag: string
  challenge_port: number
  sidecar_image: string
  sidecar_port: number
  cpu_limit: string
  memory_limit: string
  env?: unknown
  ttl_seconds: number
  ingress_host_tpl?: string
}): Promise<void> {
  const response = await apiFetch(apiUrl(`/api/admin/problems/${problemID}/container`), {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  await mustOK(response, '保存容器配置失败')
}

export async function uploadProblemImage(problemID: string, payload: {
  file: File
  imageRepo: string
  imageTag?: string
  uploadType: BuildUploadType
  hostPortMin: number
  hostPortMax: number
}): Promise<BuildTaskView> {
  const formData = new FormData()
  formData.append('file', payload.file)
  formData.append('image_repo', payload.imageRepo)
  if (payload.imageTag && payload.imageTag.trim()) {
    formData.append('image_tag', payload.imageTag.trim())
  }
  formData.append('upload_type', payload.uploadType)
  formData.append('host_port_min', String(payload.hostPortMin))
  formData.append('host_port_max', String(payload.hostPortMax))

  const response = await apiFetch(apiUrl(`/api/admin/problems/${problemID}/image`), {
    method: 'POST',
    body: formData,
  })
  await mustOK(response, '上传镜像文件失败')
  const body = await response.json() as { task?: BuildTaskView }
  if (!body.task) {
    throw new Error('上传镜像文件失败：未返回构建任务')
  }
  return body.task
}

export async function fetchProblemBuildTask(problemID: string): Promise<BuildTaskView | null> {
  const response = await apiFetch(apiUrl(`/api/admin/problems/${problemID}/build-task`))
  await mustOK(response, '获取构建任务失败')
  const body = await response.json() as { task?: BuildTaskView | null }
  return body.task ?? null
}

export async function publishProblem(problemID: string): Promise<PublishProblemResult> {
  const response = await apiFetch(apiUrl(`/api/admin/problems/${problemID}/publish`), {
    method: 'POST',
  })
  if (response.status === 202) {
    const body = await response.json() as { message?: string; status?: string }
    return {
      published: false,
      building: true,
      message: body.message || '镜像构建任务已启动',
    }
  }
  await mustOK(response, '发布题目失败')
  const body = await response.json() as { message?: string; status?: string }
  return {
    published: true,
    building: false,
    message: body.message || '题目发布成功',
  }
}

export async function deleteProblemDraft(problemID: string): Promise<void> {
  const response = await apiFetch(apiUrl(`/api/admin/problems/${problemID}`), {
    method: 'DELETE',
  })
  await mustOK(response, '删除草稿失败')
}
