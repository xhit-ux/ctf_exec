export const APP_HTTP_ERROR_EVENT = 'app:http-error'

export type GlobalHttpErrorPayload = {
  status: number
  message: string
  url: string
  method: string
}

export function publishGlobalHttpError(payload: GlobalHttpErrorPayload) {
  window.dispatchEvent(new CustomEvent<GlobalHttpErrorPayload>(APP_HTTP_ERROR_EVENT, {
    detail: payload,
  }))
}
