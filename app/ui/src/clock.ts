// SPDX-License-Identifier: AGPL-3.0-only

/** Formats a length of media in seconds as minutes and seconds ("1:05"), with the digits of `locale`. */
export function formatClock(locale: string, seconds: number): string {
  const digits = new Intl.NumberFormat(locale, { minimumIntegerDigits: 2, useGrouping: false });
  const plain = new Intl.NumberFormat(locale, { useGrouping: false });
  const whole = Number.isFinite(seconds) ? Math.max(0, Math.floor(seconds)) : 0;
  return `${plain.format(Math.floor(whole / 60))}:${digits.format(whole % 60)}`;
}
