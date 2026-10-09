import { request } from '../request';

/** get the effective object storage settings, the secrets are masked */
export function fetchGetSettings() {
  return request<Api.IsoBuilder.Settings>({ url: '/api/settings' });
}

/**
 * Update the object storage settings, they take effect immediately.
 *
 * The backend probes the bucket before answering, so a `storage_unreachable` error means the
 * settings have been saved but the connection failed. The settings page reports that case itself.
 */
export function fetchUpdateSettings(payload: Api.IsoBuilder.SettingsPayload) {
  return request<{ storage: Api.IsoBuilder.StorageSettings }>({
    url: '/api/settings',
    method: 'put',
    data: payload,
    silentError: true
  });
}
