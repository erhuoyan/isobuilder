<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { fetchGetSettings, fetchUpdateSettings } from '@/service/api';
import { getApiErrorCode, getApiErrorMessage } from '@/service/api/errors';
import { useConfigStore } from '@/store/modules/config';

const configStore = useConfigStore();

const appVersion = APP_VERSION;
const buildTime = BUILD_TIME;

const loading = ref(false);
const saving = ref(false);

/** the write-only secret inputs, they are cleared after every load */
const form = ref({
  endpoint: '',
  region: '',
  bucket: '',
  prefix: '',
  path_style: false,
  insecure_tls: false,
  public_base: '',
  presign_ttl_seconds: 86400 as number | null,
  access_key: '',
  secret_key: ''
});

/** the read-only status reported by the backend */
const source = ref<Api.IsoBuilder.StorageSettingsSource>('env');
const storageRaw = ref(false);
const enabled = ref(false);
const accessKeySet = ref(false);
const secretKeySet = ref(false);

/** the settings were saved but the bucket probe failed, it is not a save failure */
const unreachableMessage = ref('');
/** an inline validation / request error, shown next to the form instead of a toast */
const settingsError = ref<{ title: string; message: string } | null>(null);

const sourceTag = computed(() => {
  if (source.value === 'db') {
    return { label: '配置页已保存', type: 'info' as const };
  }

  return { label: '仅环境变量', type: 'default' as const };
});

const accessKeyPlaceholder = computed(() => (accessKeySet.value ? '已配置，留空不修改' : '未配置，请输入'));
const secretKeyPlaceholder = computed(() => (secretKeySet.value ? '已配置，留空不修改' : '未配置，请输入'));

async function loadSettings() {
  loading.value = true;

  const { data, error } = await fetchGetSettings();

  loading.value = false;

  if (error || !data) return;

  const { storage } = data;

  storageRaw.value = data.storage_raw;
  source.value = storage.source;
  enabled.value = storage.enabled;
  accessKeySet.value = storage.access_key_set;
  secretKeySet.value = storage.secret_key_set;
  form.value = {
    endpoint: storage.endpoint,
    region: storage.region,
    bucket: storage.bucket,
    prefix: storage.prefix,
    path_style: storage.path_style,
    insecure_tls: storage.insecure_tls,
    public_base: storage.public_base,
    presign_ttl_seconds: storage.presign_ttl_seconds,
    // the backend never returns the secrets, so the boxes always start empty
    access_key: '',
    secret_key: ''
  };
}

async function handleSave() {
  if (saving.value) return;

  unreachableMessage.value = '';
  settingsError.value = null;

  // Secrets are write-only. Only a non-empty box is sent, so leaving it blank keeps the stored
  // key untouched; the backend also treats an empty string as "keep", but omitting the field is
  // the strongest guarantee. Typing a value overwrites the key.
  const payload: Api.IsoBuilder.StorageSettingsPayload = {
    endpoint: form.value.endpoint.trim(),
    region: form.value.region.trim(),
    bucket: form.value.bucket.trim(),
    prefix: form.value.prefix.trim(),
    path_style: form.value.path_style,
    insecure_tls: form.value.insecure_tls,
    public_base: form.value.public_base.trim(),
    presign_ttl_seconds: form.value.presign_ttl_seconds ?? 0
  };

  if (form.value.access_key) payload.access_key = form.value.access_key;
  if (form.value.secret_key) payload.secret_key = form.value.secret_key;

  saving.value = true;

  const { error } = await fetchUpdateSettings({ storage: payload });

  saving.value = false;

  if (!error) {
    window.$message?.success('已保存并生效');
    await loadSettings();
    await configStore.refreshConfig();
    return;
  }

  const code = getApiErrorCode(error);
  const message = getApiErrorMessage(error);

  if (code === 'storage_unreachable') {
    // the settings were persisted and activated, only the HeadBucket probe failed
    unreachableMessage.value = message;
    await loadSettings();
    await configStore.refreshConfig();
    return;
  }

  // `invalid_settings` and every other failure are shown in place: the user is still editing the
  // form, so a transient toast would be the wrong place for a field level message
  settingsError.value = {
    title: code === 'invalid_settings' ? '配置校验未通过' : '保存失败',
    message
  };
}

onMounted(() => {
  loadSettings();
});
</script>

<template>
  <div class="flex-col-stretch gap-4">
    <NCard :bordered="false" class="card-wrapper" size="small">
    <div class="mb-4 flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 class="text-18px font-500">系统配置</h2>
        <p class="mt-1 text-13px text-gray-500">维护对象存储（S3 兼容）配置，保存后立即生效，无需重启服务。</p>
      </div>
      <NSpace :size="8" align="center" :wrap="true">
        <NTag :type="sourceTag.type" size="small" :bordered="false">{{ sourceTag.label }}</NTag>
        <NTag :type="enabled ? 'success' : 'default'" size="small" :bordered="false">
          {{ enabled ? '对象存储已启用' : '对象存储已关闭' }}
        </NTag>
        <NButton :loading="loading" :disabled="saving" @click="loadSettings()">
          <template #icon>
            <SvgIcon icon="mdi:refresh" />
          </template>
          刷新
        </NButton>
      </NSpace>
    </div>

    <NAlert class="mb-3" type="default" :bordered="false" :show-icon="false">
      <span>
        <span class="font-500">bucket 留空并保存</span>
        即关闭对象存储：构建产物将留在服务器本地，由后端直传下载。
      </span>
      <span v-if="storageRaw" class="ml-1 text-gray-500">当前配置来自配置页并已持久化。</span>
    </NAlert>

    <NAlert
      v-if="unreachableMessage"
      class="mb-3"
      type="error"
      :bordered="false"
      :show-icon="true"
      title="配置已保存，但对象存储连接失败"
    >
      <div class="break-all">{{ unreachableMessage }}</div>
      <div class="mt-1 text-12px text-gray-500">
        配置已写入并生效；保存时的连通性探测（HeadBucket）失败，请检查 endpoint / bucket / 凭据或网络后再次保存。
      </div>
    </NAlert>

    <NAlert v-if="settingsError" class="mb-3" type="warning" :bordered="false" :show-icon="true" :title="settingsError.title">
      {{ settingsError.message }}
    </NAlert>

    <n-form :model="form" :label-width="150" label-placement="left" size="medium" class="py-2">
      
        <n-form-item label="Endpoint" path="endpoint" >
          <NInput
            v-model:value="form.endpoint"
            :input-props="{ class: 'font-mono', spellcheck: 'false', autocomplete: 'off' }"
            placeholder="https://oss.example.com"
          />
        </n-form-item>
        <n-form-item label="Region" path="region">
          <NInput
            v-model:value="form.region"
            :input-props="{ class: 'font-mono', spellcheck: 'false', autocomplete: 'off' }"
            placeholder="us-east-1（可选）"
          />
        </n-form-item>
        <n-form-item label="Bucket" path="bucket">
          <NInput
            v-model:value="form.bucket"
            :input-props="{ class: 'font-mono', spellcheck: 'false', autocomplete: 'off' }"
            placeholder="留空 = 关闭对象存储"
          />
        </n-form-item>
        <n-form-item label="Prefix" path="prefix">
          <NInput
            v-model:value="form.prefix"
            :input-props="{ class: 'font-mono', spellcheck: 'false', autocomplete: 'off' }"
            placeholder="iso-builder（可选）"
          />
        </n-form-item>
        <n-form-item label="Public base" path="public_base">
          <NInput
            v-model:value="form.public_base"
            :input-props="{ class: 'font-mono', spellcheck: 'false', autocomplete: 'off' }"
            placeholder="CDN 或对外访问前缀（可选）"
          />
        </n-form-item>
        <n-form-item label="预签名有效期（秒）" path="presign_ttl_seconds">
          <NInputNumber v-model:value="form.presign_ttl_seconds" class="w-full" :min="60" :step="3600">
            <template #suffix>秒</template>
          </NInputNumber>
        </n-form-item>
        <n-form-item label="Path style" path="path_style">
          <div class="flex flex-wrap items-center gap-2">
            <NSwitch v-model:value="form.path_style" />
            <span class="text-13px text-gray-500">MinIO 等自建服务通常需要开启</span>
          </div>
        </n-form-item>
        <n-form-item label="跳过 TLS 校验" path="insecure_tls">
          <div class="flex flex-wrap items-center gap-2">
            <NSwitch v-model:value="form.insecure_tls" />
            <span class="text-13px text-gray-500">自签名证书时使用</span>
          </div>
        </n-form-item>
        <n-form-item label="Access Key" path="access_key">
          <NInput
            v-model:value="form.access_key"
            type="password"
            show-password-on="click"
            :placeholder="accessKeyPlaceholder"
            :input-props="{ spellcheck: 'false', autocomplete: 'new-password' }"
          />
        </n-form-item>
        <n-form-item label="Secret Key" path="secret_key">
          <NInput
            v-model:value="form.secret_key"
            type="password"
            show-password-on="click"
            :placeholder="secretKeyPlaceholder"
            :input-props="{ spellcheck: 'false', autocomplete: 'new-password' }"
          />
        </n-form-item>
      
      <div class="flex flex-wrap items-center gap-3">
        <NButton type="primary" :loading="saving" @click="handleSave">保存</NButton>
        <span class="text-13px text-gray-500">保存时会自动探测对象存储连通性（即测试连接），失败不影响配置留存。</span>
      </div>
    </n-form>
    </NCard>

    <NCard :bordered="false" class="card-wrapper" size="small">
      <h2 class="mb-3 text-18px font-500">版本信息</h2>
      <n-descriptions label-placement="left" :column="1" bordered size="small">
        <n-descriptions-item label="前端版本">{{ appVersion }}</n-descriptions-item>
        <n-descriptions-item label="前端构建时间">{{ buildTime }}</n-descriptions-item>
        <n-descriptions-item label="后端版本">{{ configStore.config?.version ?? '-' }}</n-descriptions-item>
        <n-descriptions-item label="后端 Git Commit">{{ configStore.config?.git_commit ?? '-' }}</n-descriptions-item>
        <n-descriptions-item label="后端构建时间">{{ configStore.config?.build_time ?? '-' }}</n-descriptions-item>
        <n-descriptions-item label="Go 版本">{{ configStore.config?.go_version ?? '-' }}</n-descriptions-item>
      </n-descriptions>
    </NCard>
  </div>
</template>

<style scoped></style>
