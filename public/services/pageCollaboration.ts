import type * as Y from "yjs"
import { DraftImage } from "./draftImages"

export interface PageWorkingCopy {
  title: string
  slug: string
  content: string
  excerpt: string
  metaDescription: string
  bannerImage: DraftImage | null
  status: string
  visibility: string
  parentPageId: number | null
  allowedRoles: string[]
  allowComments: boolean
  allowCommentImages: boolean
  allowReactions: boolean
  showToc: boolean
  topics: number[]
  tags: number[]
  authors: number[]
  scheduledFor: string
}

const textFields = ["title", "slug", "content", "excerpt", "metaDescription"] as const
const collectionFields = ["topics", "tags", "allowedRoles"] as const
const settingFields = [
  "status", "visibility", "parentPageId", "allowComments", "allowCommentImages", "allowReactions", "showToc", "scheduledFor",
] as const

export function encodePageState(bytes: Uint8Array): string {
  let binary = ""
  for (let offset = 0; offset < bytes.length; offset += 8192) {
    binary += String.fromCharCode(...bytes.subarray(offset, offset + 8192))
  }
  return btoa(binary)
}

export function decodePageState(encoded: string): Uint8Array {
  return Uint8Array.from(atob(encoded), character => character.charCodeAt(0))
}

export function pageScheduleInput(value: string): string {
  if (!value) {
    return ""
  }

  const date = new Date(value)
  return new Date(date.getTime() - date.getTimezoneOffset() * 60_000).toISOString().slice(0, 16)
}

function orderedAuthors(document: Y.Doc): [string, number][] {
  return [...document.getMap<number>("authors")]
    .sort(([leftID, leftPosition], [rightID, rightPosition]) => leftPosition - rightPosition || Number(leftID) - Number(rightID))
}

export function readPageDocument(document: Y.Doc): PageWorkingCopy {
  const settings = document.getMap("settings")
  const numbers = (field: string) => [...document.getMap<boolean>(field).keys()].map(Number).sort((left, right) => left - right)
  const banner = settings.get("bannerImageBKey") as string

  return {
    title: document.getText("title").toString(),
    slug: document.getText("slug").toString(),
    content: document.getText("content").toString(),
    excerpt: document.getText("excerpt").toString(),
    metaDescription: document.getText("metaDescription").toString(),
    bannerImage: banner ? { kind: "stored", bkey: banner } : null,
    status: settings.get("status") as string,
    visibility: settings.get("visibility") as string,
    parentPageId: settings.get("parentPageId") as number | null,
    allowComments: settings.get("allowComments") as boolean,
    allowCommentImages: settings.get("allowCommentImages") as boolean,
    allowReactions: settings.get("allowReactions") as boolean,
    showToc: settings.get("showToc") as boolean,
    scheduledFor: pageScheduleInput(settings.get("scheduledFor") as string),
    topics: numbers("topics"),
    tags: numbers("tags"),
    authors: orderedAuthors(document).map(([id]) => Number(id)),
    allowedRoles: [...document.getMap<boolean>("allowedRoles").keys()].sort(),
  }
}

export function changePageDocument(document: Y.Doc, changes: Partial<PageWorkingCopy>, origin: unknown): void {
  document.transact(() => {
    for (const field of textFields) {
      const next = changes[field]
      if (next === undefined) {
        continue
      }

      const text = document.getText(field)
      const previous = text.toString()
      let start = 0
      while (start < previous.length && start < next.length && previous[start] === next[start]) {
        start++
      }

      if (start > 0 && previous.charCodeAt(start - 1) >= 0xD800 && previous.charCodeAt(start - 1) <= 0xDBFF) {
        start--
      }

      let end = 0
      while (end < previous.length - start && end < next.length - start && previous[previous.length - end - 1] === next[next.length - end - 1]) {
        end++
      }

      if (end > 0 && previous.charCodeAt(previous.length - end) >= 0xDC00 && previous.charCodeAt(previous.length - end) <= 0xDFFF) {
        end--
      }

      if (previous.length > start + end) {
        text.delete(start, previous.length - start - end)
      }

      if (next.length > start + end) {
        text.insert(start, next.slice(start, next.length - end))
      }
    }

    const settings = document.getMap("settings")
    for (const field of settingFields) {
      const next = field === "scheduledFor" && changes.scheduledFor
        ? new Date(changes.scheduledFor).toISOString()
        : changes[field]
      if (next !== undefined && next !== settings.get(field)) {
        settings.set(field, next)
      }
    }

    if (changes.bannerImage !== undefined) {
      const banner = changes.bannerImage
      const key = banner?.kind === "stored" ? banner.bkey : ""
      if ((banner === null || banner.kind === "removed" || banner.kind === "stored") && key !== settings.get("bannerImageBKey")) {
        settings.set("bannerImageBKey", key)
      }
    }

    if (changes.authors !== undefined) {
      const requested = changes.authors
      const authors = document.getMap<number>("authors")
      const previous = orderedAuthors(document)
      const selected = new Set(requested.map(String))
      const retained = previous.filter(([id]) => selected.has(id))
      const preservesOrder = retained.every(([id], index) => id === String(requested[index]))
      let nextPosition = previous.reduce((maximum, [, position]) => Math.max(maximum, position + 1), 0)

      for (const id of authors.keys()) {
        if (!selected.has(id)) {
          authors.delete(id)
        }
      }

      requested.forEach((id, index) => {
        const key = String(id)
        if (preservesOrder) {
          // A position write competes with another editor's removal.
          if (!authors.has(key)) {
            authors.set(key, nextPosition++)
          }
        } else if (authors.get(key) !== index) {
          authors.set(key, index)
        }
      })
    }

    for (const field of collectionFields) {
      const next = changes[field]
      if (next === undefined) {
        continue
      }

      const entries = document.getMap<boolean>(field)
      const selected = new Set(next.map(String))
      for (const key of entries.keys()) {
        if (!selected.has(key)) {
          entries.delete(key)
        }
      }

      for (const key of selected) {
        if (!entries.has(key)) {
          entries.set(key, true)
        }
      }
    }
  }, origin)
}
