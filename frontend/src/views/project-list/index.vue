<script setup lang="ts">
import { computed, h, onMounted, ref, watch } from 'vue';
import type { DataTableColumns, DataTableRowKey } from 'naive-ui';
import { NButton, NPopconfirm, NSpace, NTag, NTooltip } from 'naive-ui';
import { useIntervalFn } from '@vueuse/core';
import { fetchDeleteProject, fetchGetBuilds, fetchGetProjects } from '@/service/api';
import { useRouterPush } from '@/hooks/common/router';
import { formatDateTime } from '@/utils/format';
import BuildStatusTag from '@/components/isobuilder/build-status-tag.vue';

type ProjectRow = Api.IsoBuilder.ProjectListItem;

const { routerPushByKey } = useRouterPush();

const POLL_INTERVAL = 3000;

const loading = ref(false);
const projects = ref<ProjectRow[]>([]);
const latestBuilds = ref<Record<string, Api.IsoBuilder.Build>>({});

/** the projects which are being built right now, the list is polled until they finish */
const activeProjectIds = computed(() =>
  Object.values(latestBuilds.value)
    .filter(build => build.status === 'running' || build.status === 'queued')
    .map(build => build.project_id)
);

const hasActiveBuild = computed(() => activeProjectIds.value.length > 0);

async function loadProjects(silent = false) {
  if (!silent) loading.value = true;

  const [projectResponse, buildResponse] = await Promise.all([fetchGetProjects(), fetchGetBuilds({ limit: 200 })]);

  if (!projectResponse.error) {
    projects.value = projectResponse.data.projects;
  }

  if (!buildResponse.error) {
    // the builds are ordered by created_at desc, so the first one of a project is its latest build
    const record: Record<string, Api.IsoBuilder.Build> = {};

    buildResponse.data.builds.forEach(build => {
      if (!record[build.project_id]) {
        record[build.project_id] = build;
      }
    });

    latestBuilds.value = record;
  }

  loading.value = false;
}

const { pause: pausePolling, resume: resumePolling } = useIntervalFn(() => loadProjects(true), POLL_INTERVAL, {
  immediate: false
});

watch(hasActiveBuild, value => {
  if (value) {
    resumePolling();
  } else {
    pausePolling();
  }
});

async function handleDelete(id: string) {
  const { error } = await fetchDeleteProject(id);

  if (error) return;

  window.$message?.success('项目已删除');
  await loadProjects();
}

function renderLatestBuild(row: ProjectRow) {
  const build = latestBuilds.value[row.id];

  if (!build) {
    return h('span', { class: 'text-gray-400' }, '尚未构建');
  }

  return h(NSpace, { size: 8, align: 'center', wrap: false }, {
    default: () => [
      h(BuildStatusTag, { status: build.status, animated: false }),
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () =>
            h('span', { class: 'text-13px text-gray-500' }, formatDateTime(build.created_at, 'MM-DD HH:mm')),
          default: () => formatDateTime(build.created_at)
        }
      ),
      build.status === 'running'
        ? h('span', { class: 'font-mono text-13px text-gray-500' }, `${build.progress}%`)
        : null
    ]
  });
}

const columns: DataTableColumns<ProjectRow> = [
  {
    key: 'name',
    title: '项目',
    minWidth: 220,
    render: row =>
      h('div', { class: 'py-4px' }, [
        h('div', { class: 'text-15px font-500' }, row.name),
        h('div', { class: 'text-12px text-gray-400' }, row.description || '—')
      ])
  },
  { key: 'arch', title: '架构', width: 100, render: row => h('span', { class: 'font-mono' }, row.arch) },
  {
    key: 'iso',
    title: '基础镜像',
    minWidth: 220,
    render: row =>
      row.iso
        ? h('div', { class: 'min-w-0' }, [
            h('div', { class: 'truncate text-14px' }, row.iso.name),
            h('div', { class: 'truncate font-mono text-12px text-gray-400' }, row.iso.filename)
          ])
        : h(NTag, { size: 'small', type: 'error', bordered: false }, { default: () => '镜像已删除' })
  },
  {
    key: 'hosts_count',
    title: '主机台数',
    width: 110,
    render: row => h('span', { class: 'font-mono' }, `${row.hosts_count} 台`)
  },
  { key: 'latest_build', title: '最近构建', minWidth: 260, render: renderLatestBuild },
  {
    key: 'actions',
    title: '操作',
    width: 190,
    fixed: 'right',
    align: 'center',
    render: row =>
      h(NSpace, { size: 0, justify: 'center', wrap: false }, {
        default: () => [
          h(
            NButton,
            {
              size: 'small',
              text: true,
              type: 'primary',
              onClick: () => routerPushByKey('project-detail', { params: { id: row.id } })
            },
            { default: () => '进入' }
          ),
          h(
            NButton,
            {
              size: 'small',
              text: true,
              type: 'primary',
              onClick: () => routerPushByKey('project-edit', { params: { id: row.id } })
            },
            { default: () => '编辑' }
          ),
          h(
            NPopconfirm,
            { onPositiveClick: () => handleDelete(row.id), positiveText: '删除', negativeText: '取消' },
            {
              trigger: () => h(NButton, { size: 'small', text: true, type: 'error' }, { default: () => '删除' }),
              default: () => `删除项目「${row.name}」及其全部构建记录？`
            }
          )
        ]
      })
  }
];

const rowKey = (row: ProjectRow): DataTableRowKey => row.id;

onMounted(() => {
  loadProjects();
});
</script>

<template>
  <div class="flex-col-stretch gap-16px">
    <NCard :bordered="false" class="card-wrapper" size="medium">
    <div class="mb-16px flex flex-wrap items-center justify-between gap-12px">
      <div>
        <h2 class="text-18px font-500">装机项目</h2>
        <p class="mt-4px text-13px text-gray-500">
          每个项目对应一次 ISO 定制：绑定基础镜像与 kickstart 应答文件，并携带一份主机清单。
        </p>
      </div>
      <NSpace :size="8" :wrap="true" justify="end">
        <NButton :loading="loading" @click="loadProjects()">
          <template #icon>
            <SvgIcon icon="mdi:refresh" />
          </template>
          刷新
        </NButton>
        <NButton type="primary" @click="routerPushByKey('project-create')">
          <template #icon>
            <SvgIcon icon="mdi:plus" />
          </template>
          新建项目
        </NButton>
      </NSpace>
    </div>

    <NAlert v-if="hasActiveBuild" class="mb-12px" type="info" :bordered="false">
      有 {{ activeProjectIds.length }} 个项目正在构建，列表每 3 秒自动刷新。
    </NAlert>

    <NDataTable
      :columns="columns"
      :data="projects"
      :loading="loading"
      :row-key="rowKey"
      :scroll-x="1080"
      size="medium"
      :bordered="false"
      :single-line="false"
    >
      <template #empty>
        <div class="py-40px">
          <NEmpty description="还没有项目，先创建一个吧">
            <template #extra>
              <NButton type="primary" @click="routerPushByKey('project-create')">新建项目</NButton>
            </template>
          </NEmpty>
        </div>
      </template>
    </NDataTable>
    </NCard>
  </div>
</template>

<style scoped></style>
