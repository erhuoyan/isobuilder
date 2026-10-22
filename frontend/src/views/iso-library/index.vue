<script setup lang="ts">
import { computed, h, onMounted, ref, watch } from 'vue';
import type { DataTableColumns, DataTableRowKey, FormInst, FormRules } from 'naive-ui';
import { NButton, NPopconfirm, NSpace, NTag, NTooltip } from 'naive-ui';
import { useIntervalFn } from '@vueuse/core';
import { fetchDeleteIso, fetchGetIsos, fetchRegisterIso } from '@/service/api';
import { getApiErrorCode } from '@/service/api/errors';
import { archOptions, isoSourceRecord } from '@/constants/isobuilder';
import { EMPTY_VALUE, formatBytes, formatDateTime } from '@/utils/format';
import CopyText from '@/components/isobuilder/copy-text.vue';
import IsoStatusTag from '@/components/isobuilder/iso-status-tag.vue';
import IsoUploadDrawer from '@/components/isobuilder/iso-upload-drawer.vue';

/** the interval used while an iso is being uploaded or hashed */
const POLL_INTERVAL = 3000;

const loading = ref(false);
const isos = ref<Api.IsoBuilder.Iso[]>([]);

const uploadShow = ref(false);
/** which tab of the add-image drawer to open: chunked browser upload or an s3 pull */
const uploadMode = ref<'url' | 'path'>('url');

/** an iso which is still being transferred or hashed keeps changing on the backend */
const activeCount = computed(
  () =>
    isos.value.filter(iso => iso.status === 'downloading' || iso.status === 'hashing')
      .length
);

/** open the add-image drawer on the requested tab */
function openAddDrawer(mode: 'url' | 'path') {
  uploadMode.value = mode;
  uploadShow.value = true;
}

async function loadIsos(silent = false) {
  if (!silent) loading.value = true;

  const { data, error } = await fetchGetIsos();

  if (!error && data) {
    isos.value = data.isos;
  }

  loading.value = false;
}

const { pause: pausePolling, resume: resumePolling } = useIntervalFn(() => loadIsos(true), POLL_INTERVAL, {
  immediate: false
});

watch(activeCount, count => {
  if (count > 0) {
    resumePolling();
  } else {
    pausePolling();
  }
});

async function handleDelete(row: Api.IsoBuilder.Iso) {
  const { error } = await fetchDeleteIso(row.id);

  if (error) {
    if (getApiErrorCode(error) === 'iso_in_use') {
      window.$message?.warning('镜像已被项目引用，无法删除。');
    }

    return;
  }

  window.$message?.success('镜像已删除');
  await loadIsos();
}

function renderStatus(row: Api.IsoBuilder.Iso) {
  if (row.status !== 'error') {
    return h(IsoStatusTag, { status: row.status });
  }

  return h(
    NTooltip,
    { trigger: 'hover' },
    {
      trigger: () => h(IsoStatusTag, { status: row.status }),
      default: () => h('span', { class: 'block max-w-280px break-all text-12px' }, row.error || '未知错误')
    }
  );
}

function renderMd5(row: Api.IsoBuilder.Iso) {
  if (!row.md5) {
    return h('span', { class: 'text-gray-400' }, EMPTY_VALUE);
  }

  return h(
    NTooltip,
    { trigger: 'hover', keepAliveOnHover: true },
    {
      trigger: () => h('code', { class: 'cursor-help font-mono text-12px' }, `${row.md5.slice(0, 8)}…`),
      default: () => h(CopyText, { text: row.md5 })
    }
  );
}

function renderPath(row: Api.IsoBuilder.Iso) {
  if (!row.path) {
    return h('span', { class: 'text-gray-400' }, EMPTY_VALUE);
  }

  return h(
    NTooltip,
    { trigger: 'hover' },
    {
      trigger: () => h('code', { class: 'block max-w-240px truncate font-mono text-12px' }, row.path),
      default: () => h('span', { class: 'font-mono text-12px break-all' }, row.path)
    }
  );
}

const columns: DataTableColumns<Api.IsoBuilder.Iso> = [
  {
    key: 'name',
    title: '名称',
    minWidth: 260,
    render: row =>
      h('div', { class: 'min-w-0 py-1' }, [
        h('div', { class: 'truncate text-15px font-500', title: row.name }, row.name),
        h('div', { class: 'truncate font-mono text-12px text-gray-400' }, row.filename)
      ])
  },
  {
    key: 'arch',
    title: '架构',
    width: 100,
    render: row => h('span', { class: 'font-mono' }, row.arch)
  },
  {
    key: 'size_bytes',
    title: '大小',
    width: 110,
    render: row => h('span', { class: 'font-mono' }, formatBytes(row.size_bytes))
  },
  {
    key: 'status',
    title: '状态',
    width: 110,
    render: renderStatus
  },
  {
    key: 'source',
    title: '来源',
    width: 110,
    render: row => {
      const record = isoSourceRecord[row.source] || { label: row.source, type: 'default' as const };

      return h(NTag, { size: 'small', bordered: false, type: record.type }, { default: () => record.label });
    }
  },
  {
    key: 'md5',
    title: 'MD5',
    minWidth: 140,
    render: renderMd5
  },
  {
    key: 'remark',
    title: '备注',
    minWidth: 160,
    ellipsis: { tooltip: true },
    render: row => row.remark || h('span', { class: 'text-gray-400' }, EMPTY_VALUE)
  },
  {
    key: 'created_at',
    title: '创建时间',
    width: 170,
    render: row => h('span', { class: 'font-mono text-13px' }, formatDateTime(row.created_at))
  },
  {
    key: 'actions',
    title: '操作',
    width: 90,
    fixed: 'right',
    align: 'center',
    render: row =>
      h(
        NPopconfirm,
        { positiveText: '删除', negativeText: '取消', onPositiveClick: () => handleDelete(row) },
        {
          trigger: () => h(NButton, { size: 'small', text: true, type: 'error' }, { default: () => '删除' }),
          default: () =>
            row.source === 'registered'
              ? `删除登记记录「${row.name}」？服务器上的文件会保留。`
              : `删除镜像「${row.name}」及其文件？`
        }
      )
  }
];

const rowKey = (row: Api.IsoBuilder.Iso): DataTableRowKey => row.id;

onMounted(() => {
  loadIsos();
});
</script>

<template>
  <div class="flex-col-stretch gap-4">
    <NCard :bordered="false" class="card-wrapper" size="medium">
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 class="text-18px font-500">镜像库</h2>
        <p class="mt-1 text-13px text-gray-500">上传或登记基础 ISO，构建时作为原版镜像注入 kickstart 与主机清单。</p>
      </div>
      <NSpace :size="8" :wrap="true">
        <NButton :loading="loading" @click="loadIsos()">
          <template #icon>
            <SvgIcon icon="mdi:refresh" />
          </template>
          刷新
        </NButton>
        <NButton @click="openAddDrawer('path')">
          <template #icon>
            <SvgIcon icon="mdi:file-link-outline" />
          </template>
          本地注册
        </NButton>
        <NButton type="primary" @click="openAddDrawer('url')">
          <template #icon>
            <SvgIcon icon="mdi:cloud-download-outline" />
          </template>
          链接导入
        </NButton>
      </NSpace>
    </div>

    <NAlert v-if="activeCount > 0" class="mb-3" type="info" :bordered="false" :show-icon="true">
      有 {{ activeCount }} 个镜像正在下载或计算校验值，列表每 3 秒自动刷新。
    </NAlert>

    <NDataTable
      :columns="columns"
      :data="isos"
      :loading="loading"
      :row-key="rowKey"
      :scroll-x="1290"
      size="medium"
      bordered
      :single-line="false"
    >
      <template #empty>
        <div class="py-10">
          <NEmpty description="还没有基础镜像，先用「链接导入」拉取一个 ISO 吧" />
        </div>
      </template>
    </NDataTable>

    <IsoUploadDrawer v-model:show="uploadShow" :mode="uploadMode" @finished="loadIsos()" />

    </NCard>
  </div>
</template>

<style scoped></style>
