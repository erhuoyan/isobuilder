import { computed, ref, shallowRef } from 'vue';
import { defineStore } from 'pinia';
import { fetchGetConfig } from '@/service/api';
import { SetupStoreId } from '@/enum';

/**
 * The runtime config of the isobuilder backend.
 *
 * `GET /api/config` never requires authentication and tells the frontend whether a token is needed,
 * so it is fetched before the first navigation is resolved.
 */
export const useConfigStore = defineStore(SetupStoreId.Config, () => {
  const config = shallowRef<Api.IsoBuilder.Config | null>(null);
  const loading = ref(false);
  const failed = ref(false);

  /** whether the backend requires a token */
  const authRequired = computed(() => Boolean(config.value?.auth_required));

  let pending: Promise<void> | null = null;

  async function loadConfig() {
    loading.value = true;
    failed.value = false;

    const { data, error } = await fetchGetConfig();

    if (!error && data) {
      config.value = data;
    } else {
      failed.value = true;
    }

    loading.value = false;
  }

  /** load the config once, concurrent callers share the same request */
  async function ensureConfig() {
    if (config.value) return;

    if (!pending) {
      pending = loadConfig().finally(() => {
        pending = null;
      });
    }

    await pending;
  }

  /** reload the config, used by the settings page */
  async function refreshConfig() {
    await loadConfig();
  }

  /** the maximum uploaded size of an iso, 0 when the config is not loaded */
  const maxUploadBytes = computed(() => config.value?.max_upload_bytes || 0);

  return {
    config,
    loading,
    failed,
    authRequired,
    maxUploadBytes,
    ensureConfig,
    refreshConfig
  };
});
