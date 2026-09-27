import React, { useLayoutEffect, useMemo, useRef, useState } from "react"

export type DiscussionRow =
  | { kind: "comment"; id: number; depth: number; end: number; collapsed: boolean }
  | { kind: "load"; parentId: number; depth: number; levels: number }
  | { kind: "continue"; parentId: number; depth: number }

interface DiscussionViewportProps {
  rows: DiscussionRow[]
  target?: number
  readingTarget?: number
  measurements?: Map<string, number>
  initialViewport?: { top: number; bottom: number }
  onVisible?: (ids: number[]) => void
  onCollapse: (id: number, collapsed: boolean) => void
  children: (row: DiscussionRow) => React.ReactNode
}

function rowKey(row: DiscussionRow): string {
  return row.kind === "comment" ? `comment:${row.id}` : `${row.kind}:${row.parentId}`
}

function rowAtOffset(offsets: number[], position: number): number {
  let first = 0
  let last = offsets.length - 1

  while (first < last) {
    const middle = Math.floor((first + last + 1) / 2)

    if (offsets[middle] <= position) {
      first = middle
    } else {
      last = middle - 1
    }
  }

  return first
}

export function DiscussionViewport({ rows, target, readingTarget, measurements, initialViewport, onVisible, onCollapse, children }: DiscussionViewportProps) {
  const container = useRef<HTMLDivElement>(null)
  const sizes = useRef(measurements ?? new Map<string, number>())
  const [measurement, setMeasurement] = useState(0)
  const [viewport, setViewport] = useState(initialViewport ?? { top: 0, bottom: 1000 })
  const lastTarget = useRef<number>()
  const [focusedRow, setFocusedRow] = useState<string>()

  const offsets = useMemo(() => {
    const result = [0]

    for (const row of rows) {
      const estimate = row.kind === "comment" && !row.collapsed ? 240 : 48
      result.push(result[result.length - 1] + (sizes.current.get(rowKey(row)) ?? estimate))
    }

    return result
  }, [rows, measurement])

  const first = Math.min(rows.length, rowAtOffset(offsets, Math.max(0, viewport.top - 800)))
  const last = Math.min(rows.length, rowAtOffset(offsets, viewport.bottom + 800) + 1)
  const focusIndex = focusedRow ? rows.findIndex((row) => rowKey(row) === focusedRow) : -1
  const targetIndex = target && lastTarget.current !== target
    ? rows.findIndex((row) => row.kind === "comment" && row.id === target)
    : -1
  const readingIndex = readingTarget
    ? rows.findIndex((row) => row.kind === "comment" && row.id === readingTarget)
    : -1
  const visible = new Set<number>()

  for (let index = first; index < last; index++) {
    visible.add(index)
  }

  if (focusIndex >= 0) {
    visible.add(focusIndex)
  }

  if (targetIndex >= 0) {
    visible.add(targetIndex)
  }

  if (readingIndex >= 0) {
    visible.add(readingIndex)
  }

  const indexes = [...visible].sort((left, right) => left - right)

  useLayoutEffect(() => {
    onVisible?.(indexes.flatMap((index) => {
      const row = rows[index]
      return row.kind === "comment" ? [row.id] : []
    }))
  }, [rows, first, last, focusedRow, targetIndex, readingIndex, onVisible])

  useLayoutEffect(() => {
    const element = container.current!
    let scrollParent = element.parentElement

    while (scrollParent && !/(auto|scroll)/.test(getComputedStyle(scrollParent).overflowY)) {
      scrollParent = scrollParent.parentElement
    }

    let frame = 0
    const update = () => {
      frame = 0
      const bounds = element.getBoundingClientRect()
      const clip = scrollParent?.getBoundingClientRect()
      const top = Math.max(0, (clip ? Math.max(0, clip.top) : 0) - bounds.top)
      const bottom = Math.max(top, Math.min(window.innerHeight, clip?.bottom ?? window.innerHeight) - bounds.top)

      setViewport((previous) => previous.top === top && previous.bottom === bottom ? previous : { top, bottom })
    }

    const schedule = () => {
      if (!frame) {
        frame = requestAnimationFrame(update)
      }
    }

    if (!initialViewport) {
      update()
    }
    window.addEventListener("scroll", schedule, { capture: true, passive: true })
    window.addEventListener("resize", schedule)

    return () => {
      cancelAnimationFrame(frame)
      window.removeEventListener("scroll", schedule, true)
      window.removeEventListener("resize", schedule)
    }
  }, [])

  useLayoutEffect(() => {
    const element = container.current!
    const observer = new ResizeObserver((entries) => {
      let changed = false

      for (const entry of entries) {
        const key = (entry.target as HTMLElement).dataset.discussionRow!
        const height = entry.borderBoxSize[0]?.blockSize ?? entry.target.getBoundingClientRect().height

        if (height > 0 && sizes.current.get(key) !== height) {
          sizes.current.set(key, height)
          changed = true
        }
      }

      if (changed) {
        setMeasurement((previous) => previous + 1)
      }
    })

    for (const row of element.querySelectorAll<HTMLElement>("[data-discussion-row]")) {
      observer.observe(row)
    }

    return () => observer.disconnect()
  }, [rows, first, last, focusedRow, targetIndex])

  useLayoutEffect(() => {
    if (targetIndex >= 0) {
      container.current?.querySelector(`#comment-${target}`)?.scrollIntoView({ block: "center" })
      lastTarget.current = target
    } else if (!target) {
      lastTarget.current = undefined
    }
  }, [target, targetIndex])

  const gridRows: React.ReactNode[] = []
  const gridPositions = new Map<number, number>()
  let previous = 0
  let gridLine = 1

  for (const index of indexes) {
    if (index > previous) {
      gridRows.push(
        <div
          key={`space:${previous}`}
          aria-hidden="true"
          style={{ gridRow: gridLine, gridColumn: "1 / -1", height: offsets[index] - offsets[previous] }}
        />
      )
      gridLine++
    }

    const row = rows[index]
    gridPositions.set(index, gridLine)
    gridRows.push(
      <div
        key={rowKey(row)}
        data-discussion-row={rowKey(row)}
        className="discussion-row"
        style={{ gridRow: gridLine, gridColumn: `${Math.min(row.depth, 4) + 1} / -1` }}
        onFocusCapture={() => setFocusedRow(rowKey(row))}
        onPointerDownCapture={() => setFocusedRow(rowKey(row))}
      >
        {children(row)}
      </div>
    )

    gridLine++
    previous = index + 1
  }

  if (previous < rows.length) {
    gridRows.push(
      <div
        key={`space:${previous}`}
        aria-hidden="true"
        style={{ gridRow: gridLine, gridColumn: "1 / -1", height: offsets[rows.length] - offsets[previous] }}
      />
    )
  }

  const rails: React.ReactNode[] = []
  let visiblePosition = 0

  for (let index = 0; index < rows.length; index++) {
    while (visiblePosition < indexes.length && indexes[visiblePosition] < index) {
      visiblePosition++
    }

    if (visiblePosition === indexes.length) {
      break
    }

    const row = rows[index]

    if (row.kind !== "comment") {
      continue
    }

    const end = row.depth < 4 ? row.end : index + 1

    if (indexes[visiblePosition] >= end) {
      continue
    }

    let endPosition = visiblePosition

    while (endPosition + 1 < indexes.length && indexes[endPosition + 1] < end) {
      endPosition++
    }

    const startsHere = indexes[visiblePosition] === index
    rails.push(
      <button
        key={`rail:${row.id}`}
        type="button"
        className="discussion-thread-toggle"
        style={{
          gridRow: `${gridPositions.get(indexes[visiblePosition])} / ${gridPositions.get(indexes[endPosition])! + 1}`,
          gridColumn: Math.min(row.depth, 4) + 1,
        }}
        aria-label={`${row.collapsed ? "Expand" : "Collapse"} thread for comment ${row.id}`}
        aria-expanded={!row.collapsed}
        data-has-replies={row.end > index + 1}
        data-continuation={!startsHere || undefined}
        onClick={() => onCollapse(row.id, !row.collapsed)}
      >
        {startsHere && <span aria-hidden="true">{row.collapsed ? "+" : "−"}</span>}
      </button>
    )
  }

  return (
    <div
      ref={container}
      className="discussion-threads"
      data-comment-count={rows.filter((row) => row.kind === "comment").length}
      onBlurCapture={(event) => {
        if (event.relatedTarget && !event.currentTarget.contains(event.relatedTarget)) {
          setFocusedRow(undefined)
        }
      }}
    >
      {gridRows}
      {rails}
    </div>
  )
}
