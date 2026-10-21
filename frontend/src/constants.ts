/** 业务常量：状态文案与配色，前后端枚举一一对应。 */

import type { BuildStage, BuildStatus, IsoSource, IsoStatus, PayloadMode } from '@/services/types';

export const ARCH_OPTIONS = [
  { label: 'x86_64', value: 'x86_64' },
  { label: 'aarch64', value: 'aarch64' },
];

export const KICKSTART_ARCH_OPTIONS = [{ label: '通用（any）', value: 'any' }, ...ARCH_OPTIONS];

export const HOSTS_FORMAT_OPTIONS = [
  { label: '自动识别', value: 'auto' },
  { label: 'YAML', value: 'yaml' },
  { label: 'JSON', value: 'json' },
];

export const PAYLOAD_MODE_OPTIONS: { label: string; value: PayloadMode; description: string }[] = [
  { label: '不注入（默认）', value: 'off', description: '不注入任何逻辑' },
  { label: '按卷标查找', value: 'label', description: '只按卷标 ISO_PAYLOAD 查找 U 盘分区，找不到就跳过' },
  { label: '卷标优先并扫描', value: 'auto', description: '卷标优先；未命中时扫描非目标磁盘的分区' },
];

export const BUILD_STAGES: { key: BuildStage; label: string; description: string }[] = [
  { key: 'mount', label: '提取原版', description: '挂载并复制原版 ISO 的内容' },
  { key: 'inject', label: '注入应答', description: '写入 kickstart 与主机清单 JSON' },
  { key: 'boot', label: '改写引导', description: '修改 grub.cfg / isolinux.cfg' },
  { key: 'build', label: '重建镜像', description: 'xorriso 重建并 isohybrid' },
  { key: 'seal', label: '校验上传', description: 'implantisomd5 并上传对象存储' },
];

export const BUILD_STATUS: Record<BuildStatus, { label: string; color: string }> = {
  queued: { label: '排队中', color: 'default' },
  running: { label: '构建中', color: 'processing' },
  succeeded: { label: '已完成', color: 'success' },
  failed: { label: '失败', color: 'error' },
  canceled: { label: '已取消', color: 'warning' },
};

export const ISO_STATUS: Record<IsoStatus, { label: string; color: string }> = {
  downloading: { label: '下载中', color: 'processing' },
  hashing: { label: '校验中', color: 'warning' },
  ready: { label: '可用', color: 'success' },
  error: { label: '异常', color: 'error' },
};

export const ISO_SOURCE: Record<IsoSource, { label: string; color: string }> = {
  registered: { label: '本地注册', color: 'default' },
  url: { label: '链接导入', color: 'orange' },
};

/** 已经进入终态的构建不再轮询。 */
export function isBuildActive(status: BuildStatus) {
  return status === 'queued' || status === 'running';
}
