<script setup lang="ts">
import { computed, h } from 'vue';
import type { DataTableColumns } from 'naive-ui';
import { NTag } from 'naive-ui';

interface Props {
  hosts: Api.IsoBuilder.HostSummary[];
  /** the validation errors, the rows of the reported hosts are marked */
  errors?: Api.IsoBuilder.HostError[];
  loading?: boolean;
  maxHeight?: number;
}

const props = withDefaults(defineProps<Props>(), { errors: () => [], loading: false, maxHeight: 420 });

const errorIndexes = computed(() => new Set(props.errors.map(item => item.index)));

const mono = (value: string) => h('span', { class: 'font-mono' }, value || '—');

const columns: DataTableColumns<Api.IsoBuilder.HostSummary> = [
  {
    key: 'index',
    title: '#',
    width: 56,
    render: (_row, index) => String(index + 1)
  },
  { key: 'sn', title: 'SN', minWidth: 150, render: row => mono(row.sn) },
  { key: 'hostname', title: 'hostname', minWidth: 130, render: row => mono(row.hostname) },
  { key: 'ip', title: 'IP', minWidth: 130, render: row => mono(row.ip) },
  { key: 'gateway', title: '网关', minWidth: 120, render: row => mono(row.gateway) },
  {
    key: 'network_mode',
    title: '网络模式',
    width: 110,
    render: row =>
      h(
        NTag,
        { size: 'small', type: row.network_mode === 'bond' ? 'warning' : 'default', bordered: false, round: true },
        { default: () => (row.network_mode === 'bond' ? 'bond' : '单口') }
      )
  },
  { key: 'mac_count', title: 'MAC 数', width: 90, render: row => String(row.mac_count ?? 0) },
  { key: 'disk_gb', title: '磁盘 GB', width: 90, render: row => (row.disk_gb ? String(row.disk_gb) : '—') }
];

/** highlight the rows which have validation errors */
function rowProps(_row: Api.IsoBuilder.HostSummary, index: number) {
  return errorIndexes.value.has(index) ? { class: 'host-error-row' } : {};
}
</script>

<template>
  <NDataTable
    :columns="columns"
    :data="hosts"
    :loading="loading"
    :row-props="rowProps"
    :max-height="maxHeight"
    :row-key="(row: Api.IsoBuilder.HostSummary) => `${row.sn}-${row.ip}`"
    size="small"
    :bordered="false"
    :single-line="false"
  />
</template>

<style lang="scss" scoped>
:deep(.host-error-row) {
  td {
    background-color: rgb(var(--error-color) / 8%) !important;
  }
}
</style>
