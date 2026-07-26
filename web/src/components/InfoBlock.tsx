// InfoBlock renders the facts-chip row and optional prose bio/description
// TDR 003 adds to the Artist/Album detail pages. Facts and text track
// independent enrichment statuses on the backend, so either can be present
// without the other — this renders whichever combination it's given, and
// nothing at all when both are empty (AC-13).
interface Fact {
  label: string
  value: string
}

interface InfoBlockProps {
  facts: Fact[]
  text: string
  sourceUrl: string
}

export default function InfoBlock({ facts, text, sourceUrl }: InfoBlockProps) {
  if (facts.length === 0 && !text) return null

  return (
    <div className="info-block">
      {facts.length > 0 && (
        <div className="fact-row">
          {facts.map((f, i) => (
            <span className="fact-chip" key={`${f.label}-${i}`}>
              {f.label} · <b>{f.value}</b>
            </span>
          ))}
        </div>
      )}
      {text && (
        <p className="bio">
          {text}
          {sourceUrl && (
            <span className="bio-source">
              via{' '}
              <a href={sourceUrl} target="_blank" rel="noreferrer">
                Wikipedia
              </a>
              , CC BY-SA
            </span>
          )}
        </p>
      )}
    </div>
  )
}
