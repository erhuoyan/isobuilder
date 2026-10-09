import type { AxiosProgressEvent } from 'axios';
import { request } from '../request';

/** get all base isos, ordered by created_at desc */
export function fetchGetIsos() {
  return request<{ isos: Api.IsoBuilder.Iso[] }>({ url: '/api/isos' });
}

/** delete an iso record and its file */
export function fetchDeleteIso(id: string) {
  return request<null>({ url: `/api/isos/${id}`, method: 'delete' });
}

/** start a chunked upload */
export function fetchCreateIsoUpload(payload: { name: string; filename: string; arch: Api.IsoBuilder.Arch; size_bytes: number }) {
  return request<Api.IsoBuilder.UploadSession>({ url: '/api/isos/uploads', method: 'post', data: payload });
}

/**
 * Append one chunk to the upload
 *
 * The chunks must be sent strictly in order, the backend replies `409 offset_mismatch` when the
 * offset does not match the received bytes.
 */
export function fetchUploadIsoChunk(
  uploadId: string,
  offset: number,
  chunk: Blob,
  options: { signal?: AbortSignal; onProgress?: (event: AxiosProgressEvent) => void } = {}
) {
  return request<{ received_bytes: number; complete: boolean }>({
    url: `/api/isos/uploads/${uploadId}`,
    method: 'put',
    params: { offset },
    data: chunk,
    headers: { 'Content-Type': 'application/octet-stream' },
    signal: options.signal,
    timeout: 0,
    onUploadProgress: options.onProgress,
    // the upload drawer recovers from `offset_mismatch` itself and reports the other failures
    silentError: true
  });
}

/** finish the upload, the backend writes the file and computes the sha256 asynchronously */
export function fetchFinishIsoUpload(uploadId: string) {
  return request<{ iso: Api.IsoBuilder.Iso }>({ url: `/api/isos/uploads/${uploadId}/finish`, method: 'post' });
}

/** cancel the upload and remove the temporary file */
export function fetchCancelIsoUpload(uploadId: string) {
  return request<null>({ url: `/api/isos/uploads/${uploadId}`, method: 'delete' });
}

/**
 * Register an iso which is already reachable from the server
 *
 * `source: "path"` adopts a file on the server disk, `source: "s3"` pulls an object out of an
 * S3-compatible bucket. The drawer reports the failures itself, so the global toast is off.
 */
export function fetchRegisterIso(payload: Api.IsoBuilder.RegisterIsoPayload) {
  return request<{ iso: Api.IsoBuilder.Iso }>({
    url: '/api/isos/register',
    method: 'post',
    data: payload,
    silentError: true
  });
}
