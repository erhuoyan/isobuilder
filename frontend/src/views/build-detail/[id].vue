<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import { useIntervalFn } from '@vueuse/core';
import { fetchCancelBuild, fetchGetBuild, fetchRetryBuild } from '@/service/api';
import { getApiErrorCode } from '@/service/api/errors';
import { useRouterPush } from '@/hooks/common/router';
import { useAppStore } from '@/store/modules/app';
import { buildStages } from '@/constants/isobuilder';
import { formatBytes, formatDateTime, formatDuration } from '@/utils/format';
import ArtifactPanel from '@/components/isobuilder/artifact-panel.vue';
import BuildLogViewer from '@/components/isobuilder/build-log-viewer.vue';
import BuildStatusTag from '@/components/isobuilder/build-status-tag.vue';
import CopyText from '@/components/isobuilder/copy-text.vue';
import StageTrack from '@/components/isobuilder/stage-track.vue';

const route = useRoute();
const appStore = useAppStore();
const { routerPushByKey } = useRouterPush();

const buildId = computed(() => String(route.params.id || ''));
const descColumns = computed(() => (appStore.isMobile ? 1 : 2));

const loading = ref(false);
const notFound = ref(false);
const canceling = ref(false);
const retrying = ref(false);
const build = ref<Api.IsoBuilder.Build | null>(null);

/** the build is not finished yet, its state is polled */
const active = computed(() => Boolean(build.value && (build.value.status === 'queued' || build.value.status === 'running')));

const currentStage = computed(() => buildStages.find(item => item.key === build.value?.stage) || null);

const { pause: pausePolling, resume: resumePolling } = useIntervalFn(() => loadBuild(true), 2000, { immediate: false });

watch(active, value => {
  if (value) {
    resumePolling();
  } else {
    pausePolling();
  }
});

// the route is reused when the id changes, e.g. after a retry jumps to the new build
watch(buildId, () => {
  build.value = null;
  notFound.value = false;
  loadBuild();
});

async function loadBuild(silent = false) {
  if (!silent) loading.value = true;

  const { data, error } = await fetchGetBuild(buildId.value);

  if (error) {
    if (!silent) notFound.value = true;
    loading.value = false;
    return;
  }

  build.value = data.build;
  loading.value = false;
}

async function handleCancel() {
  canceling.value = true;

  const { error } = await fetchCancelBuild(buildId.value);

  canceling.value = false;

  if (error) {
    if (getApiErrorCode(error) === 'build_finished') {
      window.$message?.warning('构建已经结束，无需取消');
    }

    await loadBuild();

    return;
  }

  window.$message?.success('已请求取消构建');
  await loadBuild();
}

async function handleRetry() {
  retrying.value = true;

  const { data, error } = await fetchRetryBuild(buildId.value);

  retrying.value = false;

  if (error) return;

  window.$message?.success('已重新排队构建');
  routerPushByKey('build-detail', { params: { id: data.build.id } });
}

/** the log stream reached the end of the build, refresh the build once to pick up its final state */
function handleLogEnd() {
  loadBuild(true);
}

onMounted(() => {
  loadBuild();
});
</script>

<template>
  <NSpin :show="loading && !build">
    <NResult
      v-if="notFound"
      class="card-wrapper"
      status="404"
      title="构建不存在"
      description="该构建记录可能已被删除。"
    >
      <template #footer>
        <NSpace justify="center">
          <NButton @click="routerPushByKey('project-list')">返回项目列表</NButton>
        </NSpace>
      </template>
    </NResult>

    <NSpace v-else-if="build" vertical :size="16">
      <NCard :bordered="false" class="card-wrapper" size="small">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h2 class="text-18px font-500">构建详情</h2>
              <BuildStatusTag :status="build.status" />
              <NTag size="small" :bordered="false">{{ build.arch }}</NTag>
            </div>
            <div class="mt-2 flex flex-wrap items-center gap-x-16px gap-y-4px text-13px text-gray-500">
              <span>
                项目：
                <NButton
                  text
                  type="primary"
                  @click="routerPushByKey('project-detail', { params: { id: build.project_id } })"
                >
                  {{ build.project_name || build.project_id }}
                </NButton>
              </span>
              <span>
                输出：<span class="font-mono">{{ build.output_name || '—' }}</span>
              </span>
              <span>
                主机：<span class="font-mono">{{ build.hosts_count }} 台</span>
              </span>
              <span>
                耗时：<span class="font-mono">{{ formatDuration(build.started_at, build.finished_at) }}</span>
              </span>
            </div>
          </div>
          <NSpace :size="8" :wrap="true" justify="end">
            <NButton @click="routerPushByKey('project-detail', { params: { id: build.project_id } })">
              <template #icon>
                <SvgIcon icon="mdi:arrow-left" />
              </template>
              返回项目
            </NButton>
            <NPopconfirm v-if="active" positive-text="取消构建" negative-text="继续等待" @positive-click="handleCancel">
              <template #trigger>
                <NButton type="warning" secondary :loading="canceling">
                  <template #icon>
                    <SvgIcon icon="mdi:stop-circle-outline" />
                  </template>
                  取消构建
                </NButton>
              </template>
              取消后本次构建的工作目录会被清理，确认取消？
            </NPopconfirm>
            <NButton v-else type="primary" :loading="retrying" @click="handleRetry">
              <template #icon>
                <SvgIcon icon="mdi:restart" />
              </template>
              重新构建
            </NButton>
          </NSpace>
        </div>

        <div class="mt-5">
          <StageTrack
            :stage="build.stage"
            :stage-index="build.stage_index"
            :stage-total="build.stage_total"
            :progress="build.progress"
            :status="build.status"
          />
          <p class="mt-2 text-13px text-gray-500">
            <template v-if="currentStage">
              第 <span class="font-mono">{{ build.stage_index }}/{{ build.stage_total }}</span> 段 ·
              <span class="font-500">{{ currentStage.label }}</span> · {{ currentStage.description }}
            </template>
            <template v-else-if="build.status === 'queued'">已排队，等待 worker 取任务。</template>
            <template v-else-if="build.status === 'succeeded'">全部阶段已完成。</template>
            <template v-else>尚未开始。</template>
          </p>
        </div>
      </NCard>

      <NAlert v-if="build.status === 'failed'" type="error" :bordered="false" title="构建失败">
        <span class="font-mono break-all">{{ build.error || '未提供失败原因' }}</span>
      </NAlert>

      <NCard :bordered="false" class="card-wrapper" size="small" title="构建参数">
        <NDescriptions :column="descColumns" label-placement="left" bordered size="small">
          <NDescriptionsItem label="基础镜像">
            <span class="font-mono">{{ build.iso_name || '—' }}</span>
          </NDescriptionsItem>
          <NDescriptionsItem label="ISO Label">
            <span class="font-mono">{{ build.iso_label || '—' }}</span>
          </NDescriptionsItem>
          <NDescriptionsItem label="创建时间">
            <span class="font-mono text-13px">{{ formatDateTime(build.created_at) }}</span>
          </NDescriptionsItem>
          <NDescriptionsItem label="开始时间">
            <span class="font-mono text-13px">{{ formatDateTime(build.started_at) }}</span>
          </NDescriptionsItem>
          <NDescriptionsItem label="结束时间">
            <span class="font-mono text-13px">{{ formatDateTime(build.finished_at) }}</span>
          </NDescriptionsItem>
          <NDescriptionsItem label="日志大小">
            <span class="font-mono">{{ formatBytes(build.log_bytes) }}</span>
          </NDescriptionsItem>
        </NDescriptions>
        <div class="mt-3">
          <CopyText label="构建 ID" :text="build.id" />
        </div>
      </NCard>

      <ArtifactPanel :artifact="build.artifact" :build-status="build.status" />

      <BuildLogViewer
        :key="build.id"
        :build-id="build.id"
        :status="build.status"
        :log-bytes="build.log_bytes"
        @end="handleLogEnd"
      />
    </NSpace>
  </NSpin>
</template>

<style scoped></style>
