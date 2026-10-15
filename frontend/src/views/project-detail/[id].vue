<script setup lang="ts">
import { computed, h, onMounted, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import type { DataTableColumns } from 'naive-ui';
import { NButton, NSpace } from 'naive-ui';
import { useIntervalFn } from '@vueuse/core';
import { fetchCreateBuild, fetchDeleteProject, fetchGetBuilds, fetchGetProject } from '@/service/api';
import { getApiErrorCode, getApiErrorDetails } from '@/service/api/errors';
import { useRouterPush } from '@/hooks/common/router';
import { useAppStore } from '@/store/modules/app';
import { EMPTY_VALUE, formatBytes, formatDateTime, formatDuration } from '@/utils/format';
import BuildStatusTag from '@/components/isobuilder/build-status-tag.vue';
import CopyText from '@/components/isobuilder/copy-text.vue';
import HostSummaryTable from '@/components/isobuilder/host-summary-table.vue';

const route = useRoute();
const appStore = useAppStore();
const { routerPushByKey } = useRouterPush();

const projectId = computed(() => String(route.params.id || ''));
const descColumns = computed(() => (appStore.isMobile ? 1 : 2));

const loading = ref(false);
const notFound = ref(false);
const building = ref(false);
const project = ref<Api.IsoBuilder.Project | null>(null);
const builds = ref<Api.IsoBuilder.Build[]>([]);

const activeBuild = computed(
  () => builds.value.find(item => item.status === 'running' || item.status === 'queued') || null
);

const { pause: pausePolling, resume: resumePolling } = useIntervalFn(loadBuilds, 3000, { immediate: false });

watch(activeBuild, active => {
  if (active) {
    resumePolling();
  } else {
    pausePolling();
  }
});

async function loadProject() {
  loading.value = true;

  const { data, error } = await fetchGetProject(projectId.value);

  if (error) {
    notFound.value = true;
    loading.value = false;
    return;
  }

  project.value = data.project;
  loading.value = false;
}

/** load the build history of the project */
async function loadBuilds() {
  const { data, error } = await fetchGetBuilds({ project_id: projectId.value, limit: 50 });

  if (!error) {
    builds.value = data.builds;
  }
}

async function handleBuild() {
  building.value = true;
  const { data, error } = await fetchCreateBuild(projectId.value);

  building.value = false;

  if (error) {
    if (getApiErrorCode(error) === 'project_incomplete') {
      const details = getApiErrorDetails<{ missing?: string[] }>(error);
      const missing = (details?.missing || []).join('、');

      window.$message?.warning(`项目配置不完整${missing ? `：缺少 ${missing}` : ''}，请先补全后再构建`);
    }

    return;
  }

  window.$message?.success('构建任务已排队');
  routerPushByKey('build-detail', { params: { id: data.build.id } });
}

async function handleDelete() {
  const { error } = await fetchDeleteProject(projectId.value);

  if (error) return;

  window.$message?.success('项目已删除');
  routerPushByKey('project-list');
}

const buildColumns: DataTableColumns<Api.IsoBuilder.Build> = [
  {
    key: 'status',
    title: '状态',
    width: 110,
    render: row => h(BuildStatusTag, { status: row.status, animated: row.status === 'running' })
  },
  {
    key: 'stage',
    title: '阶段',
    width: 90,
    render: row => h('span', { class: 'font-mono' }, row.stage ? `${row.stage_index}/${row.stage_total}` : '—')
  },
  {
    key: 'created_at',
    title: '创建时间',
    minWidth: 170,
    render: row => h('span', { class: 'font-mono text-13px' }, formatDateTime(row.created_at))
  },
  {
    key: 'duration',
    title: '耗时',
    width: 120,
    render: row => h('span', { class: 'font-mono text-13px' }, formatDuration(row.started_at, row.finished_at))
  },
  {
    key: 'log_bytes',
    title: '日志',
    width: 100,
    render: row => h('span', { class: 'font-mono text-13px' }, formatBytes(row.log_bytes))
  },
  {
    key: 'result',
    title: '结果',
    minWidth: 200,
    render: row =>
      row.error
        ? h('span', { class: 'font-mono text-12px text-error' }, row.error)
        : h('span', { class: 'text-gray-400' }, row.status === 'succeeded' ? row.output_name : EMPTY_VALUE)
  },
  {
    key: 'actions',
    title: '操作',
    width: 90,
    fixed: 'right',
    align: 'center',
    render: row =>
      h(
        NButton,
        {
          size: 'small',
          text: true,
          type: 'primary',
          onClick: () => routerPushByKey('build-detail', { params: { id: row.id } })
        },
        { default: () => '查看' }
      )
  }
];

const buildRowKey = (row: Api.IsoBuilder.Build) => row.id;

onMounted(async () => {
  await loadProject();
  await loadBuilds();
});
</script>

<template>
  <NSpin :show="loading">
    <NResult
      v-if="notFound"
      class="card-wrapper"
      status="404"
      title="项目不存在"
      description="该项目可能已被删除。"
    >
      <template #footer>
        <NButton type="primary" @click="routerPushByKey('project-list')">返回项目列表</NButton>
      </template>
    </NResult>

    <NSpace v-else-if="project" vertical :size="16">
      <NCard :bordered="false" class="card-wrapper" size="small">
        <div class="flex flex-wrap items-start justify-between gap-12px">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-8px">
              <h2 class="text-18px font-500">{{ project.name }}</h2>
              <NTag size="small" :bordered="false">{{ project.arch }}</NTag>
              <NTag v-if="activeBuild" size="small" :bordered="false" type="info">
                {{ activeBuild.status === 'queued' ? '排队中' : `构建中 ${activeBuild.progress}%` }}
              </NTag>
            </div>
            <p class="mt-6px text-13px text-gray-500">{{ project.description || '暂无描述' }}</p>
          </div>
          <NSpace :size="8">
            <NButton @click="routerPushByKey('project-edit', { params: { id: project.id } })">
              <template #icon>
                <SvgIcon icon="mdi:pencil-outline" />
              </template>
              编辑
            </NButton>
            <NButton type="primary" :loading="building" :disabled="Boolean(activeBuild)" @click="handleBuild">
              <template #icon>
                <SvgIcon icon="mdi:hammer-wrench" />
              </template>
              {{ activeBuild ? '构建进行中' : '开始构建' }}
            </NButton>
            <NPopconfirm positive-text="删除" negative-text="取消" @positive-click="handleDelete">
              <template #trigger>
                <NButton type="error" secondary>
                  <template #icon>
                    <SvgIcon icon="mdi:trash-can-outline" />
                  </template>
                  删除
                </NButton>
              </template>
              删除项目「{{ project.name }}」及其全部构建记录？
            </NPopconfirm>
          </NSpace>
        </div>

        <NAlert v-if="activeBuild" class="mt-16px" type="info" :bordered="false">
          <span>
            正在构建，最新进度
            <span class="font-mono">{{ activeBuild.progress }}%</span>
            ，每 3 秒自动刷新。
          </span>
          <NButton
            class="ml-8px"
            text
            type="primary"
            @click="routerPushByKey('build-detail', { params: { id: activeBuild.id } })"
          >
            查看实时日志
          </NButton>
        </NAlert>
      </NCard>

      <NCard :bordered="false" class="card-wrapper" size="small" title="配置摘要">
        <NDescriptions :column="descColumns" label-placement="left" bordered size="small">
          <NDescriptionsItem label="基础镜像">
            <template v-if="project.iso">
              <div class="text-14px">{{ project.iso.name }}</div>
              <div class="font-mono text-12px text-gray-500">{{ project.iso.filename }}</div>
            </template>
            <NTag v-else size="small" type="error" :bordered="false">镜像已删除</NTag>
          </NDescriptionsItem>
          <NDescriptionsItem label="kickstart">
            <template v-if="project.kickstart">
              <span class="font-mono">{{ project.kickstart.name }}</span>
              <span class="ml-6px text-12px text-gray-500">{{ project.kickstart.arch }}</span>
            </template>
            <span v-else class="text-gray-400">—</span>
          </NDescriptionsItem>
          <NDescriptionsItem label="ISO Label">
            <span class="font-mono">{{ project.iso_label || '（构建时自动探测）' }}</span>
          </NDescriptionsItem>
          <NDescriptionsItem label="输出文件名">
            <span class="font-mono">{{ project.output_name || '（构建时自动生成）' }}</span>
          </NDescriptionsItem>
          <NDescriptionsItem label="装机后脚本">
            <NTag v-if="project.post_script" size="small" type="success" :bordered="false">已配置</NTag>
            <span v-else class="text-gray-400">未配置</span>
          </NDescriptionsItem>
          <NDescriptionsItem label="主机清单">
            <span class="font-mono">{{ project.hosts_count }} 台</span>
            <span class="ml-6px text-12px text-gray-500">{{ project.hosts_format.toUpperCase() }}</span>
          </NDescriptionsItem>
          <NDescriptionsItem label="更新时间">
            <span class="font-mono text-13px">{{ formatDateTime(project.updated_at) }}</span>
          </NDescriptionsItem>
        </NDescriptions>
        <div class="mt-12px">
          <CopyText label="项目 ID" :text="project.id" />
        </div>
      </NCard>

      <NCard :bordered="false" class="card-wrapper" size="small" title="主机载荷">
        <template #header-extra>
          <span class="text-13px text-gray-500">共 {{ project.hosts_summary.length }} 台</span>
        </template>
        <HostSummaryTable :hosts="project.hosts_summary" :max-height="420" />
      </NCard>

      <NCard :bordered="false" class="card-wrapper" size="small" title="构建历史">
        <template #header-extra>
          <span v-if="activeBuild" class="text-13px text-gray-500">构建中，自动刷新</span>
        </template>
        <NDataTable
          :columns="buildColumns"
          :data="builds"
          :row-key="buildRowKey"
          :scroll-x="900"
          size="small"
          :bordered="false"
          :single-line="false"
        >
          <template #empty>
            <div class="py-32px">
              <NEmpty description="还没有构建记录，点击「开始构建」生成第一份镜像" />
            </div>
          </template>
        </NDataTable>
      </NCard>
    </NSpace>
  </NSpin>
</template>

<style scoped></style>
