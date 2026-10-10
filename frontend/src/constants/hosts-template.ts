/**
 * 表格导入的列协议：xlsx 模板的表头必须与这里完全一致（导入按表头严格匹配）。
 * 模板之外的列会作为附加信息保留到主机记录的 extra。
 */
export const TEMPLATE_COLUMNS = [
  'sn',
  'hostname',
  'ip',
  'netmask',
  'gateway',
  'mac1',
  'mac2',
  'disk_target_size',
  'bmc_ip',
  'bmc_mac',
  'bmc_netmask',
  'bmc_gateway',
  'location'
];

export const TEMPLATE_HEADER_SET = new Set(TEMPLATE_COLUMNS);

/** 表头归一化：小写、去空格/下划线/连字符（用户的 `IP`/`Mac_1` → `ip`/`mac1`） */
export function normalizeHeader(header: string): string {
  return header.trim().toLowerCase().replace(/[\s_-]+/g, '');
}
