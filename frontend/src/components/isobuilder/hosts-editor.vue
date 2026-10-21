<script setup lang="ts">
import { computed, h, ref, watch } from 'vue';
import { useDebounceFn } from '@vueuse/core';
import * as XLSX from 'xlsx';
import { PREVIEW_COLUMN_SET, TEMPLATE_COLUMNS, TEMPLATE_HEADER_SET } from '@/constants/hosts-template';
import { fetchValidateHosts } from '@/service/api';

/** the idle time before the inventory is validated automatically */
const VALIDATE_DEBOUNCE = 500;

/** 本地文件与电子表格共用一个入口，按扩展名分流 */
const FILE_ACCEPT = '.yaml,.yml,.json,.txt,.xlsx,.xls,.csv';

const PLACEHOLDER = `hosts:
  - sn: SN0000000
    hostname: node-01
    network:
      ip: 172.16.0.11
      netmask: 255.255.255.0
      gateway: 172.16.0.1
      mac: 00:11:22:33:44:55
  
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

/** the raw inventory text, YAML or JSON (格式由后端自动识别) */
const value = defineModel<string>('value', { required: true });

const emit = defineEmits<{
  validate: [result: Api.IsoBuilder.ValidateHostsResult | null];
}>();

const result = ref<Api.IsoBuilder.ValidateHostsResult | null>(null);
const validating = ref(false);
/** the content of the last successful validation, empty when stale */
const validatedContent = ref('');
const requestFailed = ref(false);
const fileInputRef = ref<HTMLInputElement | null>(null);

/** the sequence of the newest validation request, the stale responses are dropped */
let requestSeq = 0;

const text = computed(() => value.value ?? '');

const hasContent = computed(() => text.value.trim().length > 0);

/** the input has been edited since the last validation, the shown result is outdated */
const stale = computed(
  () => hasContent.value && !validating.value && text.value !== validatedContent.value
);

/** 校验通过的主机数，用于顶部一行摘要 */
const hostCount = computed(() => (result.value?.ok ? result.value.hosts_count : 0));

function invalidate() {
  requestSeq += 1;
  validatedContent.value = '';
  validating.value = false;
  requestFailed.value = false;
  emit('validate', null);
}

async function runValidate() {
  const content = text.value;

  if (!content.trim()) {
    result.value = null;
    invalidate();
    return;
  }

  // 内容没变就复用上次结果，不重复打后端
  if (content === validatedContent.value && result.value) {
    emit('validate', result.value);
    return;
  }

  const seq = ++requestSeq;
  validating.value = true;

  const { data, error } = await fetchValidateHosts(content, 'auto');

  if (seq !== requestSeq) return;

  validating.value = false;

  if (error) {
    validatedContent.value = '';
    result.value = null;
    requestFailed.value = true;
    emit('validate', null);
    return;
  }

  validatedContent.value = content;
  result.value = data;
  emit('validate', data);
}

const debouncedValidate = useDebounceFn(runValidate, VALIDATE_DEBOUNCE);

watch(
  value,
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

/** ─── 导入：一次点击，按扩展名分流 ─────────────────────────────── */

function triggerFileSelect() {
  fileInputRef.value?.click();
}

async function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];

  // reset the input so that the same file can be selected twice in a row
  input.value = '';

  if (!file) return;

  const name = file.name.toLowerCase();
  const isSpreadsheet = name.endsWith('.xlsx') || name.endsWith('.xls') || name.endsWith('.csv');

  if (isSpreadsheet) {
    await importSpreadsheet(file);
    return;
  }

  try {
    value.value = await file.text();
    window.$message?.success(`已读入 ${file.name}`);
  } catch {
    window.$message?.error('读取本地文件失败，请重试。');
  }
}

/** ─── 导出：把编辑框里的内容存成文件 ───────────────────────────── */

function handleExport() {
  if (!hasContent.value) {
    window.$message?.warning('主机清单还是空的');
    return;
  }

  // 内容本来就是 YAML/JSON 原文，直接原样下载，不做任何转换。
  const blob = new Blob([text.value], { type: 'text/yaml;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = 'hosts.yaml';
  link.click();
  URL.revokeObjectURL(url);
}

/** ─── 电子表格 → YAML ─────────────────────────────────────────── */

interface SpreadsheetRow {
  [column: string]: string | number | undefined;
}

const tableRows = ref<SpreadsheetRow[]>([]);
const tableHeaders = ref<string[]>([]);

function cellText(row: SpreadsheetRow, column: string): string {
  const v = row[column];
  return v === undefined || v === null ? '' : String(v).trim();
}

/** 按标准表头取一行里的字段值（空单元格继承上一行非空值：同批次网关/掩码常留空） */
function extractHost(rows: SpreadsheetRow[], index: number) {
  const get = (field: string): string => {
    if (!TEMPLATE_HEADER_SET.has(field)) return '';
    return cellText(rows[index], field);
  };

  const inherit = (field: string): string => {
    let v = get(field);
    if (v) return v;
    for (let i = index - 1; i >= 0; i -= 1) {
      v = cellText(rows[i], field);
      if (v) return v;
    }
    return '';
  };

  return { get, inherit };
}

/** quote one string scalar as a YAML double-quoted token */
function yamlQuote(text: string): string {
  return `"${text.replace(/\\/g, '\\\\').replace(/"/g, '\\"')}"`;
}

async function importSpreadsheet(file: File) {
  try {
    const buffer = await file.arrayBuffer();
    const workbook = XLSX.read(buffer, { type: 'array' });
    const sheet = workbook.Sheets[workbook.SheetNames[0]];
    const rows = XLSX.utils.sheet_to_json<SpreadsheetRow>(sheet, { defval: '' });

    if (!rows.length) {
      window.$message?.error('表格没有数据行');
      return;
    }

    const headers = Object.keys(rows[0]);
    // 表头按模板原样严格匹配，区分大小写，仅容忍前后空格
    const originals = new Map(headers.map(header => [header.trim(), header]));

    const missing = TEMPLATE_COLUMNS.filter(col => !originals.has(col));
    if (missing.length) {
      window.$message?.error(
        `表格缺少列 ${missing.join('、')}。实际表头：${headers.join('、')}。列名需按模板原样填写（区分大小写）。`
      );
      return;
    }

    // 按协议列重排，非协议列原样保留（进 extra，后端透传）
    tableRows.value = rows.map(row => {
      const rebased: SpreadsheetRow = {};
      TEMPLATE_COLUMNS.forEach(col => {
        const original = originals.get(col);
        rebased[col] = original === undefined ? '' : row[original];
      });
      headers.forEach(header => {
        if (!TEMPLATE_HEADER_SET.has(header.trim())) {
          rebased[header] = row[header];
        }
      });
      return rebased;
    });

    tableHeaders.value = [
      ...TEMPLATE_COLUMNS,
      ...headers.filter(header => !TEMPLATE_HEADER_SET.has(header.trim()))
    ];

    generateYamlFromTable();
  } catch {
    window.$message?.error('解析表格失败，请确认为 .xlsx / .xls / .csv 格式');
  }
}

/** 表格 → YAML 清单，直接写进编辑框，用户可继续手工微调 */
function generateYamlFromTable() {
  const probe = extractHost(tableRows.value, 0);
  const mode = probe.get('mac1') && probe.get('mac2') ? 'bond' : probe.get('mac') ? 'single' : 'unknown';

  if (mode === 'unknown') {
    window.$message?.error('表格里没有 mac 列（单口）也没有 mac1/mac2 列（bond），无法生成网络配置');
    return;
  }

  const lines: string[] = ['hosts:'];

  tableRows.value.forEach((_, index) => {
    const { get, inherit } = extractHost(tableRows.value, index);

    const sn = get('sn');
    const hostname = get('hostname');
    if (!sn && !hostname) return; // 全空行跳过

    lines.push(`  - sn: ${sn || `AUTO-${index + 1}`}`);
    lines.push(`    hostname: ${hostname || `node-${index + 1}`}`);

    // YAML 双引号风格转义：兼容密码里的 : # " \ 等字符，避免破坏清单结构
    const password = get('root_password');
    if (password) lines.push(`    root_password: ${yamlQuote(password)}`);

    const disk = get('disk_target_size');
    if (disk) lines.push(`    disk_target_size: ${disk}`);

    lines.push('    network:');
    lines.push(`      ip: ${get('ip') || inherit('ip')}`);

    const netmask = get('netmask') || inherit('netmask');
    const gateway = get('gateway') || inherit('gateway');
    if (netmask) lines.push(`      netmask: ${netmask}`);
    if (gateway) lines.push(`      gateway: ${gateway}`);

    if (mode === 'bond') {
      lines.push('      bond:');
      lines.push('        mode: 4');
      lines.push('        slaves:');
      lines.push(`          - ${get('mac1')}`);
      lines.push(`          - ${get('mac2')}`);
    } else {
      lines.push(`      mac: ${get('mac')}`);
    }
  });

  value.value = `${lines.join('\n')}\n`;
  window.$message?.success(`已从表格生成 ${tableRows.value.length} 台主机的清单`);
}

/** 不参与装机的列，提示用户这些值会被忽略 */
const droppedColumns = computed(() => tableHeaders.value.filter(column => !PREVIEW_COLUMN_SET.has(column)));
const droppedColumnsText = computed(() =>
  droppedColumns.value.length ? `：${droppedColumns.value.join('、')}` : ''
);

const errorCount = computed(() => result.value?.errors.length ?? 0);

/** the label of one error, an inventory level error carries an empty field */
function hostLabel(error: Api.IsoBuilder.HostError) {
  return error.index >= 0 ? `第 ${error.index + 1} 台` : '清单级';
}
</script>

<template>
  <div class="flex flex-col gap-12px">
    <div class="flex flex-wrap items-center justify-between gap-8px">
      <span class="text-13px text-gray-500">粘贴、导入文件或表格；停止输入 0.5 秒后自动校验</span>

      <NSpace :size="8">
        <NButton size="small" @click="triggerFileSelect">
          <template #icon>
            <SvgIcon icon="mdi:file-upload-outline" />
          </template>
          导入
        </NButton>
        <NButton size="small" :disabled="!hasContent" @click="handleExport">
          <template #icon>
            <SvgIcon icon="mdi:file-download-outline" />
          </template>
          导出
        </NButton>
      </NSpace>
    </div>

    <input ref="fileInputRef" type="file" :accept="FILE_ACCEPT" class="hidden" @change="handleFileChange" />

    <NInput
      v-model:value="value"
      type="textarea"
      :rows="14"
      class="font-mono"
      :input-props="{ class: 'font-mono', spellcheck: 'false' }"
      :placeholder="PLACEHOLDER"
    />

    <NAlert v-if="validating" size="small" type="info" :bordered="false">正在校验主机清单…</NAlert>

    <NAlert v-else-if="requestFailed" size="small" type="error" :bordered="false">
      校验请求失败，请检查清单格式后重试。
    </NAlert>

    <NAlert v-else-if="stale" size="small" type="warning" :bordered="false">
      清单已修改，等待自动校验…
    </NAlert>

    <template v-else-if="result?.ok">
      <NAlert size="small" type="success" :bordered="false">
        校验通过：{{ hostCount }} 台主机
      </NAlert>

      <NDataTable
        :columns="[
          { key: 'sn', title: 'SN', width: 160, render: (r: Api.IsoBuilder.HostSummary) => h('span', { class: 'font-mono text-12px' }, r.sn) },
          { key: 'hostname', title: '主机名', width: 140 },
          { key: 'ip', title: 'IP', width: 140, render: (r: Api.IsoBuilder.HostSummary) => h('span', { class: 'font-mono text-12px' }, r.ip) },
          { key: 'gateway', title: '网关', width: 140, render: (r: Api.IsoBuilder.HostSummary) => h('span', { class: 'font-mono text-12px' }, r.gateway || '—') },
          { key: 'network_mode', title: '网络', width: 110, render: (r: Api.IsoBuilder.HostSummary) => (r.network_mode === 'bond' ? `bond (${r.mac_count})` : '单口') },
          { key: 'disk_gb', title: '磁盘', width: 100, render: (r: Api.IsoBuilder.HostSummary) => (r.disk_gb ? `${r.disk_gb} GB` : '—') },
          { key: 'bmc_ip', title: 'BMC', width: 140, render: (r: Api.IsoBuilder.HostSummary) => h('span', { class: 'font-mono text-12px' }, r.bmc_ip || '—') }
        ]"
        :data="result.hosts_summary"
        :row-key="(row: Api.IsoBuilder.HostSummary) => row.sn"
        size="small"
        :bordered="false"
        :single-line="false"
        :max-height="260"
      />
    </template>

    <template v-else-if="result">
      <NAlert size="small" type="error" :bordered="false">
        校验失败，共 {{ errorCount }} 处问题
      </NAlert>
      <div class="max-h-200px flex flex-col gap-4px overflow-y-auto">
        <div v-for="(error, index) in result.errors" :key="index" class="text-12px text-red-500">
          {{ hostLabel(error) }} · {{ error.field || '清单' }}：{{ error.message }}
        </div>
      </div>
    </template>

    <NAlert v-if="tableRows.length" size="small" type="default" :bordered="false">
      已从表格读取 {{ tableRows.length }} 行，其中 {{ droppedColumns.length }} 列不参与装机，会被忽略{{
        droppedColumnsText
      }}。
    </NAlert>
  </div>
</template>

<style scoped></style>
