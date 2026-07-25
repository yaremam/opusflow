interface PagerProps {
  page: number
  totalPages: number
  onChange: (page: number) => void
}

// pageWindow picks which page numbers to render around current, with a
// leading/trailing "…" gap marker (represented as 0) when pages are
// skipped, so a 40-page list doesn't render 40 buttons.
function pageWindow(current: number, total: number): number[] {
  const pages = new Set<number>([1, total, current, current - 1, current + 1])
  const sorted = [...pages].filter((p) => p >= 1 && p <= total).sort((a, b) => a - b)

  const result: number[] = []
  for (let i = 0; i < sorted.length; i++) {
    if (i > 0 && sorted[i] - sorted[i - 1] > 1) result.push(0)
    result.push(sorted[i])
  }
  return result
}

export default function Pager({ page, totalPages, onChange }: PagerProps) {
  if (totalPages <= 1) return null

  return (
    <div className="pager">
      <button type="button" disabled={page <= 1} onClick={() => onChange(page - 1)} aria-label="Previous page">
        ‹
      </button>
      {pageWindow(page, totalPages).map((p, i) =>
        p === 0 ? (
          <span key={`gap-${i}`} className="gap">
            …
          </span>
        ) : (
          <button key={p} type="button" className={p === page ? 'current' : ''} onClick={() => onChange(p)}>
            {p}
          </button>
        ),
      )}
      <button type="button" disabled={page >= totalPages} onClick={() => onChange(page + 1)} aria-label="Next page">
        ›
      </button>
    </div>
  )
}
