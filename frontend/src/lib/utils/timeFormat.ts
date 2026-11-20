/** Format seconds using integer ms/centis rounding so 2.3 never formats as 00:02.29. */
export function formatTime(secs: number): string {
  if (!Number.isFinite(secs) || secs < 0) return '00:00.00';
  const totalMs = Math.round(secs * 1000);
  const totalCentis = Math.round(totalMs / 10);
  const hours = Math.floor(totalCentis / 360000);
  const mins = Math.floor((totalCentis % 360000) / 6000);
  const wholeSecs = Math.floor((totalCentis % 6000) / 100);
  const centis = totalCentis % 100;

  const mStr = String(mins).padStart(2, '0');
  const sStr = String(wholeSecs).padStart(2, '0');
  const cStr = String(centis).padStart(2, '0');
  if (hours > 0) {
    return `${String(hours).padStart(2, '0')}:${mStr}:${sStr}.${cStr}`;
  }
  return `${mStr}:${sStr}.${cStr}`;
}

/** Compact clock display without centiseconds for axis ticks. */
export function formatTimeShort(secs: number): string {
  if (!Number.isFinite(secs) || secs < 0) return '0:00';
  const totalSecs = Math.round(secs);
  const h = Math.floor(totalSecs / 3600);
  const m = Math.floor((totalSecs % 3600) / 60);
  const s = totalSecs % 60;
  if (h > 0) {
    return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
  }
  return `${m}:${String(s).padStart(2, '0')}`;
}

/** Parse HH:MM:SS(.ms), MM:SS(.ms), or raw numeric seconds to float seconds. */
export function parseTime(input: string): number {
  if (!input) return 0;
  const trimmed = input.trim();
  if (/^\d+(\.\d+)?$/.test(trimmed)) {
    const val = Number(trimmed);
    return Number.isFinite(val) ? Math.max(0, val) : 0;
  }

  const parts = trimmed.split(':');
  if (parts.length > 3 || parts.length === 0) return 0;

  const nums = parts.map((p) => Number(p));
  if (nums.some((n) => !Number.isFinite(n) || n < 0)) return 0;

  if (nums.length === 3) return nums[0] * 3600 + nums[1] * 60 + nums[2];
  if (nums.length === 2) return nums[0] * 60 + nums[1];
  return nums[0];
}
