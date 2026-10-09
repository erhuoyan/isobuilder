<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useIntervalFn } from '@vueuse/core';
import { fetchGetArtifact, getArtifactDownloadUrl } from '@/service/api';
import { formatBytes, formatDateTime, formatDuration } from '@/utils/format';
import CopyText from './copy-text.vue';

interface Props {
  artifact: Api.IsoBuilder.Artifact | null;
  buildStatus: Api.IsoBuilder.BuildStatus;
}

const props = defineProps<Props>();

/** the artifact is refreshed in place, `GET /api/artifacts/{id}` renews the presigned url */
const current = ref<Api.IsoBuilder.Artifact | null>(props.artifact);
const refreshing = ref(false);
const now = ref(Date.now());

watch(
  () => props.artifact,
  value => {
    current.value = value;
  }
);

useIntervalFn(() => {
  now.value = Date.now();
}, 30 * 1000);

const downloadUrl = computed(() => {
  if (!current.value) return '';

  return current.value.url || getArtifactDownloadUrl(current.value.id);
});

const isPresigned = computed(() => Boolean(current.value?.url));

const expiresAt = computed(() => current.value?.url_expires_at || '');

const expired = computed(() => Boolean(expiresAt.value) && new Date(expiresAt.value).getTime() <= now.value);

/** the remaining lifetime of the presigned url */
const remainingText = computed(() => {
  if (!expiresAt.value) return '';

  return formatDuration(new Date(now.value).toISOString(), expiresAt.value);
});

async function handleRefresh() {
  if (!current.value) return;

  refreshing.value = true;

  const { data, error } = await fetchGetArtifact(current.value.id);

  refreshing.value = false;

  if (error) return;

  current.value = data.artifact;
  now.value = Date.now();
  window.$message?.success('下载链接已刷新');
}

function handleDownload() {
  if (!downloadUrl.value) return;

  window.open(downloadUrl.value, '_blank', 'noopener');
}
</script>

<template>
  <NCard :bordered="false" class="card-wrapper" size="small" title="构建产物">
    <template #header-extra>
      <NSpace :size="8" :wrap="true" justify="end">
        <NButton size="small" :loading="refreshing" :disabled="!current" @click="handleRefresh">
          <template #icon>
            <SvgIcon icon="mdi:link-variant" />
          </template>
          刷新链接
        </NButton>
        <NButton size="small" type="primary" :disabled="!downloadUrl" @click="handleDownload">
          <template #icon>
            <SvgIcon icon="mdi:download" />
          </template>
          下载
        </NButton>
      </NSpace>
    </template>

    <NAlert v-if="!current" type="info" :bordered="false">
      {{
        buildStatus === 'succeeded'
          ? '构建已完成，但对象存储未启用或产物尚未登记，请稍后刷新本页。'
          : '构建成功后，这里会展示镜像文件名、大小、SHA256 与下载地址。'
      }}
    </NAlert>

    <div v-else class="min-w-0">
      <NDescriptions :column="1" label-placement="left" bordered size="small">
        <NDescriptionsItem label="文件名">
          <span class="font-mono break-all">{{ current.filename }}</span>
        </NDescriptionsItem>
        <NDescriptionsItem label="大小">
          <span class="font-mono">{{ formatBytes(current.size_bytes) }}</span>
        </NDescriptionsItem>
        <NDescriptionsItem label="存储">
          <NTag size="small" :bordered="false" :type="current.storage === 's3' ? 'info' : 'default'">
            {{ current.storage === 's3' ? '对象存储 (s3)' : '本地存储 (local)' }}
          </NTag>
        </NDescriptionsItem>
        <NDescriptionsItem label="生成时间">
          <span class="font-mono text-13px">{{ formatDateTime(current.created_at) }}</span>
        </NDescriptionsItem>
      </NDescriptions>

      <div class="mt-12px">
        <CopyText label="SHA256" :text="current.sha256" :wrap="true" />
      </div>

      <div class="mt-8px">
        <CopyText label="下载链接" :text="downloadUrl" />
      </div>

      <p v-if="isPresigned" class="mt-8px text-12px text-gray-500">
        <template v-if="expired">
          <span class="text-danger">预签名链接已于 {{ formatDateTime(expiresAt) }} 过期，请点击「刷新链接」。</span>
        </template>
        <template v-else-if="expiresAt">
          链接有效期至 <span class="font-mono">{{ formatDateTime(expiresAt) }}</span>
          （剩余 <span class="font-mono">{{ remainingText }}</span>），过期后可刷新。
        </template>
      </p>
      <p v-else class="mt-8px text-12px text-gray-500">当前由服务端直接下载（storage=local），链接长期有效。</p>
    </div>
  </NCard>
</template>

<style lang="scss" scoped>
.text-danger {
  color: rgb(var(--error-color));
}
</style>
