/**
 * 密码强度策略：仅作用于「新设密码」场景（注册 / 重置 / 修改密码）。
 *
 * 与后端 utils.ValidatePasswordStrength 保持一致：
 *   - 长度 [PASSWORD_MIN_LENGTH, PASSWORD_MAX_LENGTH]
 *   - 必须同时包含字母与数字
 *
 * 登录场景不做该校验，避免影响存量弱密码用户登录。
 */

export const PASSWORD_MIN_LENGTH = 8
export const PASSWORD_MAX_LENGTH = 128

export const PASSWORD_POLICY_HINT = `密码长度 ${PASSWORD_MIN_LENGTH}-${PASSWORD_MAX_LENGTH} 位，需同时包含字母与数字`

/**
 * 校验密码是否满足强度要求；不满足时返回错误文案，满足时返回 null。
 */
export function validatePasswordStrength(password: string): string | null {
  if (password.length < PASSWORD_MIN_LENGTH) {
    return `密码长度至少 ${PASSWORD_MIN_LENGTH} 个字符`
  }
  if (password.length > PASSWORD_MAX_LENGTH) {
    return `密码长度不能超过 ${PASSWORD_MAX_LENGTH} 个字符`
  }
  const hasLetter = /[A-Za-z]/.test(password)
  const hasDigit = /\d/.test(password)
  if (!hasLetter || !hasDigit) {
    return '密码须同时包含字母与数字'
  }
  return null
}
