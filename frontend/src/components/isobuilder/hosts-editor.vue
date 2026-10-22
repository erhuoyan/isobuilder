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
  /** 第 1 个网卡 MAC；单口时是唯一的网卡，bond 时是第 1 个 slave */
  mac1: string;
  /** 第 2 个网卡 MAC；只在 bond 时填，单口留空 */
  mac2: string;
  disk_target_size: string;
  root_password: string;
  bmc_ip: string;
  /** BMC 台账信息，不参与装机，但会随清单存库（后端原样保留） */
  bmc_mac: string;
  bmc_netmask: string;
  bmc_gateway: string;
  /** 机位/机房备注 */
  location: string;
}

/**
 * 可编辑的列，顺序即表格列顺序。
 *
 * MAC 列的语义由填写数量决定（与 kickstart 一致）：
 *   - 填 1 个 = 单口（network.mac）
 *   - 填 2 个 = bond（network.bond.slaves，mode 4）
 * 不需要用户自己区分模式。
 */
const EDITABLE_FIELDS: { key: keyof Omit<HostRow, 'uid'>; title: string; width: number; placeholder: string }[] = [
  { key: 'sn', title: 'SN', width: 150, placeholder: '必填，装机按 SN 匹配主机' },
  { key: 'hostname', title: '主机名', width: 130, placeholder: 'node-01' },
  { key: 'ip', title: 'IP', width: 135, placeholder: '172.16.0.11' },
  { key: 'netmask', title: '掩码', width: 125, placeholder: '255.255.255.0' },
  { key: 'gateway', title: '网关', width: 125, placeholder: '选填' },
  { key: 'mac1', title: 'MAC', width: 150, placeholder: '必填；bond 填第 1 个' },
  { key: 'mac2', title: 'MAC2', width: 150, placeholder: '仅 bond 填第 2 个网卡' },
  { key: 'disk_target_size', title: '磁盘 GB', width: 100, placeholder: '选填' },
  { key: 'root_password', title: 'root 密码', width: 130, placeholder: '选填' },
  { key: 'bmc_ip', title: 'BMC IP', width: 130, placeholder: '选填' },
  // 以下 4 列是台账信息：不参与装机，但会随清单存库，导出/导入都保留
  { key: 'bmc_mac', title: 'BMC MAC', width: 150, placeholder: '选填' },
  { key: 'bmc_netmask', title: 'BMC 掩码', width: 130, placeholder: '选填' },
  { key: 'bmc_gateway', title: 'BMC 网关', width: 130, placeholder: '选填' },
  { key: 'location', title: '机位', width: 120, placeholder: '选填' }
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
    mac1: '',
    mac2: '',
    disk_target_size: '',
    root_password: '',
    bmc_ip: '',
    bmc_mac: '',
    bmc_netmask: '',
    bmc_gateway: '',
    location: ''
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
    // 台账字段：不参与装机，但后端会把它们原样存进 hosts_json
    if (row.bmc_ip.trim()) lines.push(`    bmc_ip: ${row.bmc_ip.trim()}`);
    if (row.bmc_mac.trim()) lines.push(`    bmc_mac: ${row.bmc_mac.trim()}`);
    if (row.bmc_netmask.trim()) lines.push(`    bmc_netmask: ${row.bmc_netmask.trim()}`);
    if (row.bmc_gateway.trim()) lines.push(`    bmc_gateway: ${row.bmc_gateway.trim()}`);
    if (row.location.trim()) lines.push(`    location: ${yamlQuote(row.location.trim())}`);

    lines.push('    network:');
    lines.push(`      ip: ${row.ip.trim()}`);
    if (row.netmask.trim()) lines.push(`      netmask: ${row.netmask.trim()}`);
    if (row.gateway.trim()) lines.push(`      gateway: ${row.gateway.trim()}`);

    const mac1 = row.mac1.trim();
    const mac2 = row.mac2.trim();

    // 与 kickstart/Excel 模板一致：mac1+mac2 都填 = bond（2 个 slave），
    // 只填 mac1 = 单口。判据是数据本身，用户不需要选模式。
    const isBond = Boolean(mac1 && mac2);
    if (!mac1 && !mac2) {
      // 没有任何 MAC：后端会报「缺少网络绑定方式」，让错误标到这行
      lines.push('      mac: ');
    } else if (isBond) {
      lines.push('      bond:');
      lines.push('        mode: 4');
      lines.push('        slaves:');
      lines.push(`          - ${mac1}`);
      lines.push(`          - ${mac2}`);
    } else {
      lines.push(`      mac: ${mac1 || mac2}`);
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
            mac1: String(network.mac ?? '') || (slaves[0] ?? ''),
            mac2: slaves[1] ?? '',
            disk_target_size: String(host.disk_target_size ?? ''),
            root_password: String(host.root_password ?? ''),
            bmc_ip: String(host.bmc_ip ?? ''),
            bmc_mac: String(host.bmc_mac ?? ''),
            bmc_netmask: String(host.bmc_netmask ?? ''),
            bmc_gateway: String(host.bmc_gateway ?? ''),
            location: String(host.location ?? '')
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
        mac1: pick(row, 'mac') || pick(row, 'mac1'),
        mac2: pick(row, 'mac2'),
        disk_target_size: pick(row, 'disk_target_size'),
        root_password: pick(row, 'root_password'),
        bmc_ip: pick(row, 'bmc_ip'),
        bmc_mac: pick(row, 'bmc_mac'),
        bmc_netmask: pick(row, 'bmc_netmask'),
        bmc_gateway: pick(row, 'bmc_gateway'),
        location: pick(row, 'location')
      }))
      .filter(row => row.sn || row.hostname);

    appendParsed(parsed);
    window.$message?.success(`已从表格导入 ${parsed.length} 台主机`);
  } catch {
    window.$message?.error('解析表格失败，请确认为 .xlsx / .xls / .csv 格式');
  }
}

/**
 * 导出当前表格为 xlsx。
 *
 * 空表格导出的就是模板本身（只有表头行），用户下载后按列名填，
 * 再用「导入」读回来 —— 所以导出在空状态下也可用。
 */
function exportSpreadsheet() {
  const filled = rows.value.filter(isFilled);

  // 列顺序即导入协议的列顺序；空表也要有表头行，否则导出的不是模板
  const header = [
    'sn',
    'hostname',
    'ip',
    'netmask',
    'gateway',
    'mac1',
    'mac2',
    'disk_target_size',
    'root_password',
    'bmc_ip',
    'bmc_mac',
    'bmc_netmask',
    'bmc_gateway',
    'location'
  ];

  const data = filled.map(row => [
    row.sn,
    row.hostname,
    row.ip,
    row.netmask,
    row.gateway,
    row.mac1,
    row.mac2,
    row.disk_target_size,
    row.root_password,
    row.bmc_ip,
    row.bmc_mac,
    row.bmc_netmask,
    row.bmc_gateway,
    row.location
  ]);

  const sheet = XLSX.utils.aoa_to_sheet([header, ...data]);
  // 表头列宽按内容给足，避免打开是挤压的一列
  sheet['!cols'] = header.map(name => ({ wch: Math.max(12, name.length + 4) }));

  const workbook = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(workbook, sheet, '主机清单');
  XLSX.writeFile(workbook, 'isobuilder-主机清单.xlsx');
  window.$message?.success(
    filled.length ? `已导出 ${filled.length} 台主机` : '已导出空白模板，按列名填写后可用「导入」读回'
  );
}

const exportOptions = [
  { key: 'xlsx', label: 'Excel 表格（可再导回）' },
  { key: 'yaml', label: 'YAML 清单' }
];

function onExportSelect(key: string | number) {
  if (String(key) === 'yaml') exportYaml();
  else exportSpreadsheet();
}

/** 导出 YAML 文本清单 */
function exportYaml() {
  if (!hasContent.value) {
    window.$message?.warning('还没有主机');
    return;
  }

  const blob = new Blob([rowsToYaml(rows.value)], { type: 'text/yaml;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = 'hosts.yaml';
  link.click();
  URL.revokeObjectURL(url);
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
    const yaml = rowsToYaml(rows.value);

    /**
     * 组件挂载瞬间表格还是空的，但父组件可能已经传进来一份已保存的清单
     * （编辑页异步加载完成后才挂载本组件）。
     * 此时若把空表格回写，会把父组件的值覆盖成空串，
     * 下面的 value watch 就再也解析不到内容了 —— 表现为「编辑页主机载荷丢失」。
     */
    if (!yaml && value.value.trim()) return;

    value.value = yaml;

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

/**
 * 父组件（编辑页）加载已保存项目后会把 YAML 塞进 value；
 * 此时表格还是空的，需要解析回表格行——否则用户一打开编辑页
 * 就会把已保存的主机清单覆盖成空。
 *
 * 只解析本组件自己生成的结构（hosts/network/bond），其余字段忽略。
 */
watch(
  value,
  next => {
    const text = next ?? '';

    // 表格行生成的 YAML 与 value 一致，属于正常回流，不反向解析
    if (text === rowsToYaml(rows.value)) return;

    const parsed = text.trim() ? readYamlHosts(text) : [];
    if (parsed.length) {
      appendParsed(parsed);
    } else if (!text.trim()) {
      // 父组件真正清空（比如新建后重置）
      rows.value = [emptyRow()];
    }
    // text 有内容但解析不出行：保持表格现状，等用户用文本模式
  },
  { immediate: true }
);
</script>

<template>
  <div class="flex flex-col gap-3">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <span class="text-13px text-gray-500">
        每行一台主机；填 MAC 即单口，再填 MAC2 则为 bond
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
        <NDropdown trigger="click" :options="exportOptions" @select="onExportSelect">
          <NButton size="small" :disabled="false">
            <template #icon>
              <SvgIcon icon="mdi:file-download-outline" />
            </template>
            导出
          </NButton>
        </NDropdown>
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
              { class: 'flex flex-col gap-0.5' },
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
      :scroll-x="2100"
      :max-height="420"
      size="medium"
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
      <div v-if="globalErrors.length" class="flex flex-col gap-0.5">
        <div v-for="(error, index) in globalErrors" :key="index" class="text-12px text-red-500">
          清单级 · {{ error.field || '清单' }}：{{ error.message }}
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
  /**
   * 每行是一个可编辑输入框，Naive 默认单元格只包住控件本体，
   * 行与行之间几乎没有呼吸感。这里给单元格纵向留白，
   * 并补一条极浅的分隔线，密集数据才读得清。
   */
  :deep(.n-data-table-td) {
    padding: 10px 8px;
  }

  :deep(.n-data-table-tr:not(:last-child) > .n-data-table-td) {
    border-bottom: 1px solid rgb(239 240 245);
  }

  /* 表格头稍微压暗，与数据区拉开层次 */
  :deep(.n-data-table-th) {
    background-color: rgb(250 250 252);
  }
</style>
