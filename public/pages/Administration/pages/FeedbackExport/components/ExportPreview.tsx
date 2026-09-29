import React from "react"
import { Icon, ShowTag } from "@fider/components"
import { FeedbackExportRow, Tag } from "@fider/models"
import { classSet } from "@fider/services"
import { heroiconsChatAlt2 as IconChat, heroiconsThumbsup as IconVotes } from "@fider/icons.generated"
import { PICK_LABELS } from "../recipes"
import { SheetSection, postURL } from "../sheet"

interface ExportPreviewProps {
  sections: SheetSection[]
  tags: Map<number, Tag>
  baseURL: string
  stale: boolean
}

export const ExportPreview: React.FC<ExportPreviewProps> = ({ sections, tags, baseURL, stale }) => (
  <div className={classSet({ "flex flex-col gap-5 transition-opacity duration-150": true, "opacity-60": stale })} aria-busy={stale}>
    {sections.map((section, i) => (
      <section key={i}>
        <div className="mb-1.5 flex items-baseline justify-between gap-3 px-1">
          <h3 className="min-w-0 break-words text-category text-xs tracking-wide">{section.name || `Section ${i + 1}`}</h3>
          <span className="shrink-0 text-xs tabular-nums text-subtle">{section.rows.length} posts</span>
        </div>
        {section.rows.length === 0 ? (
          <p className="rounded-card border border-dashed border-border px-4 py-6 text-center text-sm text-muted">No posts match this section.</p>
        ) : (
          <ol className="divide-y divide-surface-alt overflow-hidden rounded-card border border-border bg-elevated">
            {section.rows.map((row) => (
              <PreviewRow key={row.number} row={row} tags={tags} baseURL={baseURL} />
            ))}
          </ol>
        )}
      </section>
    ))}
  </div>
)

const PreviewRow = ({ row, tags, baseURL }: { row: FeedbackExportRow; tags: Map<number, Tag>; baseURL: string }) => (
  <li className="flex items-center gap-3 px-3 py-2 text-sm transition-colors duration-100 hover:bg-surface-alt">
    <span className="w-12 shrink-0 text-right text-xs tabular-nums text-subtle">#{row.number}</span>
    <div className="min-w-0 flex-1">
      <a href={postURL(baseURL, row)} target="_blank" rel="noopener" className="block truncate text-foreground hover:text-primary" title={row.title}>
        {row.title}
      </a>
      {row.tagIds.length > 0 && (
        <div className="mt-1 flex flex-wrap gap-1">
          {row.tagIds.map((id) => {
            const tag = tags.get(id)
            return tag ? <ShowTag key={id} tag={tag} size="mini" /> : null
          })}
        </div>
      )}
    </div>
    <span className="flex w-16 shrink-0 items-center justify-end gap-1 tabular-nums text-foreground" data-tooltip="Votes">
      <Icon sprite={IconVotes} className="h-3.5 w-3.5 text-subtle" />
      {row.votes}
    </span>
    <span className="flex w-14 shrink-0 items-center justify-end gap-1 tabular-nums text-muted" data-tooltip="Comments">
      <Icon sprite={IconChat} className="h-3.5 w-3.5 text-subtle" />
      {row.comments}
    </span>
    <span className="w-24 shrink-0 text-right text-[11px] font-medium uppercase tracking-wide text-subtle max-sm:hidden">{PICK_LABELS[row.pick]}</span>
  </li>
)
