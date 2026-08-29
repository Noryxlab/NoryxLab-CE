/** Presentation helpers. Every raw value the API returns passes through here
 *  before it reaches a screen, so units and dates read the same everywhere. */

const BYTE_UNITS = ['B', 'Ko', 'Mo', 'Go', 'To', 'Po'] as const;
const BYTE_UNITS_EN = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'] as const;

export function formatBytes(bytes: number | null | undefined, locale: 'fr' | 'en' = 'fr'): string {
  if (bytes === null || bytes === undefined || Number.isNaN(bytes)) return '—';
  const units = locale === 'fr' ? BYTE_UNITS : BYTE_UNITS_EN;
  if (bytes === 0) return `0 ${units[0]}`;
  const exponent = Math.min(Math.floor(Math.log(Math.abs(bytes)) / Math.log(1024)), units.length - 1);
  const value = bytes / 1024 ** exponent;
  const digits = value >= 100 || exponent === 0 ? 0 : value >= 10 ? 1 : 2;
  return `${value.toFixed(digits)} ${units[exponent]}`;
}

/** Kubernetes quantity (e.g. "10Gi", "500m") -> human label. */
export function formatQuantity(quantity: string | null | undefined): string {
  if (!quantity) return '—';
  const match = /^(\d+(?:\.\d+)?)(Ki|Mi|Gi|Ti|K|M|G|T|m)?$/.exec(quantity.trim());
  if (!match) return quantity;
  const amount = Number(match[1]);
  switch (match[2]) {
    case 'Ki':
      return formatBytes(amount * 1024);
    case 'Mi':
      return formatBytes(amount * 1024 ** 2);
    case 'Gi':
      return formatBytes(amount * 1024 ** 3);
    case 'Ti':
      return formatBytes(amount * 1024 ** 4);
    case 'm':
      return `${(amount / 1000).toFixed(2)} vCPU`;
    default:
      return quantity;
  }
}

/** Millicores -> "1,5 vCPU". The old UI printed raw "500m" at the user. */
export function formatCpu(millicores: number | null | undefined, locale: 'fr' | 'en' = 'fr'): string {
  if (millicores === null || millicores === undefined) return '—';
  const cores = millicores / 1000;
  const formatted = new Intl.NumberFormat(locale === 'fr' ? 'fr-FR' : 'en-US', {
    maximumFractionDigits: 2,
  }).format(cores);
  return `${formatted} vCPU`;
}

export function parseDate(value: string | number | Date | null | undefined): Date | null {
  if (value === null || value === undefined || value === '') return null;
  const date = value instanceof Date ? value : new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

export function formatDateTime(
  value: string | number | Date | null | undefined,
  locale: 'fr' | 'en' = 'fr',
): string {
  const date = parseDate(value);
  if (!date) return '—';
  return new Intl.DateTimeFormat(locale === 'fr' ? 'fr-FR' : 'en-US', {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date);
}

export function formatDate(
  value: string | number | Date | null | undefined,
  locale: 'fr' | 'en' = 'fr',
): string {
  const date = parseDate(value);
  if (!date) return '—';
  return new Intl.DateTimeFormat(locale === 'fr' ? 'fr-FR' : 'en-US', { dateStyle: 'medium' }).format(date);
}

/** "il y a 3 minutes" / "3 minutes ago", refreshed by the caller. */
export function formatRelative(
  value: string | number | Date | null | undefined,
  locale: 'fr' | 'en' = 'fr',
): string {
  const date = parseDate(value);
  if (!date) return '—';
  const formatter = new Intl.RelativeTimeFormat(locale === 'fr' ? 'fr-FR' : 'en-US', { numeric: 'auto' });
  const seconds = Math.round((date.getTime() - Date.now()) / 1000);
  const thresholds: [Intl.RelativeTimeFormatUnit, number][] = [
    ['second', 60],
    ['minute', 60],
    ['hour', 24],
    ['day', 30],
    ['month', 12],
  ];
  let amount = seconds;
  for (const [unit, step] of thresholds) {
    if (Math.abs(amount) < step) return formatter.format(amount, unit);
    amount = Math.round(amount / step);
  }
  return formatter.format(amount, 'year');
}

/** Elapsed wall-clock duration, used for "how long has this workspace run". */
export function formatDuration(
  from: string | number | Date | null | undefined,
  to: string | number | Date | null | undefined = new Date(),
  locale: 'fr' | 'en' = 'fr',
): string {
  const start = parseDate(from);
  if (!start) return '—';
  const end = parseDate(to) ?? new Date();
  let seconds = Math.max(0, Math.floor((end.getTime() - start.getTime()) / 1000));
  const days = Math.floor(seconds / 86400);
  seconds -= days * 86400;
  const hours = Math.floor(seconds / 3600);
  seconds -= hours * 3600;
  const minutes = Math.floor(seconds / 60);
  const dayUnit = locale === 'fr' ? 'j' : 'd';
  if (days > 0) return `${days}${dayUnit} ${hours}h`;
  if (hours > 0) return `${hours}h ${String(minutes).padStart(2, '0')}m`;
  if (minutes > 0) return `${minutes} min`;
  return locale === 'fr' ? "moins d'une minute" : 'less than a minute';
}

export function formatNumber(value: number | null | undefined, locale: 'fr' | 'en' = 'fr'): string {
  if (value === null || value === undefined || Number.isNaN(value)) return '—';
  return new Intl.NumberFormat(locale === 'fr' ? 'fr-FR' : 'en-US').format(value);
}

/** Slugify a display name into a DNS-safe identifier for URLs and resources. */
export function slugify(value: string): string {
  return value
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 63);
}
