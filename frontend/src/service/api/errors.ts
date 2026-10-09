import type { AxiosError } from 'axios';

/**
 * Read the isobuilder envelope out of a failed request. Everything the backend
 * reports is `{code, message, data, details?}`, regardless of business domain.
 */

interface ErrorEnvelope {
  code?: string;
  message?: string;
  details?: unknown;
}

function errorEnvelope(error: unknown): ErrorEnvelope | undefined {
  return (error as AxiosError<ErrorEnvelope>)?.response?.data;
}

/** the message to show to the operator for a failed request */
export function getApiErrorMessage(error: unknown, fallback = '请求失败，请稍后重试'): string {
  return errorEnvelope(error)?.message || (error as AxiosError)?.message || fallback;
}

/** the stable biz code of a failed request (e.g. `storage_unreachable`, `invalid_hosts`) */
export function getApiErrorCode(error: unknown): string {
  return errorEnvelope(error)?.code || '';
}

/** structured details of a failed request (e.g. `received_bytes`, offending host rows) */
export function getApiErrorDetails<T = Record<string, unknown>>(error: unknown): T | null {
  return (errorEnvelope(error)?.details as T) ?? null;
}

/** the HTTP status of a failed request (`0` when it never left the client) */
export function getApiErrorStatus(error: unknown): number {
  return (error as AxiosError)?.response?.status ?? 0;
}
