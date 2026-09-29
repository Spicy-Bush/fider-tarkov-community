import { FeedbackExportRow } from "@fider/models"
import { escapeUTF8 as escapeHTML } from "entities"

export const SHEET_COLUMNS = ["Post", "Votes", "Nikita", "Version", "GD", "GD Contractor"]

type Cell = { text: string } | { link: string; text: string } | { number: number }

interface SheetRow {
  kind: "title" | "header" | "section" | "post"
  cells: Cell[]
}

export interface SheetSection {
  name: string
  rows: FeedbackExportRow[]
}

export interface SheetOptions {
  baseURL: string
  date: string
  title: string
  titleRow: boolean
}

export const postURL = (baseURL: string, row: FeedbackExportRow) => `${baseURL}/posts/${row.number}/${row.slug}`

export const buildSheet = (sections: SheetSection[], options: SheetOptions): SheetRow[] => {
  const rows: SheetRow[] = []
  if (options.titleRow) {
    rows.push({ kind: "title", cells: [{ text: options.title ? `${options.title} (${options.date})` : options.date }] })
  }
  rows.push({ kind: "header", cells: SHEET_COLUMNS.map((text) => ({ text })) })

  const labelled = sections.length > 1
  for (const section of sections) {
    if (labelled) {
      rows.push({ kind: "section", cells: [{ text: section.name }] })
    }
    for (const row of section.rows) {
      rows.push({
        kind: "post",
        cells: [{ link: postURL(options.baseURL, row), text: row.title }, { number: row.votes }, ...SHEET_COLUMNS.slice(2).map(() => ({ text: "" }))],
      })
    }
  }
  return rows
}

// Titles are user content; a leading formula character would run as a formula in the sheet.
const plainText = (text: string) => {
  const flat = text.replace(/[\t\r\n]+/g, " ")
  return /^\s*[=+\-@]/.test(flat) ? `'${flat}` : flat
}

const formulaString = (text: string) => `"${text.replace(/"/g, '""')}"`

const delimitedCell = (cell: Cell): string => {
  if ("link" in cell) return `=HYPERLINK(${formulaString(cell.link)},${formulaString(cell.text.replace(/[\t\r\n]+/g, " "))})`
  if ("number" in cell) return String(cell.number)
  return plainText(cell.text)
}

export const toTSV = (rows: SheetRow[]) => rows.map((row) => row.cells.map(delimitedCell).join("\t")).join("\n")

export const toCSV = (rows: SheetRow[]) => {
  const quote = (value: string) => (/[",\n]/.test(value) ? `"${value.replace(/"/g, '""')}"` : value)
  return rows.map((row) => row.cells.map((cell) => quote(delimitedCell(cell))).join(",")).join("\r\n")
}

export const toHTML = (rows: SheetRow[]) => {
  const cell = (value: Cell, bold: boolean) => {
    let content: string
    if ("link" in value) content = `<a href="${escapeHTML(value.link)}">${escapeHTML(value.text)}</a>`
    else if ("number" in value) content = String(value.number)
    else content = escapeHTML(plainText(value.text))
    return `<td>${bold && content ? `<b>${content}</b>` : content}</td>`
  }
  const body = rows.map((row) => `<tr>${row.cells.map((value) => cell(value, row.kind !== "post")).join("")}</tr>`).join("")
  return `<table>${body}</table>`
}
