import { ApiClientError } from '../api/client'

export function requestErrorMessage(error: unknown, fallback: string) {
  return error instanceof ApiClientError ? error.message : fallback
}
