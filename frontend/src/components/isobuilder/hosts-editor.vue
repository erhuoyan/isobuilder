<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useDebounceFn } from '@vueuse/core';
import { fetchValidateHosts } from '@/service/api';
import { hostsFormatOptions } from '@/constants/isobuilder';

/** the idle time before the inventory is validated automatically */
const VALIDATE_DEBOUNCE = 500;

const PLACEHOLDER = `hosts:
  - sn: SN0000000
    hostname: node-01
    network:
      ip: 172.16.0.11
      netmask: 255.255.255.0
      gateway: 172.16.0.1
      mac: 00:11:22:33:44:55
    bmc_ip: 10.0.0.11
  - sn: SN0000001
    hostname: node-02
    network:
      ip: 172.16.0.12
      prefix: 24
      bond:
        mode: 4
        slaves:
          - 00:11:22:33:44:66
          - 00:11:22:33:44:67`;

/** the raw inventory text, YAML or JSON */
const value = defineModel<string>('value', { required: true });
/** the format sent to the backend, `auto` lets the backend detect it */
const format = defineModel<Api.IsoBuilder.HostsFormatInput>('format', { required: true });

const emit = defineEmits<{
  validate: [result: Api.IsoBuilder.ValidateHostsResult | null];
}>();

const result = ref<Api.IsoBuilder.ValidateHostsResult | null>(null);
const validating = ref(false);
/** the `format + content` pair of the last successful validation, empty when stale */
const validatedKey = ref('');
/** the last validation request failed, the input has not been checked at all */
const requestFailed = ref(false);
const fileInputRef = ref<HTMLInputElement | null>(null);

/** the sequence of the newest validation request, the stale responses are dropped */
let requestSeq = 0;

const hasContent = computed(() => value.value.trim().length > 0);

const currentKey = computed(() => `${format.value}\u0000${value.value}`);

/** the input has been edited since the last validation, the shown result is outdated */
const stale = computed(() => hasContent.value && !validating.value && currentKey.value !== validatedKey.value);

const errorCount = computed(() => result.value?.errors.length ?? 0);

/** invalidate the current result, the wizard must not save an unvalidated inventory */
function invalidate() {
  requestSeq += 1;
  validatedKey.value = '';
  validating.value = false;
  requestFailed.value = false;
  emit('validate', null);
}

async function runValidate() {
  const content = value.value;

  if (!content.trim()) {
    result.value = null;
    invalidate();
    return;
  }

  const key = currentKey.value;

  // the content has already been validated, reuse the result instead of hitting the backend again
  if (key === validatedKey.value && result.value) {
    emit('validate', result.value);
    return;
  }

  const seq = ++requestSeq;
  validating.value = true;

  const { data, error } = await fetchValidateHosts(content, format.value);

  // a newer request has been fired meanwhile, drop this response
  if (seq !== requestSeq) return;

  validating.value = false;

  if (error) {
    validatedKey.value = '';
    result.value = null;
    requestFailed.value = true;
    emit('validate', null);
    return;
  }

  validatedKey.value = key;
  result.value = data;
  emit('validate', data);
}

const debouncedValidate = useDebounceFn(runValidate, VALIDATE_DEBOUNCE);

watch(
  [value, format],
  () => {
    if (!hasContent.value) {
      result.value = null;
      invalidate();
      return;
    }

    invalidate();
    debouncedValidate();
  },
  { immediate: true }
);

function triggerFileSelect() {
  fileInputRef.value?.click();
}

async function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];

  // reset the input so that the same file can be selected twice in a row
  input.value = '';

  if (!file) return;

  try {
    value.value = await file.text();
    window.$message?.success(`已读入 ${file.name}`);
  } catch {
    window.$message?.error('读取本地文件失败，请重试。');
  }
}

function handleClear() {
  value.value = '';
}

/** the label of one error, an inventory level error carries an empty field */
function hostLabel(error: Api.IsoBuilder.HostError) {
  return error.index >= 0 ? `第 ${error.index + 1} 台` : '清单级';
}
</script>

<template>
  <div class="flex flex-col gap-12px">
    <div class="flex flex-wrap items-center justify-between gap-8px">
      <NRadioGroup v-model:value="format" size="small">
        <NRadioButton v-for="option in hostsFormatOptions" :key="option.value" :value="option.value">
          {{ option.label }}
        </NRadioButton>
      </NRadioGroup>

      <NSpace :size="8" :wrap="true">
        <NButton size="small" @click="triggerFileSelect">
          <template #icon>
            <SvgIcon icon="mdi:file-upload-outline" />
          </template>
          读入本地文件
        </NButton>
        <NButton size="small" :disabled="!hasContent" @click="handleClear">
          <template #icon>
            <SvgIcon icon="mdi:eraser" />
          </template>
          清空
        </NButton>
        <NButton size="small" type="primary" :loading="validating" :disabled="!hasContent" @click="runValidate">
          <template #icon>
            <SvgIcon icon="mdi:check-decagram-outline" />
          </template>
          立即校验
        </NButton>
      </NSpace>
    </div>

    <input
      ref="fileInputRef"
      type="file"
      accept=".yaml,.yml,.json,.txt"
      class="hidden"
      @change="handleFileChange"
    />

    <NInput
      v-model:value="value"
      type="textarea"
      :rows="14"
      class="font-mono"
      :input-props="{ class: 'font-mono', spellcheck: 'false' }"
      :placeholder="PLACEHOLDER"
    />

    <NAlert v-if="!hasContent" size="small" type="default" :bordered="false" :show-icon="false">
      粘贴或读入主机清单（YAML / JSON），停止输入 500 毫秒后自动校验。
    </NAlert>

    <NAlert v-else-if="validating" size="small" type="info" :bordered="false">
      正在校验主机清单…
    </NAlert>

    <NAlert v-else-if="requestFailed" size="small" type="error" :bordered="false">
      校验请求失败，请检查清单格式后重试。
    </NAlert>

    <NAlert v-else-if="stale" size="small" type="warning" :bordered="false">
      输入已改动，等待自动校验完成后再进入下一步。
    </NAlert>

    <NAlert v-else-if="result" size="small" :type="result.ok ? 'success' : 'error'" :bordered="false">
      <span class="font-mono">{{ result.hosts_count }}</span> 台主机 · 识别格式
      <span class="font-mono">{{ result.format.toUpperCase() }}</span> ·
      {{ result.ok ? '校验通过' : `校验未通过，共 ${errorCount} 处问题` }}
    </NAlert>

    <NAlert v-else size="small" type="warning" :bordered="false">
      主机清单尚未校验。
    </NAlert>

    <NList v-if="result && errorCount" bordered size="small">
      <NListItem v-for="(error, index) in result.errors" :key="`${error.index}-${error.field}-${index}`">
        <div class="flex flex-wrap items-center gap-8px">
          <span class="error-text font-mono font-500">{{ hostLabel(error) }}</span>
          <code v-if="error.field" class="error-text font-mono text-12px">{{ error.field }}</code>
          <span class="error-text">{{ error.message }}</span>
        </div>
      </NListItem>
    </NList>

    <div v-if="result && result.hosts_summary.length" class="flex flex-col gap-8px">
      <div class="text-13px text-gray-500">
        解析结果（共 <span class="font-mono">{{ result.hosts_summary.length }}</span> 台）
      </div>
      <HostSummaryTable :hosts="result.hosts_summary" :errors="result.errors" :loading="validating" :max-height="360" />
    </div>
  </div>
</template>

<style lang="scss" scoped>
.error-text {
  color: rgb(var(--error-color));
}
</style>
