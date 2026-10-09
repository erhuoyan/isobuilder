<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import type { FormInst, FormRules } from 'naive-ui';
import { useIntervalFn } from '@vueuse/core';
import {
  fetchCancelIsoUpload,
  fetchCreateIsoUpload,
  fetchFinishIsoUpload,
  fetchRegisterIso,
  fetchUploadIsoChunk
} from '@/service/api';
import { getApiErrorCode, getApiErrorDetails, getApiErrorMessage } from '@/service/api/errors';
import { useConfigStore } from '@/store/modules/config';
import { useRouterPush } from '@/hooks/common/router';
import { archOptions } from '@/constants/isobuilder';
import { formatBytes, formatSpeed } from '@/utils/format';

interface Props {
  /** whether the drawer is visible */
  show: boolean;
  /** which way of adding an image the drawer opens on */
  mode?: 'upload' | 's3';
}

const props = withDefaults(defineProps<Props>(), { mode: 'upload' });

const emit = defineEmits<{
  'update:show': [value: boolean];
  finished: [];
}>();

const { routerPushByKey } = useRouterPush();

/** how often the elapsed time is refreshed while uploading */
const TICK_INTERVAL = 500;
/** the sliding window used to estimate the throughput */
const SPEED_WINDOW = 3000;
/** the shortest window which yields a meaningful speed */
const SPEED_MIN_SPAN = 500;
/** the minimum delay between two speed samples */
const SPEED_SAMPLE_GAP = 200;

const configStore = useConfigStore();

/** which entry the drawer is showing: browser chunked upload or an s3 pull */
const activeTab = ref<'upload' | 's3'>(props.mode);

// the parent decides the tab when it opens the drawer
watch(
  () => props.show,
  visible => {
    if (visible) activeTab.value = props.mode;
  }
);

/** the upload lifecycle: idle → uploading → finishing → idle */
const phase = ref<'idle' | 'uploading' | 'finishing'>('idle');
const formRef = ref<FormInst | null>(null);
const fileInputRef = ref<HTMLInputElement | null>(null);

const form = ref({
  name: '',
  arch: 'x86_64' as Api.IsoBuilder.Arch
});

const rules: FormRules = {
  name: { required: true, message: '请输入镜像名称', trigger: ['input', 'blur'] }
};

/** the selected file, it is only sliced, never read into memory as a whole */
const file = ref<File | null>(null);
const uploadId = ref('');
const chunkSize = ref(0);
/** the offset of the chunk which is being sent right now */
const offset = ref(0);
const uploadedBytes = ref(0);
const totalBytes = ref(0);
const speed = ref(0);
const elapsedMs = ref(0);
const errorMessage = ref('');
const abortController = ref<AbortController | null>(null);

/** the wall clock of the running upload, only read by the timer */
let startedAt = 0;
/** the upload has been canceled, a pending chunk request must not report a failure */
let canceled = false;
/** the name derived from the selected file, it follows the file until the user edits the name */
let derivedName = '';

const busy = computed(() => phase.value !== 'idle');
const percent = computed(() => (totalBytes.value ? Math.min(100, (uploadedBytes.value / totalBytes.value) * 100) : 0));
const chunkCount = computed(() => (chunkSize.value ? Math.ceil(totalBytes.value / chunkSize.value) : 0));
const currentChunk = computed(() =>
  chunkSize.value ? Math.min(chunkCount.value, Math.floor(offset.value / chunkSize.value) + 1) : 0
);
/** every byte has reached the backend, only the finish call is left */
const allSent = computed(() => totalBytes.value > 0 && uploadedBytes.value >= totalBytes.value && Boolean(uploadId.value));
const submitText = computed(() => {
  if (phase.value === 'finishing') return '正在收尾';

  return allSent.value ? '完成上传' : '开始上传';
});
const elapsed = computed(() => formatClock(elapsedMs.value));

interface SpeedSample {
  time: number;
  bytes: number;
}

const samples: SpeedSample[] = [];
let lastSampleAt = 0;

/** the timer only drives the elapsed time and the decay of a stalled speed */
const { pause: pauseTimer, resume: resumeTimer } = useIntervalFn(
  () => {
    elapsedMs.value = Date.now() - startedAt;

    if (Date.now() - lastSampleAt > SPEED_WINDOW) {
      speed.value = 0;
    }
  },
  TICK_INTERVAL,
  { immediate: false }
);

function formatClock(ms: number) {
  const seconds = Math.max(0, Math.round(ms / 1000));
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const rest = String(seconds % 60).padStart(2, '0');

  if (hours) return `${hours}:${String(minutes).padStart(2, '0')}:${rest}`;

  return `${String(minutes).padStart(2, '0')}:${rest}`;
}

function resetSpeed() {
  samples.length = 0;
  lastSampleAt = 0;
  speed.value = 0;
}

/** estimate the throughput from a sliding window of `time + uploaded bytes` samples */
function trackSpeed(bytes: number) {
  const now = Date.now();

  if (lastSampleAt && now - lastSampleAt < SPEED_SAMPLE_GAP) return;

  lastSampleAt = now;
  samples.push({ time: now, bytes });

  while (samples.length > 2 && now - samples[0].time > SPEED_WINDOW) {
    samples.shift();
  }

  const span = now - samples[0].time;

  if (span >= SPEED_MIN_SPAN) {
    speed.value = Math.max(0, ((bytes - samples[0].bytes) / span) * 1000);
  }
}

function triggerSelect() {
  fileInputRef.value?.click();
}

function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement;
  const selected = input.files?.[0] ?? null;

  // reset the input so that the same file can be picked twice in a row
  input.value = '';

  if (!selected) return;

  const { maxUploadBytes } = configStore;

  if (maxUploadBytes && selected.size > maxUploadBytes) {
    window.$message?.error(`文件大小 ${formatBytes(selected.size)} 超出上限 ${formatBytes(maxUploadBytes)}。`);
    return;
  }

  const nextDerived = selected.name.replace(/\.[^.]+$/, '');

  if (!form.value.name.trim() || form.value.name === derivedName) {
    form.value.name = nextDerived;
  }

  derivedName = nextDerived;
  file.value = selected;
  errorMessage.value = '';
}

function handleClearFile() {
  file.value = null;
  derivedName = '';
  form.value.name = '';
  errorMessage.value = '';
}

/** the user either wants to cancel a running upload or close the drawer */
function handleUpdateShow(value: boolean) {
  if (!value && busy.value) {
    window.$message?.warning('上传进行中，请先点击「取消上传」。');
    return;
  }

  if (!value && s3Submitting.value) {
    window.$message?.warning('正在提交拉取请求，请稍候。');
    return;
  }

  emit('update:show', value);
}

/** drop the temporary file of an unfinished session, so the backend does not keep it around */
async function discardSession() {
  const id = uploadId.value;

  uploadId.value = '';

  if (!id) return;

  await fetchCancelIsoUpload(id);
}

function fail(message: string) {
  errorMessage.value = message;
  pauseTimer();
  speed.value = 0;
  phase.value = 'idle';
}

async function finishUpload() {
  phase.value = 'finishing';
  pauseTimer();
  speed.value = 0;
  elapsedMs.value = Date.now() - startedAt;

  const { error } = await fetchFinishIsoUpload(uploadId.value);

  if (error) {
    errorMessage.value = getApiErrorMessage(error);
    phase.value = 'idle';
    return;
  }

  uploadId.value = '';
  uploadedBytes.value = totalBytes.value;
  phase.value = 'idle';
  window.$message?.success('上传完成，后台正在计算 SHA256 校验值。');
  emit('finished');
  emit('update:show', false);
}

/** send the file chunk by chunk, resuming from the offset the backend reports */
async function sendChunks(selected: File) {
  let nextOffset = 0;

  while (nextOffset < selected.size) {
    const baseOffset = nextOffset;
    const chunk = selected.slice(baseOffset, Math.min(baseOffset + chunkSize.value, selected.size));
    const controller = new AbortController();

    abortController.value = controller;
    offset.value = baseOffset;

    const { data, error } = await fetchUploadIsoChunk(uploadId.value, baseOffset, chunk, {
      signal: controller.signal,
      onProgress: event => {
        uploadedBytes.value = baseOffset + Math.min(event.loaded, chunk.size);
        trackSpeed(uploadedBytes.value);
      }
    });

    abortController.value = null;

    if (canceled) return;

    if (error) {
      if (getApiErrorCode(error) === 'offset_mismatch') {
        const received = getApiErrorDetails<{ received_bytes: number }>(error)?.received_bytes;

        // the backend owns the byte count, continue exactly from it (breakpoint resume)
        if (typeof received === 'number' && received >= 0 && received !== baseOffset) {
          nextOffset = Math.min(received, selected.size);
          uploadedBytes.value = Math.max(uploadedBytes.value, nextOffset);
          continue;
        }

        fail('上传偏移与后端不一致，请重新开始上传。');
        return;
      }

      fail(getApiErrorMessage(error));
      return;
    }

    const received = data?.received_bytes ?? 0;

    if (received <= baseOffset) {
      fail('后端未确认已接收的字节数，上传已中止。');
      return;
    }

    uploadedBytes.value = Math.max(uploadedBytes.value, Math.min(received, selected.size));
    nextOffset = received;
  }

  uploadedBytes.value = selected.size;
  await finishUpload();
}

async function handleStart() {
  if (busy.value) return;

  if (allSent.value) {
    await finishUpload();
    return;
  }

  const selected = file.value;

  if (!selected) {
    window.$message?.warning('请先选择要上传的 ISO 文件。');
    return;
  }

  try {
    await formRef.value?.validate();
  } catch {
    return;
  }

  errorMessage.value = '';
  canceled = false;
  uploadedBytes.value = 0;
  totalBytes.value = selected.size;
  offset.value = 0;
  elapsedMs.value = 0;
  resetSpeed();

  // a previous attempt may have left a session behind
  await discardSession();

  const { data, error } = await fetchCreateIsoUpload({
    name: form.value.name.trim(),
    filename: selected.name,
    arch: form.value.arch,
    size_bytes: selected.size
  });

  if (error || !data) return;

  uploadId.value = data.upload_id;
  chunkSize.value = data.chunk_size;

  if (!chunkSize.value || chunkSize.value <= 0) {
    fail('后端未返回有效的分片大小，上传已中止。');
    return;
  }

  startedAt = Date.now();
  phase.value = 'uploading';
  resumeTimer();

  await sendChunks(selected);
}

async function handleCancel() {
  canceled = true;
  abortController.value?.abort();
  abortController.value = null;
  pauseTimer();
  phase.value = 'idle';
  elapsedMs.value = startedAt ? Date.now() - startedAt : 0;

  await discardSession();

  // keep the file and its name so that the upload can be restarted right away
  uploadedBytes.value = 0;
  totalBytes.value = file.value?.size ?? 0;
  offset.value = 0;
  chunkSize.value = 0;
  errorMessage.value = '';
  resetSpeed();
  window.$message?.info('已取消上传。');
}

const s3Submitting = ref(false);
/** the failure of the last s3 pull, reported inline with its stable error code */
const s3Error = ref<{ code: string; message: string } | null>(null);

function emptyS3Form() {
  return {
    url: '',
    bucket: '',
    key: '',
    name: '',
    arch: 'x86_64' as Api.IsoBuilder.Arch,
    endpoint: '',
    region: '',
    // 'inherit' keeps the system path_style, the other values override it
    path_style: 'inherit' as 'inherit' | 'path' | 'virtual',
    access_key: '',
    secret_key: ''
  };
}

/** the s3 pull form, a blank credential box falls back to the saved system settings */
const s3Form = ref(emptyS3Form());

const pathStyleOptions = [
  { label: '跟随系统配置', value: 'inherit' },
  { label: '开启（path style）', value: 'path' },
  { label: '关闭（virtual-hosted）', value: 'virtual' }
];

async function handleS3Submit() {
  if (s3Submitting.value) return;

  s3Error.value = null;

  const url = s3Form.value.url.trim();
  const bucket = s3Form.value.bucket.trim();
  const key = s3Form.value.key.trim();

  // the backend accepts an s3:// url, or an explicit bucket + key pair
  if (!url && !(bucket && key)) {
    s3Error.value = {
      code: 'invalid_s3',
      message: '请输入对象地址（s3://bucket/path/file.iso），或同时填写 bucket 与 key。'
    };
    return;
  }

  const s3: Api.IsoBuilder.S3RegisterSource = {};

  if (url) {
    s3.url = url;
  } else {
    s3.bucket = bucket;
    s3.key = key;
  }

  if (s3Form.value.endpoint.trim()) s3.endpoint = s3Form.value.endpoint.trim();
  if (s3Form.value.region.trim()) s3.region = s3Form.value.region.trim();

  // the credentials are optional: only a non-empty box is sent, otherwise the backend uses the
  // credentials saved on the system settings page
  if (s3Form.value.access_key) s3.access_key = s3Form.value.access_key;
  if (s3Form.value.secret_key) s3.secret_key = s3Form.value.secret_key;

  if (s3Form.value.path_style !== 'inherit') s3.path_style = s3Form.value.path_style === 'path';

  s3Submitting.value = true;

  const { error } = await fetchRegisterIso({
    source: 's3',
    arch: s3Form.value.arch,
    name: s3Form.value.name.trim() || undefined,
    s3
  });

  s3Submitting.value = false;

  if (error) {
    s3Error.value = { code: getApiErrorCode(error), message: getApiErrorMessage(error) };
    return;
  }

  window.$message?.success('已提交，后台开始从对象存储下载镜像。');
  emit('finished');
  emit('update:show', false);
}

/** jump to the system settings page when the storage credentials are missing */
async function goSettings() {
  emit('update:show', false);
  await routerPushByKey('settings');
}

function resetForm() {
  pauseTimer();
  phase.value = 'idle';
  canceled = false;
  form.value.name = '';
  form.value.arch = 'x86_64';
  file.value = null;
  derivedName = '';
  uploadId.value = '';
  chunkSize.value = 0;
  offset.value = 0;
  uploadedBytes.value = 0;
  totalBytes.value = 0;
  elapsedMs.value = 0;
  errorMessage.value = '';
  s3Form.value = emptyS3Form();
  s3Error.value = null;
  resetSpeed();
}

onBeforeUnmount(() => {
  pauseTimer();
  abortController.value?.abort();

  // the backend drops the temporary file of a session which is never finished
  if (uploadId.value) {
    fetchCancelIsoUpload(uploadId.value);
  }
});
</script>

<template>
  <NDrawer
    :show="show"
    placement="right"
    width="min(100vw, 480px)"
    :mask-closable="!busy && !s3Submitting"
    :close-on-esc="!busy && !s3Submitting"
    @update:show="handleUpdateShow"
    @after-leave="resetForm"
  >
    <NDrawerContent title="添加基础镜像" :native-scrollbar="false" closable>
      <div class="mb-16px">
        <NRadioGroup v-model:value="activeTab" size="small">
          <NRadioButton value="upload">分片上传</NRadioButton>
          <NRadioButton value="s3">从对象存储拉取</NRadioButton>
        </NRadioGroup>
      </div>

      <div v-show="activeTab === 'upload'" class="flex flex-col gap-16px">
        <input ref="fileInputRef" type="file" accept=".iso" class="hidden" @change="handleFileChange" />

        <div class="flex flex-wrap items-center gap-8px">
          <NButton :disabled="busy" @click="triggerSelect">
            <template #icon>
              <SvgIcon icon="mdi:folder-open-outline" />
            </template>
            选择 ISO 文件
          </NButton>
          <NButton v-if="file && !busy" size="small" text type="error" @click="handleClearFile">移除</NButton>
        </div>

        <div v-if="file" class="rounded-6px bg-gray-50 px-12px py-8px dark:bg-gray-900">
          <div class="break-all text-13px">{{ file.name }}</div>
          <div class="font-mono text-12px text-gray-400">{{ formatBytes(file.size) }}</div>
        </div>

        <NForm ref="formRef" :model="form" :rules="rules" label-placement="top" :show-require-mark="false">
          <NFormItem label="镜像名称" path="name">
            <NInput v-model:value="form.name" :disabled="busy" placeholder="例如 Rocky-9.8-x86_64-dvd" />
          </NFormItem>
          <NFormItem label="架构" path="arch">
            <NSelect v-model:value="form.arch" :options="archOptions" :disabled="busy" />
          </NFormItem>
        </NForm>

        <div v-if="busy || allSent" class="flex flex-col gap-8px">
          <div class="flex items-center justify-between text-13px">
            <span class="text-gray-500">{{ phase === 'finishing' ? '正在收尾…' : '上传进度' }}</span>
            <span class="font-mono">{{ percent.toFixed(1) }}%</span>
          </div>

          <NProgress type="line" :percentage="Number(percent.toFixed(1))" :height="10" :border-radius="5" :show-indicator="false" />

          <div class="flex flex-wrap gap-x-16px gap-y-4px text-13px text-gray-500">
            <span>
              已传
              <span class="font-mono text-gray-900 dark:text-gray-100">{{ formatBytes(uploadedBytes) }}</span>
              /
              <span class="font-mono text-gray-900 dark:text-gray-100">{{ formatBytes(totalBytes) }}</span>
            </span>
            <span>
              速度 <span class="font-mono text-gray-900 dark:text-gray-100">{{ formatSpeed(speed) }}</span>
            </span>
            <span>
              已用时 <span class="font-mono text-gray-900 dark:text-gray-100">{{ elapsed }}</span>
            </span>
          </div>

          <div class="overflow-x-auto whitespace-nowrap font-mono text-12px text-gray-400">
            offset {{ offset }} · chunk {{ currentChunk }}/{{ chunkCount }} · {{ formatBytes(chunkSize) }}/片
          </div>
        </div>

        <NAlert v-if="errorMessage" type="error" :bordered="false" :show-icon="true" title="上传失败">
          {{ errorMessage }}
        </NAlert>
      </div>

      <div v-show="activeTab === 's3'" class="flex flex-col gap-16px">
        <NAlert type="default" :bordered="false" :show-icon="false">
          从已有的 S3 兼容对象存储拉取 ISO。后端先做一次 HeadObject 探活，通过后在后台流式下载并计算 SHA256。
        </NAlert>

        <NForm :model="s3Form" label-placement="top" :show-require-mark="false">
          <NFormItem label="对象地址" path="url">
            <NInput
              v-model:value="s3Form.url"
              :disabled="s3Submitting"
              :input-props="{ class: 'font-mono', spellcheck: 'false', autocomplete: 'off' }"
              placeholder="s3://bucket/path/Rocky-9.8-x86_64-dvd.iso"
            />
          </NFormItem>

          <NGrid :cols="2" :x-gap="12">
            <NFormItemGi label="Bucket（可选）" path="bucket">
              <NInput v-model:value="s3Form.bucket" :disabled="s3Submitting" placeholder="或单独指定 bucket" />
            </NFormItemGi>
            <NFormItemGi label="Key（可选）" path="key">
              <NInput v-model:value="s3Form.key" :disabled="s3Submitting" placeholder="或单独指定 key" />
            </NFormItemGi>
          </NGrid>

          <NFormItem label="镜像名称" path="name">
            <NInput v-model:value="s3Form.name" :disabled="s3Submitting" placeholder="留空则取文件名" />
          </NFormItem>

          <NFormItem label="架构" path="arch">
            <NSelect v-model:value="s3Form.arch" :options="archOptions" :disabled="s3Submitting" />
          </NFormItem>

          <NFormItem label="Endpoint（可选，覆盖系统配置）" path="endpoint">
            <NInput
              v-model:value="s3Form.endpoint"
              :disabled="s3Submitting"
              :input-props="{ class: 'font-mono', spellcheck: 'false', autocomplete: 'off' }"
              placeholder="https://oss.example.com"
            />
          </NFormItem>

          <NFormItem label="Region（可选，覆盖系统配置）" path="region">
            <NInput
              v-model:value="s3Form.region"
              :disabled="s3Submitting"
              :input-props="{ class: 'font-mono', spellcheck: 'false', autocomplete: 'off' }"
              placeholder="us-east-1"
            />
          </NFormItem>

          <NFormItem label="Path style" path="path_style">
            <NSelect v-model:value="s3Form.path_style" :options="pathStyleOptions" :disabled="s3Submitting" />
          </NFormItem>

          <NGrid :cols="2" :x-gap="12">
            <NFormItemGi label="Access Key（可选）" path="access_key">
              <NInput
                v-model:value="s3Form.access_key"
                type="password"
                show-password-on="click"
                :disabled="s3Submitting"
                placeholder="留空用系统配置"
              />
            </NFormItemGi>
            <NFormItemGi label="Secret Key（可选）" path="secret_key">
              <NInput
                v-model:value="s3Form.secret_key"
                type="password"
                show-password-on="click"
                :disabled="s3Submitting"
                placeholder="留空用系统配置"
              />
            </NFormItemGi>
          </NGrid>
        </NForm>

        <NAlert v-if="s3Error" type="error" :bordered="false" :show-icon="true" title="拉取失败">
          <div class="break-all">{{ s3Error.message }}</div>
          <NButton
            v-if="s3Error.code === 'storage_not_configured'"
            class="mt-8px"
            size="tiny"
            secondary
            type="primary"
            @click="goSettings"
          >
            前往「系统配置」填写凭据
          </NButton>
        </NAlert>
      </div>

      <template #footer>
        <NSpace justify="end">
          <template v-if="activeTab === 'upload'">
            <NButton v-if="busy" type="error" ghost :disabled="phase === 'finishing'" @click="handleCancel">取消上传</NButton>
            <NButton v-else @click="handleUpdateShow(false)">关闭</NButton>
            <NButton type="primary" :loading="busy" :disabled="busy || !file" @click="handleStart">{{ submitText }}</NButton>
          </template>
          <template v-else>
            <NButton :disabled="s3Submitting" @click="handleUpdateShow(false)">关闭</NButton>
            <NButton type="primary" :loading="s3Submitting" @click="handleS3Submit">拉取</NButton>
          </template>
        </NSpace>
      </template>
    </NDrawerContent>
  </NDrawer>
</template>

<style scoped></style>
