import type { SelectOption } from 'naive-ui';

/** the 5 stages of a build, the index is `stage_index` - 1 */
export const buildStages: { key: Api.IsoBuilder.BuildStage; label: string; description: string }[] = [
  { key: 'mount', label: '提取原版', description: '挂载并复制原版 ISO 的内容' },
  { key: 'inject', label: '注入应答', description: '写入 kickstart 与主机清单 JSON' },
  { key: 'boot', label: '改写引导', description: '修改 grub.cfg / isolinux.cfg' },
  { key: 'build', label: '重建镜像', description: 'xorriso 重建并 isohybrid' },
  { key: 'seal', label: '校验上传', description: 'implantisomd5 并上传对象存储' }
];

/** the display name and the naive-ui tag type of each build status */
export const buildStatusRecord: Record<
  Api.IsoBuilder.BuildStatus,
  { label: string; type: 'default' | 'info' | 'success' | 'warning' | 'error' }
> = {
  queued: { label: '排队中', type: 'default' },
  running: { label: '构建中', type: 'info' },
  succeeded: { label: '已完成', type: 'success' },
  failed: { label: '失败', type: 'error' },
  canceled: { label: '已取消', type: 'warning' }
};

/** the display name and the naive-ui tag type of each iso status */
export const isoStatusRecord: Record<
  Api.IsoBuilder.IsoStatus,
  { label: string; type: 'default' | 'info' | 'success' | 'warning' | 'error' }
> = {
  downloading: { label: '下载中', type: 'info' },
  hashing: { label: '校验中', type: 'warning' },
  ready: { label: '可用', type: 'success' },
  error: { label: '异常', type: 'error' }
};

/**
 * The display name and tag type of each ISO source.
 *
 * Shared by the library table and the project wizard so a source can never be
 * labelled differently in the two places that show it.
 */
export const isoSourceRecord: Record<Api.IsoBuilder.IsoSource, { label: string; type: 'default' | 'info' | 'warning' }> = {
  registered: { label: '本地注册', type: 'default' },
  url: { label: '链接导入', type: 'warning' }
};

/** the arch options */
export const archOptions: SelectOption[] = [
  { label: 'x86_64', value: 'x86_64' },
  { label: 'aarch64', value: 'aarch64' }
];

/** the kickstart arch options */
export const kickstartArchOptions: SelectOption[] = [{ label: '通用（any）', value: 'any' }, ...archOptions];

/** the hosts format options of the inventory editor */
export const hostsFormatOptions: SelectOption[] = [
  { label: '自动识别', value: 'auto' },
  { label: 'YAML', value: 'yaml' },
  { label: 'JSON', value: 'json' }
];
