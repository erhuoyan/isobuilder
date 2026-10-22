<script setup lang="ts">
import { computed, h, ref, watch } from 'vue';
import { NAlert, NButton, NDataTable, NEmpty, NInput, NSpace } from 'naive-ui';
import { useDebounceFn } from '@vueuse/core';
import * as XLSX from 'xlsx';
import { fetchValidateHosts } from '@/service/api';

/** the idle time before the inventory is validated automatically */
const VALIDATE_DEBOUNCE = 500;

/** 导入接受的文件类型：文本清单与电子表格共用一个入口 */
const FILE_ACCEPT = '.yaml,.yml,.json,.txt,.xlsx,.xls,.csv';

/** 表格里可编辑的一行主机 */
interface HostRow {
  /** 稳定 key，供虚拟滚动与增删定位使用 */
  uid: number;
  sn: string;
  hostname: string;
  ip: string;
  netmask: string;
  gateway: string;
  mac: string;
  /** bond 模式用两个 MAC；为空表示单口 */
  mac1: string;
  mac2: string;
  disk_target_size: string;
  root_password: string;
  bmc_ip: string;
}

/** 可编辑的列，顺序即表格列顺序 */
const EDITABLE_FIELDS: { key: keyof Omit<HostRow, 'uid'>; title: string; width: number; placeholder: string }[] = [
  { key: 'sn', title: 'SN', width: 150, placeholder: '必填，装机时按它匹配' },
  { key: 'hostname', title: '主机名', width: 130, placeholder: 'node-01' },
  { key: 'ip', title: 'IP', width: 135, placeholder: '172.16.0.11' },
  { key: 'netmask', title: '掩码', width: 125, placeholder: '255.255.255.0' },
  { key: 'gateway', title: '网关', width: 135, placeholder: '选填' },
  { key: 'mac', title: 'MAC（单口）', width: 150, placeholder: '单口填这里' },
  { key: 'mac1', title: 'MAC1（bond）', width: 150, placeholder: 'bond 用' },
  { key: 'mac2', title: 'MAC2（bond）', width: 150, placeholder: 'bond 用' },
  { key: 'disk_target_size', title: '目标磁盘 GB', width: 110, placeholder: '选填' },
  { key: 'root_password', title: 'root 密码', width: 140, placeholder: '选填' },
  { key: 'bmc_ip', title: 'BMC IP', width: 130, placeholder: '选填' }
];

const value = defineModel<string>('value', { required: true });

const emit = defineEmits<{
  validate: [result: Api.IsoBuilder.ValidateHostsResult | null];
}>();

const result = ref<Api.IsoBuilder.ValidateHostsResult | null>(null);
const validating = ref(false);
const validatedContent = ref('');
const requestFailed = ref(false);
const fileInputRef = ref<HTMLInputElement | null>(null);

let requestSeq = 0;
let uidSeq = 0;

/** 表格里的主机行，这是本组件的唯一数据源 */
const rows = ref<HostRow[]>([]);

const hasContent = computed(() => rows.value.some(row => row.sn.trim() || row.hostname.trim()));

/**
 * 写入单元格。
 *
 * 模板里的 `rows` 已被自动解包成数组，拿不到 .value，
 * 所以单元格编辑走这个函数，由它负责触发深层监听。
 */
function setCell(row: HostRow, key: keyof Omit<HostRow, 'uid'>, next: string) {
  row[key] = next;
  // 换一个新数组引用，让 watch 的 deep 监听立即触发
  rows.value = [...rows.value];
}

function emptyRow(): HostRow {
  uidSeq += 1;
  return {
    uid: uidSeq,
    sn: '',
    hostname: '',
    ip: '',
    netmask: '',
    gateway: '',
    mac: '',
    mac1: '',
    mac2: '',
    disk_target_size: '',
    root_password: '',
    bmc_ip: ''
  };
}

/* ─── 表格行 ←→ 主机清单文本 ─────────────────────────────────────── */

/** 一行是否填写了有效内容（空行不写入清单） */
function isFilled(row: HostRow) {
  return Boolean(row.sn.trim() || row.hostname.trim() || row.ip.trim());
}

/** YAML 双引号风格转义：兼容密码里的 : # " \ 等字符 */
function yamlQuote(text: string): string {
  return `"${text.replace(/\\/g, '\\\\').replace(/"/g, '\\"')}"`;
}

/**
 * 把表格行渲染成后端接受的 YAML。
 *
 * 网络模式由 MAC 列决定：mac1/mac2 都填 → bond；否则用单口 mac。
 * 后端要求 mac 与 bond 二选一，这里按填了哪组自动选择，避免用户填错。
 */
function rowsToYaml(list: HostRow[]): string {
  const filled = list.filter(isFilled);
  if (!filled.length) return '';

  const lines: string[] = ['hosts:'];
  let index = 0;

  filled.forEach((row, i) => {
    index = i + 1;
    lines.push(`  - sn: ${yamlQuote(row.sn.trim() || `AUTO-${index}`)}`);
    lines.push(`    hostname: ${row.hostname.trim() || `node-${index}`}`);

    if (row.root_password.trim()) lines.push(`    root_password: ${yamlQuote(row.root_password.trim())}`);
    if (row.disk_target_size.trim()) lines.push(`    disk_target_size: ${row.disk_target_size.trim()}`);
    if (row.bmc_ip.trim()) lines.push(`    bmc_ip: ${row.bmc_ip.trim()}`);

    lines.push('    network:');
    lines.push(`      ip: ${row.ip.trim()}`);
    if (row.netmask.trim()) lines.push(`      netmask: ${row.netmask.trim()}`);
    if (row.gateway.trim()) lines.push(`      gateway: ${row.gateway.trim()}`);

    const useBond = Boolean(row.mac1.trim() && row.mac2.trim());
    if (useBond) {
      lines.push('      bond:');
      lines.push('        mode: 4');
      lines.push('        slaves:');
      lines.push(`          - ${row.mac1.trim()}`);
      lines.push(`          - ${row.mac2.trim()}`);
    } else {
      lines.push(`      mac: ${row.mac.trim()}`);
    }
  });

  return `${lines.join('\n')}\n`;
}

/** 从一行 YAML 记录里取值（只支持本组件生成的结构） */
function readYamlHosts(text: string): Partial<HostRow>[] {
  const out: Partial<HostRow>[] = [];
  let current: Partial<HostRow> | null = null;
  let inNetwork = false;
  let inBond = false;
  let inSlaves = false;
  const slaves: string[] = [];

  const unquote = (raw: string) => {
    const t = raw.trim();
    if (t.startsWith('"') && t.endsWith('"')) {
      return t.slice(1, -1).replace(/\\"/g, '"').replace(/\\\\/g, '\\');
    }
    return t;
  };

  const flush = () => {
    if (!current) return;
    if (slaves.length >= 2) {
      current.mac1 = slaves[0];
      current.mac2 = slaves[1];
    }
    out.push(current);
    slaves.length = 0;
    current = null;
  };

  text.split('\n').forEach(rawLine => {
    const line = rawLine.replace(/\r$/, '');
    if (!line.trim() || line.trim().startsWith('#')) return;

    const indent = line.length - line.trimStart().length;
    const body = line.trim();

    if (body === 'hosts:') return;

    if (indent === 2 && body.startsWith('- ')) {
      flush();
      current = {};
      inNetwork = false;
      inBond = false;
      inSlaves = false;
      const [k, ...rest] = body.slice(2).split(':');
      if (current && k) (current as Record<string, string>)[k.trim()] = unquote(rest.join(':'));
      return;
    }

    if (!current) return;

    if (indent === 4) {
      inNetwork = body.startsWith('network:');
      inBond = false;
      inSlaves = false;
      if (!inNetwork) {
        const [k, ...rest] = body.split(':');
        (current as Record<string, string>)[k.trim()] = unquote(rest.join(':'));
      }
      return;
    }

    if (indent === 6 && inNetwork) {
      if (body.startsWith('bond:')) {
        inBond = true;
        return;
      }
      inBond = false;
      inSlaves = false;
      const [k, ...rest] = body.split(':');
      (current as Record<string, string>)[k.trim()] = unquote(rest.join(':'));
      return;
    }

    if (indent === 8 && inBond) {
      if (body.startsWith('slaves:')) {
        inSlaves = true;
        return;
      }
      inSlaves = false;
      return;
    }

    if (indent >= 10 && inSlaves && body.startsWith('- ')) {
      slaves.push(unquote(body.slice(2)));
    }
  });

  flush();
  return out;
}

/** 把解析出的一批主机追加到表格 */
function appendParsed(list: Partial<HostRow>[]) {
  const mapped: HostRow[] = list.map(item => ({
    ...emptyRow(),
    ...Object.fromEntries(
      Object.entries(item).filter(([, v]) => typeof v === 'string' && v !== '')
    )
  }));
  rows.value = [...rows.value, ...mapped];
}

/* ─── 加行 / 删行 ────────────────────────────────────────────────── */

function addRow() {
  rows.value = [...rows.value, emptyRow()];
}

function addRows(count: number) {
  const next: HostRow[] = [];
  for (let i = 0; i < count; i += 1) next.push(emptyRow());
  rows.value = [...rows.value, ...next];
}

function removeRow(uid: number) {
  rows.value = rows.value.filter(row => row.uid !== uid);
}

function clearAll() {
  rows.value = [];
}

/** 复制某一行（同批次主机网络参数常常只差 IP/SN） */
function duplicateRow(uid: number) {
  const source = rows.value.find(row => row.uid === uid);
  if (!source) return;
  const copy: HostRow = { ...source, uid: (uidSeq += 1), sn: '', hostname: '', ip: '' };
  const index = rows.value.findIndex(row => row.uid === uid);
  rows.value = [...rows.value.slice(0, index + 1), copy, ...rows.value.slice(index + 1)];
}

/* ─── 导入 / 导出 ────────────────────────────────────────────────── */

function triggerFileSelect() {
  fileInputRef.value?.click();
}

async function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = '';
  if (!file) return;

  const name = file.name.toLowerCase();
  const isSpreadsheet = name.endsWith('.xlsx') || name.endsWith('.xls') || name.endsWith('.csv');

  if (isSpreadsheet) {
    await importSpreadsheet(file);
    return;
  }

  const text = await file.text();

  // JSON 也接受：后端支持裸数组或 {hosts:[...]}，这里统一转成表格行
  if (name.endsWith('.json')) {
    try {
      const parsed = JSON.parse(text);
      const list: Record<string, unknown>[] = Array.isArray(parsed) ? parsed : parsed.hosts ?? [];
      appendParsed(
        list.map(host => {
          const network = (host.network ?? {}) as Record<string, unknown>;
          const bond = (network.bond ?? {}) as Record<string, unknown>;
          const slaves = (bond.slaves ?? []) as string[];
          return {
            sn: String(host.sn ?? ''),
            hostname: String(host.hostname ?? ''),
            ip: String(network.ip ?? ''),
            netmask: String(network.netmask ?? ''),
            gateway: String(network.gateway ?? ''),
            mac: String(network.mac ?? ''),
            mac1: slaves[0] ?? '',
            mac2: slaves[1] ?? '',
            disk_target_size: String(host.disk_target_size ?? ''),
            root_password: String(host.root_password ?? ''),
            bmc_ip: String(host.bmc_ip ?? '')
          };
        })
      );
      window.$message?.success(`已导入 ${list.length} 台主机`);
    } catch {
      window.$message?.error('JSON 解析失败，请检查格式');
    }
    return;
  }

  const parsed = readYamlHosts(text);
  if (!parsed.length) {
    window.$message?.error('没有从文件里识别出主机，请确认是 hosts: 开头的 YAML');
    return;
  }
  appendParsed(parsed);
  window.$message?.success(`已导入 ${parsed.length} 台主机`);
}

/** 表格文件 → 行（列名与模板列一致，区分大小写，容忍表头前后空格） */
async function importSpreadsheet(file: File) {
  try {
    const buffer = await file.arrayBuffer();
    const workbook = XLSX.read(buffer, { type: 'array' });
    const sheet = workbook.Sheets[workbook.SheetNames[0]];
    const raw = XLSX.utils.sheet_to_json<Record<string, string | number>>(sheet, { defval: '' });

    if (!raw.length) {
      window.$message?.error('表格没有数据行');
      return;
    }

    const headers = Object.keys(raw[0]);
    const pick = (row: Record<string, string | number>, field: string) => {
      const key = headers.find(item => item.trim() === field);
      return key ? String(row[key] ?? '').trim() : '';
    };

    const parsed = raw
      .map(row => ({
        sn: pick(row, 'sn'),
        hostname: pick(row, 'hostname'),
        ip: pick(row, 'ip'),
        netmask: pick(row, 'netmask'),
        gateway: pick(row, 'gateway'),
        mac: pick(row, 'mac'),
        mac1: pick(row, 'mac1'),
        mac2: pick(row, 'mac2'),
        disk_target_size: pick(row, 'disk_target_size'),
        root_password: pick(row, 'root_password'),
        bmc_ip: pick(row, 'bmc_ip')
      }))
      .filter(row => row.sn || row.hostname);

    appendParsed(parsed);
    window.$message?.success(`已从表格导入 ${parsed.length} 台主机`);
  } catch {
    window.$message?.error('解析表格失败，请确认为 .xlsx / .xls / .csv 格式');
  }
}

/** 导出：电子表格（便于 Excel 编辑后再导回） */
function exportSpreadsheet() {
  if (!hasContent.value) {
    window.$message?.warning('还没有主机');
    return;
  }

  const data = rows.value.filter(isFilled).map(row => ({
    sn: row.sn,
    hostname: row.hostname,
    ip: row.ip,
    netmask: row.netmask,
    gateway: row.gateway,
    mac: row.mac,
    mac1: row.mac1,
    mac2: row.mac2,
    disk_target_size: row.disk_target_size,
    root_password: row.root_password,
    bmc_ip: row.bmc_ip
  }));

  const sheet = XLSX.utils.json_to_sheet(data);
  const workbook = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(workbook, sheet, '主机清单');
  XLSX.writeFile(workbook, 'isobuilder-主机清单.xlsx');
}

/* ─── 校验 ───────────────────────────────────────────────────────── */

const currentYaml = computed(() => rowsToYaml(rows.value));

const stale = computed(
  () => hasContent.value && !validating.value && currentYaml.value !== validatedContent.value
);

const hostCount = computed(() => (result.value?.ok ? result.value.hosts_count : 0));

function invalidate() {
  requestSeq += 1;
  validatedContent.value = '';
  validating.value = false;
  requestFailed.value = false;
  emit('validate', null);
}

/** 把后端的错误按行归位，标到对应单元格上 */
const errorsByUid = computed(() => {
  const map: Record<number, string[]> = {};
  if (!result.value || result.value.ok) return map;

  const filled = rows.value.filter(isFilled);
  result.value.errors.forEach(error => {
    // index 是清单里的位置，映射回表格行
    const row = error.index >= 0 ? filled[error.index] : undefined;
    if (row) {
      const field = error.field.replace(/^network\./, '').replace(/^network$/, '网络');
      (map[row.uid] ??= []).push(`${field || '清单'}：${error.message}`);
    }
  });
  return map;
});

/** 清单级错误（不属于某一行的） */
const globalErrors = computed(() =>
  (result.value?.errors ?? []).filter(error => error.index < 0)
);

async function runValidate() {
  const content = currentYaml.value;

  if (!content.trim()) {
    result.value = null;
    invalidate();
    return;
  }

  if (content === validatedContent.value && result.value) {
    emit('validate', result.value);
    return;
  }

  const seq = ++requestSeq;
  validating.value = true;

  const { data, error } = await fetchValidateHosts(content, 'yaml');

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

// 任何行内容变化都重新校验，并把 YAML 同步给父组件
watch(
  rows,
  () => {
    value.value = rowsToYaml(rows.value);

    if (!hasContent.value) {
      result.value = null;
      invalidate();
      return;
    }

    invalidate();
    debouncedValidate();
  },
  { deep: true, immediate: true }
);
</script>

<template>
  <div class="flex flex-col gap-12px">
    <div class="flex flex-wrap items-center justify-between gap-8px">
      <span class="text-13px text-gray-500">
        每行一台主机，可直接编辑；MAC1/MAC2 都填则按 bond 装机
      </span>

      <NSpace :size="8">
        <NButton size="small" type="primary" @click="addRow">
          <template #icon>
            <SvgIcon icon="mdi:plus" />
          </template>
          添加一行
        </NButton>
        <NButton size="small" @click="addRows(5)">
          <template #icon>
            <SvgIcon icon="mdi:table-row-plus-after" />
          </template>
          添加 5 行
        </NButton>
        <NButton size="small" @click="triggerFileSelect">
          <template #icon>
            <SvgIcon icon="mdi:file-upload-outline" />
          </template>
          导入
        </NButton>
        <NButton size="small" :disabled="!hasContent" @click="exportSpreadsheet">
          <template #icon>
            <SvgIcon icon="mdi:microsoft-excel" />
          </template>
          导出
        </NButton>
        <NButton size="small" :disabled="!hasContent" @click="clearAll">
          <template #icon>
            <SvgIcon icon="mdi:eraser" />
          </template>
          清空
        </NButton>
      </NSpace>
    </div>

    <input ref="fileInputRef" type="file" :accept="FILE_ACCEPT" class="hidden" @change="handleFileChange" />

    <NDataTable
      :columns="[
        {
          key: 'index',
          title: '#',
          width: 50,
          render: (_row: HostRow, index: number) => h('span', { class: 'text-12px text-gray-400' }, String(index + 1))
        },
        ...EDITABLE_FIELDS.map(field => ({
          key: field.key,
          title: field.title,
          width: field.width,
          render: (row: HostRow) =>
            h(NInput, {
              value: row[field.key],
              size: 'small',
              placeholder: field.placeholder,
              'onUpdate:value': (next: string) => setCell(row, field.key, next)
            })
        })),
        {
          key: 'errors',
          title: '问题',
          width: 200,
          render: (row: HostRow) => {
            const list = errorsByUid[row.uid] ?? [];
            if (!list.length) return h('span', { class: 'text-12px text-green-600' }, '✓');
            return h(
              'div',
              { class: 'flex flex-col gap-2px' },
              list.map(msg => h('div', { class: 'text-12px text-red-500' }, msg))
            );
          }
        },
        {
          key: 'actions',
          title: '操作',
          width: 110,
          fixed: 'right',
          render: (row: HostRow) =>
            h(NSpace, { size: 4, wrap: false }, () => [
              h(
                NButton,
                { size: 'tiny', text: true, type: 'primary', onClick: () => duplicateRow(row.uid) },
                () => '复制'
              ),
              h(
                NButton,
                { size: 'tiny', text: true, type: 'error', onClick: () => removeRow(row.uid) },
                () => '删除'
              )
            ])
        }
      ]"
      :data="rows"
      :row-key="(row: HostRow) => row.uid"
      :scroll-x="1700"
      :max-height="420"
      size="small"
      :bordered="false"
      :single-line="false"
    >
      <template #empty>
        <div class="py-8">
          <NEmpty description="还没有主机，点「添加一行」开始填，或从 Excel / YAML 导入" />
        </div>
      </template>
    </NDataTable>

    <NAlert v-if="validating" size="small" type="info" :bordered="false">正在校验主机清单…</NAlert>

    <NAlert v-else-if="requestFailed" size="small" type="error" :bordered="false">
      校验请求失败，请检查内容后重试。
    </NAlert>

    <NAlert v-else-if="stale" size="small" type="warning" :bordered="false">
      清单已修改，等待自动校验…
    </NAlert>

    <template v-else-if="result?.ok">
      <NAlert size="small" type="success" :bordered="false">
        校验通过：{{ hostCount }} 台主机，可以保存了
      </NAlert>
    </template>

    <template v-else-if="result">
      <NAlert size="small" type="error" :bordered="false">
        校验未通过，共 {{ result.errors.length }} 处问题（已在表格「问题」列标出）
      </NAlert>
      <div v-if="globalErrors.length" class="flex flex-col gap-2px">
        <div v-for="(error, index) in globalErrors" :key="index" class="text-12px text-red-500">
          清单级 · {{ error.field || '清单' }}：{{ error.message }}
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped></style>
