<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useDebounceFn } from '@vueuse/core';
import * as XLSX from 'xlsx';
import { normalizeHeader, TEMPLATE_COLUMNS, TEMPLATE_HEADER_SET } from '@/constants/hosts-template';
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

/** ─── 表格导入（xlsx/csv）────────────────────────────────────────── */

interface SpreadsheetRow {
  [column: string]: string | number | undefined;
}


const tableInputRef = ref<HTMLInputElement | null>(null);
const tableFile = ref<File | null>(null);
const tableRows = ref<SpreadsheetRow[]>([]);
const tableHeaders = ref<string[]>([]);
const columnMapping = ref<Record<string, string>>({});
const tableShow = ref(false);
const tableParsing = ref(false);

function triggerTableSelect() {
  tableInputRef.value?.click();
}

/** bond 判定：mac1/mac2 同时映射到 → bond 模式；单 mac → 单口 */
function detectNetworkMode(mapping: Record<string, string>) {
  const fields = new Set(Object.values(mapping));
  const hasBond = fields.has('mac1') && fields.has('mac2');
  const hasSingle = fields.has('mac');
  return hasBond ? 'bond' : hasSingle ? 'single' : 'unknown';
}

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

/** 解析表格文件 */
async function handleTableFileChange(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = '';

  if (!file) return;

  tableFile.value = file;
  tableParsing.value = true;

  try {
    const buffer = await file.arrayBuffer();
    const workbook = XLSX.read(buffer, { type: 'array' });
    const sheet = workbook.Sheets[workbook.SheetNames[0]];
    const rows = XLSX.utils.sheet_to_json<SpreadsheetRow>(sheet, { defval: '' });

    if (!rows.length) {
      window.$message?.error('表格没有数据行');
      tableParsing.value = false;
      return;
    }

    const headers = Object.keys(rows[0]);
    const normalized = new Map(headers.map(h => [normalizeHeader(h), h]));

    const missing = TEMPLATE_COLUMNS.filter(col => !normalized.has(normalizeHeader(col)));
    if (missing.length) {
      window.$message?.error(
        `表头不符合模板：缺少列 ${missing.join('、')}。实际表头：${headers.join('、')}。请下载模板按列名填写（大小写/空格不敏感）。`
      );
      tableParsing.value = false;
      return;
    }

    // 把行数据按协议列重排：key = 归一化表头，值 = 原行对应单元格
    const columnByNormalized = new Map(headers.map(h => [normalizeHeader(h), h]));
    tableRows.value = rows.map(row => {
      const rebased: SpreadsheetRow = {};
      TEMPLATE_COLUMNS.forEach(col => {
        const original = columnByNormalized.get(normalizeHeader(col));
        rebased[col] = original ? row[original] : '';
      });
      // 非协议列原样保留（进 extra）
      headers.forEach(h => {
        if (!TEMPLATE_HEADER_SET.has(normalizeHeader(h))) {
          rebased[h] = row[h];
        }
      });
      return rebased;
    });

    tableHeaders.value = [...TEMPLATE_COLUMNS, ...headers.filter(h => !TEMPLATE_HEADER_SET.has(normalizeHeader(h)))];
    tableShow.value = true;
  } catch {
    window.$message?.error('解析表格失败，请确认为 .xlsx 或 .csv 格式');
  } finally {
    tableParsing.value = false;
  }
}

/** 映射确认 → 生成 YAML 写入编辑器 */
function handleTableConfirm() {
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

    const password = get('root_password');
    if (password) lines.push(`    root_password: ${password}`);

    const disk = get('disk_target_size');
    if (disk) lines.push(`    disk_target_size: ${disk}`);

    const bmcIp = get('bmc_ip');
    if (bmcIp) lines.push(`    bmc_ip: ${bmcIp}`);

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

    // 非协议列进 extra（保留原始表头名，如 bmc_mac、location）
    const extra: string[] = [];
    tableHeaders.value.forEach(column => {
      if (TEMPLATE_HEADER_SET.has(column)) return;
      const v = cellText(tableRows.value[index], column);
      if (v) extra.push(`    ${sanitizeKey(column)}: ${v}`);
    });
    if (extra.length) {
      lines.push('    extra:');
      lines.push(...extra);
    }
  });

  format.value = 'yaml';
  value.value = lines.join('\n') + '\n';
  tableShow.value = false;
  window.$message?.success(`已从表格生成 ${tableRows.value.length} 台主机的清单，请检查校验结果`);
}

function sanitizeKey(name: string): string {
  const key = String(name).trim().toLowerCase().replace(/[^a-z0-9_]+/g, '_').replace(/^_+|_+$/g, '');
  return key || 'column';
}

function handleTableCancel() {
  tableShow.value = false;
  tableFile.value = null;
  tableRows.value = [];
  tableHeaders.value = [];
  columnMapping.value = {};
}

function handleTemplateDownload() {
  const data = [TEMPLATE_COLUMNS];
  const sheet = XLSX.utils.aoa_to_sheet(data);
  const workbook = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(workbook, sheet, '主机清单');
  XLSX.writeFile(workbook, 'isobuilder-主机清单模板.xlsx');
}


/** 预览表：前 10 行、前 12 列 */
const tablePreviewColumns = computed(() =>
  tableHeaders.value.map(header => ({
    key: header,
    title: header,
    width: Math.max(90, Math.min(220, String(header).length * 14 + 24)),
    render: (row: SpreadsheetRow) =>
      h('span', { class: 'font-mono text-12px whitespace-nowrap' }, cellText(row, header))
  }))
);

const tablePreviewRows = computed(() => tableRows.value.slice(0, 10));

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
        <NButton size="small" @click="triggerTableSelect">
          <template #icon>
            <SvgIcon icon="mdi:table-arrow-right" />
          </template>
          导入表格
        </NButton>
        <NButton size="small" @click="handleTemplateDownload">
          <template #icon>
            <SvgIcon icon="mdi:file-download-outline" />
          </template>
          下载模板
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

    <input
      ref="tableInputRef"
      type="file"
      accept=".xlsx,.xls,.csv"
      class="hidden"
      @change="handleTableFileChange"
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

    <NDrawer v-model:show="tableShow" placement="right" width="min(100vw, 860px)">
      <NDrawerContent :title="`表格导入 · ${tableFile?.name ?? ''}`" :native-scrollbar="false" closable>
        <NAlert type="info" :bordered="false" class="mb-16px">
          共解析出 {{ tableRows.length }} 行、{{ tableHeaders.length }} 列。
          未出现在模板协议里的列（如 bmc_mac、location）会作为附加信息保留到主机记录。
        </NAlert>

        <div class="mb-8px text-13px text-gray-500">数据预览（前 10 行）：</div>
        <NDataTable
          size="small"
          bordered
          :max-height="240"
          :columns="tablePreviewColumns"
          :data="tablePreviewRows"
          :row-key="(row: SpreadsheetRow) => tableRows.indexOf(row)"
        />

        <template #footer>
          <NSpace justify="end">
            <NButton @click="handleTableCancel">取消</NButton>
            <NButton type="primary" :loading="tableParsing" @click="handleTableConfirm">生成主机清单</NButton>
          </NSpace>
        </template>
      </NDrawerContent>
    </NDrawer>

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
