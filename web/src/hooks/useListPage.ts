import { useEffect, useState } from 'react'
import { errorMessage, type ListParams, type Page } from '../api/library'

export interface ListPageFilters {
  sort: 'recent' | 'name'
  genre: string
  year: string
  q: string
  page: number
}

export const EMPTY_FILTERS: ListPageFilters = { sort: 'recent', genre: '', year: '', q: '', page: 1 }

function toParams(filters: ListPageFilters, pageSize: number): ListParams {
  return {
    page: filters.page,
    pageSize,
    sort: filters.sort,
    genre: filters.genre || undefined,
    year: filters.year ? Number(filters.year) : undefined,
    q: filters.q || undefined,
  }
}

// useListPage centralizes the fetch-on-filter-change + pagination-math
// pattern shared by the Artists/Albums/Songs index pages — each page still
// owns its own filter controls and row rendering, just not the data
// fetching or page-count arithmetic.
export function useListPage<T>(fetcher: (params: ListParams) => Promise<Page<T>>, pageSize = 24) {
  const [filters, setFilters] = useState<ListPageFilters>(EMPTY_FILTERS)
  const [page, setPage] = useState<Page<T> | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    fetcher(toParams(filters, pageSize))
      .then((result) => {
        if (!cancelled) setPage(result)
      })
      .catch((err: unknown) => {
        if (!cancelled) setLoadError(errorMessage(err))
      })
    return () => {
      cancelled = true
    }
    // fetcher is stable per page component; filters is the real dependency.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filters])

  function setFilter<K extends keyof ListPageFilters>(key: K, value: ListPageFilters[K]) {
    setFilters((prev) => ({ ...prev, [key]: value, page: key === 'page' ? (value as number) : 1 }))
  }

  const totalPages = page ? Math.max(1, Math.ceil(page.totalCount / page.pageSize)) : 1

  return { filters, setFilter, page, loadError, totalPages }
}
