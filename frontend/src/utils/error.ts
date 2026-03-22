export function genericErrorMessageByStatus(status: number, fallback = '操作失败，请稍后重试'): string {
  switch (status) {
    case 400:
      return '请求参数有误'
    case 401:
      return '请先登录后重试'
    case 403:
      return '当前操作未授权'
    case 404:
      return '请求资源不存在'
    case 409:
      return '请求冲突，请稍后重试'
    case 429:
      return '请求过于频繁，请稍后再试'
    default:
      if (status >= 500) return '服务器开小差了，请稍后再试'
      return fallback
  }
}

export async function readSafeErrorMessage(response: Response, fallback = '操作失败，请稍后重试'): Promise<string> {
  const defaultMessage = genericErrorMessageByStatus(response.status, fallback)

  // 仅在 4xx 场景读取后端业务提示，5xx 一律走泛化文案。
  if (response.status < 400 || response.status >= 500) {
    return defaultMessage
  }

  try {
    const data = await response.json() as { error?: string; message?: string }
    const msg = (data?.error || data?.message || '').trim()
    if (!msg) return defaultMessage

    // 控制长度，避免意外回显过长信息。
    return msg.slice(0, 120)
  } catch {
    return defaultMessage
  }
}

export function fromUnknownError(_err: unknown, fallback = '操作失败，请稍后重试'): string {
  return fallback
}
