const ISO_DATE = /^(\d{4})-(\d{2})-(\d{2})$/;

/**
 * A real calendar date, not merely a date-shaped string.
 *
 * `2026-99-99` and `2026-02-31` both satisfy the shape, and sending either
 * produces a permanent validation error the screen cannot recover from — the
 * repair exists precisely so a malformed link never reaches the API. The check
 * is arithmetic rather than `new Date(...)`, whose parsing rolls February 31st
 * forward to March 3rd instead of rejecting it.
 */
export function isCalendarDate(value: string): boolean {
  const match = ISO_DATE.exec(value);
  if (!match) return false;
  const [, year, month, day] = match;
  const monthNumber = Number(month);
  const dayNumber = Number(day);
  if (monthNumber < 1 || monthNumber > 12 || dayNumber < 1) return false;
  return dayNumber <= daysInMonth(Number(year), monthNumber);
}

function daysInMonth(year: number, month: number): number {
  if (month === 2) {
    const leap = (year % 4 === 0 && year % 100 !== 0) || year % 400 === 0;
    return leap ? 29 : 28;
  }
  return month === 4 || month === 6 || month === 9 || month === 11 ? 30 : 31;
}
