import React from "react"
import { Button } from "@fider/components"

export const FileCheckbox = (props: {
  checked: boolean
  disabled?: boolean
  label?: string
  children?: React.ReactNode
  onChange: (checked: boolean) => void
}) => {
  return (
    <label className={`inline-flex min-h-10 items-center gap-2 rounded-button px-2 text-sm ${props.disabled ? "opacity-50" : "cursor-pointer hover:bg-primary/10"}`}>
      <span className="relative flex h-5 w-5 shrink-0 items-center justify-center">
        <input
          type="checkbox"
          aria-label={props.label}
          checked={props.checked}
          disabled={props.disabled}
          onChange={event => props.onChange(event.target.checked)}
          className="m-0 h-5 w-5 cursor-pointer appearance-none rounded-badge border border-border-strong bg-elevated checked:border-primary checked:bg-primary disabled:cursor-not-allowed focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
        />
        {props.checked && (
          <svg
            aria-hidden="true"
            viewBox="0 0 20 20"
            className="pointer-events-none absolute h-5 w-5 text-white"
            fill="none"
            stroke="currentColor"
            strokeWidth="2.5"
          >
            <path d="m4 10 4 4 8-8" />
          </svg>
        )}
      </span>
      {props.children}
    </label>
  )
}

export const FileSelect = (props: {
  label: string
  value: string | number
  children: React.ReactNode
  onChange: (value: string) => void
  className?: string
}) => (
  <label className={`block min-w-0 text-sm font-medium ${props.className || ""}`}>
    <span className="mb-1 block">{props.label}</span>
    <span className="relative block">
      <select
        aria-label={props.label}
        value={props.value}
        onChange={event => props.onChange(event.target.value)}
        className="w-full cursor-pointer appearance-none rounded-input border border-border bg-elevated py-2 pl-3 pr-9 text-sm focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary"
      >
        {props.children}
      </select>
      <svg
        aria-hidden="true"
        viewBox="0 0 20 20"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        className="pointer-events-none absolute right-2 top-1/2 h-5 w-5 -translate-y-1/2 text-muted"
      >
        <path d="m6 8 4 4 4-4" />
      </svg>
    </span>
  </label>
)

export const FilePagination = (props: {
  page: number
  totalPages: number
  onChange: (page: number) => void
}) => {
  if (props.totalPages <= 1) {
    return null
  }

  const pages = [1]
  const first = Math.max(2, props.page - 1)
  const last = Math.min(props.totalPages - 1, props.page + 1)

  for (let page = first; page <= last; page++) {
    pages.push(page)
  }

  pages.push(props.totalPages)

  return (
    <nav aria-label="Image pages" className="flex flex-wrap items-center gap-2">
      <Button size="small" disabled={props.page === 1} onClick={() => props.onChange(props.page - 1)}>Previous page</Button>
      <div className="flex items-center gap-1">
        {pages.map((page, index) => (
          <React.Fragment key={page}>
            {index > 0 && page - pages[index - 1] > 1 && <span aria-hidden="true" className="w-2" />}
            <button
              type="button"
              aria-label={`Page ${page}`}
              aria-current={page === props.page ? "page" : undefined}
              onClick={() => props.onChange(page)}
              className={`h-9 min-w-9 cursor-pointer rounded-button border px-2 text-sm tabular-nums focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary ${page === props.page ? "border-primary bg-primary text-white" : "border-border bg-elevated hover:bg-surface-alt"}`}
            >
              {page}
            </button>
          </React.Fragment>
        ))}
      </div>
      <Button size="small" disabled={props.page === props.totalPages} onClick={() => props.onChange(props.page + 1)}>Next page</Button>
      {props.totalPages > 7 && (
        <form
          key={props.page}
          className="flex items-center gap-2"
          onSubmit={event => {
            event.preventDefault()
            const page = Number(new FormData(event.currentTarget).get("page"))
            props.onChange(page)
          }}
        >
          <label className="flex items-center gap-2 text-sm">
            Go to page
            <input
              name="page"
              type="number"
              min={1}
              max={props.totalPages}
              required
              defaultValue={props.page}
              className="w-20 rounded-input border border-border bg-elevated px-2 py-1.5"
            />
          </label>
          <Button type="submit" size="small">Go</Button>
        </form>
      )}
    </nav>
  )
}
