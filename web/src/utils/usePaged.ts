import { useState } from 'react'

// Client-side paging for tables whose rows are already loaded. Ten per page; the Pagination
// component hides itself when everything fits on one page.
export const PAGE_SIZE = 10

export function usePaged<T>(items: T[], pageSize = PAGE_SIZE) {
  const [page, setPage] = useState(1)
  const totalPages = Math.max(1, Math.ceil(items.length / pageSize))
  // Clamped rather than reset, so a list that shrinks between refreshes never strands the
  // reader on an empty page.
  const current = Math.min(page, totalPages)
  return {
    page: current,
    setPage,
    totalPages,
    total: items.length,
    pageSize,
    items: items.slice((current - 1) * pageSize, current * pageSize),
  }
}
