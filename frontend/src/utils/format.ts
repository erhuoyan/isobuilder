/** 展示用的格式化工具。 */

export function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '—';

  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let value = bytes;
  let unit = 0;

  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }

  return `${value >= 100 ? value.toFixed(0) : value.toFixed(1)} ${units[unit]}`;
}

export function formatDateTime(value: string): string {
  if (!value) return '—';

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;

  const pad = (n: number) => String(n).padStart(2, '0');

  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(
    date.getHours(),
  )}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

/** 「3 分钟前」这类相对时间，用于轮询中的构建。 */
export function formatRelative(value: string): string {
  if (!value) return '—';

  const diff = Date.now() - new Date(value).getTime();
  if (Number.isNaN(diff)) return value;
  if (diff < 0) return '刚刚';

  const minutes = Math.floor(diff / 60000);
  if (minutes < 1) return '刚刚';
  if (minutes < 60) return `${minutes} 分钟前`;

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时前`;

  return `${Math.floor(hours / 24)} 天前`;
}

/** 构建耗时：未结束的不算。 */
export function formatDuration(startedAt: string, finishedAt: string): string {
  if (!startedAt || !finishedAt) return '—';

  const ms = new Date(finishedAt).getTime() - new Date(startedAt).getTime();
  if (Number.isNaN(ms) || ms < 0) return '—';

  const seconds = Math.floor(ms / 1000);
  if (seconds < 60) return `${seconds} 秒`;

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes} 分 ${seconds % 60} 秒`;

  return `${Math.floor(minutes / 60)} 时 ${minutes % 60} 分`;
}
