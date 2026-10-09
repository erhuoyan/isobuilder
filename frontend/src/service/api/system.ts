import { request } from '../request';

/** get the backend config, it does not require authentication */
export function fetchGetConfig() {
  return request<Api.IsoBuilder.Config>({ url: '/api/config' });
}

/** get the backend health, it does not require authentication */
export function fetchGetHealth() {
  return request<Api.IsoBuilder.Health>({ url: '/api/health' });
}
