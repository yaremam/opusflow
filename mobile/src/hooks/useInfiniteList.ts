import { useCallback, useEffect, useRef, useState } from 'react';
import { ListParams, Page } from '../services/api';

// useInfiniteList backs every list in the Library hub (Artists/Albums/
// Songs, backlog/026 AC-2) — one shared infinite-scroll + filter
// implementation instead of tripling the same pagination logic per list.
// Reloads from page 1 whenever filters change (a new search/sort/genre/
// year); loadMore() appends the next page for the same filters.
export function useInfiniteList<T>(fetchPage: (params: ListParams) => Promise<Page<T>>, filters: ListParams) {
  const [items, setItems] = useState<T[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [hasMore, setHasMore] = useState(false);
  const nextPage = useRef(1);
  // Guards a loadMore() that resolves after a newer filter change already
  // started a fresh load — its page would otherwise land on the wrong list.
  const requestId = useRef(0);

  const filterKey = JSON.stringify(filters);

  useEffect(() => {
    const thisRequest = ++requestId.current;
    setLoading(true);
    setError(null);
    nextPage.current = 1;

    fetchPage({ ...filters, page: 1 })
      .then((page) => {
        if (thisRequest !== requestId.current) return;
        setItems(page.items);
        setHasMore(page.items.length > 0 && page.page * page.pageSize < page.totalCount);
        nextPage.current = 2;
      })
      .catch((e) => {
        if (thisRequest !== requestId.current) return;
        setItems([]);
        setHasMore(false);
        setError(e instanceof Error ? e.message : 'Could not load your library.');
      })
      .finally(() => {
        if (thisRequest === requestId.current) setLoading(false);
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filterKey]);

  const loadMore = useCallback(() => {
    if (loading || loadingMore || !hasMore) return;
    const thisRequest = requestId.current;
    const page = nextPage.current;
    setLoadingMore(true);

    fetchPage({ ...filters, page })
      .then((result) => {
        if (thisRequest !== requestId.current) return;
        setItems((prev) => [...prev, ...result.items]);
        setHasMore(result.items.length > 0 && result.page * result.pageSize < result.totalCount);
        nextPage.current = page + 1;
      })
      .catch((e) => {
        if (thisRequest !== requestId.current) return;
        setError(e instanceof Error ? e.message : 'Could not load more.');
      })
      .finally(() => {
        if (thisRequest === requestId.current) setLoadingMore(false);
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loading, loadingMore, hasMore, filterKey]);

  return { items, loading, loadingMore, error, hasMore, loadMore };
}
