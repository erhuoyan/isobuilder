import { request } from '../request';

/** get all base isos, ordered by created_at desc */
export function fetchGetIsos() {
  return request<{ isos: Api.IsoBuilder.Iso[] }>({ url: '/api/isos' });
}

/** delete an iso record and its file */
export function fetchDeleteIso(id: string) {
  return request<null>({ url: `/api/isos/${id}`, method: 'delete' });
}

/**
 * Register an iso the server fetches itself.
 *
 * `source: "path"` adopts a file already on the server disk; `source: "url"` makes the server
 * download an HTTP(S) link (object storage links included). The browser never uploads an image:
 * a 14 GB file would take hours through the operator's uplink. The drawer reports the failures
 * itself, so the global toast is off.
 */
export function fetchRegisterIso(payload: Api.IsoBuilder.RegisterIsoPayload) {
  return request<{ iso: Api.IsoBuilder.Iso }>({
    url: '/api/isos/register',
    method: 'post',
    data: payload,
    silentError: true
  });
}
