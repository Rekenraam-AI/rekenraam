import type { AuthSessionResponse } from '#lib/api/auth.ts';
import { isCalendarDate } from './calendar-date.ts';

/** Authenticated shell contract: never invent a date from the browser clock. */
export function sessionOwnerToday(session: AuthSessionResponse): string {
  if (!session.authenticated || !session.owner_today || !isCalendarDate(session.owner_today)) {
    throw new Error('Authenticated owner date is unavailable');
  }
  return session.owner_today;
}
