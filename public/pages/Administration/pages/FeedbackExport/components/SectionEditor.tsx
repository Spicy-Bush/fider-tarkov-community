import React from "react"
import { Icon } from "@fider/components"
import { FeedbackExportLimits, FeedbackExportMode, FeedbackExportSection, FeedbackExportStatus, Tag } from "@fider/models"
import { classSet } from "@fider/services"
import {
  heroiconsCheck as IconCheck,
  heroiconsChevronUp as IconChevronUp,
  heroiconsChevronDown as IconChevronDown,
  heroiconsPlus as IconPlus,
  heroiconsX as IconX,
} from "@fider/icons.generated"
import { AGE_OPTIONS, PICK_LABELS, STATUS_LABELS } from "../recipes"

interface SectionEditorProps {
  section: FeedbackExportSection
  index: number
  count: number
  tags: Tag[]
  statuses: FeedbackExportStatus[]
  limits: FeedbackExportLimits
  onChange: (section: FeedbackExportSection) => void
  onMove: (offset: -1 | 1) => void
  onRemove: () => void
}

const selectStyle: React.CSSProperties = {
  backgroundImage:
    "url(\"data:image/svg+xml,%3csvg xmlns='http://www.w3.org/2000/svg' fill='none' viewBox='0 0 20 20'%3e%3cpath stroke='%236b7280' stroke-linecap='round' stroke-linejoin='round' stroke-width='1.5' d='M6 8l4 4 4-4'/%3e%3c/svg%3e\")",
  backgroundPosition: "right 0.35rem center",
  backgroundSize: "1.25em 1.25em",
}
const selectClass =
  "h-9 rounded-input border border-border bg-elevated bg-no-repeat pl-2.5 pr-8 text-sm text-foreground appearance-none outline-none focus:border-primary"
const numberClass = "h-9 w-20 rounded-input border border-border bg-elevated px-2.5 text-sm tabular-nums text-foreground outline-none focus:border-primary"

const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(max, Number.isFinite(value) ? Math.trunc(value) : min))

const Label = ({ children }: { children: React.ReactNode }) => <div className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-muted">{children}</div>

const TagChip = ({ tag, state, onClick }: { tag: Tag; state: "include" | "exclude" | null; onClick: () => void }) => (
  <button
    type="button"
    onClick={onClick}
    aria-pressed={state !== null}
    data-tooltip={state === "include" ? "Included" : state === "exclude" ? "Excluded" : undefined}
    className={classSet({
      "inline-block bg-border tag-clipped p-px transition-colors duration-100": true,
      "hover:bg-border-strong": state === null,
      "bg-success-dark": state === "include",
      "bg-danger-dark": state === "exclude",
    })}
  >
    <span
      className={classSet({
        "wipe-fill tag-clipped-inner inline-flex items-center bg-surface-alt px-2.5 py-1.5 text-[13px] font-medium text-foreground": true,
        "is-selected [--wipe-color:var(--color-success-medium)]": state === "include",
        "is-selected [--wipe-color:var(--color-danger-medium)]": state === "exclude",
      })}
    >
      <span className="mr-1.5 inline-block h-3 w-3 shrink-0 rounded-full" style={{ backgroundColor: `#${tag.color}` }} />
      <span className={classSet({ "line-through decoration-danger-dark": state === "exclude" })}>{tag.name}</span>
      {state && <Icon sprite={state === "include" ? IconCheck : IconX} className="wipe-check ml-1.5 h-3.5 w-3.5" />}
    </span>
  </button>
)

export const SectionEditor: React.FC<SectionEditorProps> = ({ section, index, count, tags, statuses, limits, onChange, onMove, onRemove }) => {
  const update = (change: Partial<FeedbackExportSection>) => onChange({ ...section, ...change })

  const cycleTag = (id: number) => {
    if (section.includeTags.includes(id)) {
      update({ includeTags: section.includeTags.filter((tag) => tag !== id), excludeTags: [...section.excludeTags, id] })
    } else if (section.excludeTags.includes(id)) {
      update({ excludeTags: section.excludeTags.filter((tag) => tag !== id) })
    } else {
      update({ includeTags: [...section.includeTags, id] })
    }
  }

  const toggleStatus = (status: FeedbackExportStatus) => {
    const next = section.statuses.includes(status) ? section.statuses.filter((s) => s !== status) : [...section.statuses, status]
    if (next.length > 0) {
      update({ statuses: next })
    }
  }

  const updatePick = (i: number, change: Partial<FeedbackExportSection["picks"][number]>) =>
    update({ picks: section.picks.map((pick, j) => (i === j ? { ...pick, ...change } : pick)) })

  return (
    <div className="rounded-card border border-border bg-elevated popover-enter">
      <div className="flex items-center gap-2 border-b border-surface-alt px-4 py-3">
        <input
          value={section.name}
          aria-label="Section name"
          placeholder={`Section ${index + 1}`}
          onChange={(event) => update({ name: event.currentTarget.value })}
          className="min-w-0 flex-1 bg-transparent text-base font-semibold text-foreground outline-none placeholder:text-subtle"
        />
        <div className="flex items-center text-subtle">
          <button
            type="button"
            aria-label="Move up"
            disabled={index === 0}
            onClick={() => onMove(-1)}
            className="flex rounded-badge p-1 transition-colors hover:bg-tertiary hover:text-foreground disabled:opacity-30"
          >
            <Icon sprite={IconChevronUp} className="h-4 w-4" />
          </button>
          <button
            type="button"
            aria-label="Move down"
            disabled={index === count - 1}
            onClick={() => onMove(1)}
            className="flex rounded-badge p-1 transition-colors hover:bg-tertiary hover:text-foreground disabled:opacity-30"
          >
            <Icon sprite={IconChevronDown} className="h-4 w-4" />
          </button>
          <button
            type="button"
            aria-label="Remove section"
            disabled={count === 1}
            onClick={onRemove}
            className="flex rounded-badge p-1 transition-colors hover:bg-tertiary hover:text-danger disabled:opacity-30"
          >
            <Icon sprite={IconX} className="h-4 w-4" />
          </button>
        </div>
      </div>

      <div className="flex flex-col gap-5 p-4">
        <div>
          <Label>Tags</Label>
          <div className="flex flex-wrap gap-1.5">
            {tags.map((tag) => (
              <TagChip
                key={tag.id}
                tag={tag}
                state={section.includeTags.includes(tag.id) ? "include" : section.excludeTags.includes(tag.id) ? "exclude" : null}
                onClick={() => cycleTag(tag.id)}
              />
            ))}
          </div>
        </div>

        <div className="flex flex-wrap gap-x-6 gap-y-4">
          <div>
            <Label>Status</Label>
            <div className="flex flex-wrap gap-1.5">
              {statuses.map((status) => {
                const selected = section.statuses.includes(status)
                return (
                  <button
                    key={status}
                    type="button"
                    aria-pressed={selected}
                    onClick={() => toggleStatus(status)}
                    className={classSet({
                      "wipe-fill rounded-badge border px-2.5 py-1.5 text-xs font-medium transition-colors duration-200 ease-out hover:bg-tertiary": true,
                      "is-selected border-primary text-foreground": selected,
                      "border-border text-muted": !selected,
                    })}
                  >
                    {STATUS_LABELS[status]}
                  </button>
                )
              })}
            </div>
          </div>
          <div>
            <Label>Posted</Label>
            <select
              value={section.maxAgeDays}
              onChange={(event) => update({ maxAgeDays: Number(event.currentTarget.value) })}
              className={selectClass}
              style={selectStyle}
            >
              {AGE_OPTIONS.map((option) => (
                <option key={option.days} value={option.days}>
                  {option.label}
                </option>
              ))}
            </select>
          </div>
          <div>
            <Label>Min votes</Label>
            <input
              type="number"
              value={section.minVotes ?? ""}
              placeholder="Any"
              onChange={(event) => update({ minVotes: event.currentTarget.value === "" ? null : Math.trunc(Number(event.currentTarget.value)) })}
              className={numberClass}
            />
          </div>
        </div>

        <div>
          <Label>Picks</Label>
          <div className="flex flex-col gap-2">
            {section.picks.map((pick, i) => (
              <div key={i} className="flex flex-wrap items-center gap-2 text-sm text-muted">
                <select
                  value={pick.mode}
                  aria-label="Pick"
                  onChange={(event) => updatePick(i, { mode: event.currentTarget.value as FeedbackExportMode })}
                  className={selectClass}
                  style={selectStyle}
                >
                  {Object.entries(PICK_LABELS).map(([mode, label]) => (
                    <option key={mode} value={mode}>
                      {label}
                    </option>
                  ))}
                </select>
                <input
                  type="number"
                  min={1}
                  max={limits.pickSize}
                  value={pick.count}
                  aria-label="Posts"
                  onChange={(event) => updatePick(i, { count: clamp(Number(event.currentTarget.value), 1, limits.pickSize) })}
                  className={numberClass}
                />
                <span>posts with</span>
                <input
                  type="number"
                  min={0}
                  value={pick.minComments}
                  aria-label="Minimum comments"
                  onChange={(event) => updatePick(i, { minComments: clamp(Number(event.currentTarget.value), 0, 100000) })}
                  className={numberClass}
                />
                <span>+ comments</span>
                <button
                  type="button"
                  aria-label="Remove pick"
                  disabled={section.picks.length === 1}
                  onClick={() => update({ picks: section.picks.filter((_, j) => j !== i) })}
                  className="ml-auto flex rounded-badge p-1 text-subtle transition-colors hover:bg-tertiary hover:text-danger disabled:opacity-30"
                >
                  <Icon sprite={IconX} className="h-4 w-4" />
                </button>
              </div>
            ))}
            {section.picks.length < limits.picks && (
              <button
                type="button"
                onClick={() => update({ picks: [...section.picks, { mode: "random", count: 10, minComments: 5 }] })}
                className="flex w-max items-center gap-1 rounded-badge px-1.5 py-1 text-xs font-medium text-primary transition-colors hover:bg-tertiary"
              >
                <Icon sprite={IconPlus} className="h-3.5 w-3.5" />
                Add pick
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
