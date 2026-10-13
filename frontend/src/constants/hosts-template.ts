/**
 * 表格导入的列协议：xlsx 模板的表头必须与这里完全一致（导入按表头严格匹配，
 * 区分大小写，仅容忍表头单元格前后的空格）。
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
  'root_password',
  'bmc_ip',
  'bmc_mac',
  'bmc_netmask',
  'bmc_gateway',
  'location'
];

export const TEMPLATE_HEADER_SET = new Set(TEMPLATE_COLUMNS);

/**
 * 预览展示的列：仅 kickstart 装机时真正消费的协议列（本机识别 + 网络 + 磁盘 + 密码）。
 * bmc_*、location 等台账列与协议外的附加列不在预览里出现，也不会写入生成的清单。
 */
export const PREVIEW_COLUMNS = [
  'sn',
  'hostname',
  'ip',
  'netmask',
  'gateway',
  'mac1',
  'mac2',
  'disk_target_size',
  'root_password'
];

export const PREVIEW_COLUMN_SET = new Set(PREVIEW_COLUMNS);
