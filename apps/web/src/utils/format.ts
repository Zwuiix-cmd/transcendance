const dateFormatter = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' })
const dateTimeFormatter = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' })
const shortDateFormatter = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' })

export function formatDate(value: string | null | undefined, fallback = '—') {
  return value ? dateFormatter.format(new Date(value)) : fallback
}

export function formatDateTime(value: string | null | undefined, fallback = '—') {
  return value ? dateTimeFormatter.format(new Date(value)) : fallback
}

export function formatShortDate(value: string, fallback = '—') {
  return value ? shortDateFormatter.format(new Date(value)) : fallback
}
