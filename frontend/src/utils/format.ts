import dayjs from 'dayjs';

/** the placeholder of an unknown value */
export const EMPTY_VALUE = '—';

/**
 * Format a byte size
 *
 * @param bytes the number of bytes
 * @param digits the number of decimals
 */
export function formatBytes(bytes?: number | null, digits = 1) {
  if (bytes === null || bytes === undefined || Number.isNaN(bytes)) return EMPTY_VALUE;

  if (bytes === 0) return '0 B';

  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / 1024 ** index;

  return `${value.toFixed(index === 0 ? 0 : digits)} ${units[index]}`;
}

/** Format a byte size per second, used by the upload speed */
export function formatSpeed(bytesPerSecond: number) {
  if (!bytesPerSecond || bytesPerSecond < 0) return '0 B/s';

  return `${formatBytes(bytesPerSecond, 1)}/s`;
}

/** Format a RFC3339 timestamp in the local timezone */
export function formatDateTime(value?: string | null, template = 'YYYY-MM-DD HH:mm:ss') {
  if (!value) return EMPTY_VALUE;

  const time = dayjs(value);

  return time.isValid() ? time.format(template) : EMPTY_VALUE;
}

/** Format the duration between two RFC3339 timestamps */
export function formatDuration(start?: string | null, end?: string | null) {
  if (!start) return EMPTY_VALUE;

  const from = dayjs(start);
  const to = end ? dayjs(end) : dayjs();

  if (!from.isValid() || !to.isValid()) return EMPTY_VALUE;

  const seconds = Math.max(0, to.diff(from, 'second'));

  if (seconds < 60) return `${seconds} 秒`;

  const minutes = Math.floor(seconds / 60);
  const restSeconds = seconds % 60;

  if (minutes < 60) return restSeconds ? `${minutes} 分 ${restSeconds} 秒` : `${minutes} 分`;

  const hours = Math.floor(minutes / 60);
  const restMinutes = minutes % 60;

  return restMinutes ? `${hours} 小时 ${restMinutes} 分` : `${hours} 小时`;
}

/** Format a number with thousand separators */
export function formatNumber(value?: number | null) {
  if (value === null || value === undefined) return EMPTY_VALUE;

  return value.toLocaleString('zh-CN');
}
