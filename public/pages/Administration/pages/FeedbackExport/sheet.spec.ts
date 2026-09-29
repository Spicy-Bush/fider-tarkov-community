import { expect, test } from "@jest/globals"
import { FeedbackExportRow } from "@fider/models"
import { buildSheet, toCSV, toHTML, toTSV } from "./sheet"

const row = (number: number, title: string, votes: number): FeedbackExportRow => ({
  number,
  title,
  slug: `post-${number}`,
  votes,
  comments: 0,
  tagIds: [],
  pick: "top",
})

const options = { baseURL: "https://tarkov.community", date: "2026-09-28", title: "Weekly", titleRow: true }

test("one section has a title row, a header and one line per post", () => {
  const sheet = buildSheet([{ name: "Top bugs", rows: [row(12, "Desync on Customs", 812)] }], options)
  expect(toTSV(sheet)).toBe(
    [
      "Weekly (2026-09-28)",
      "Post\tVotes\tNikita\tVersion\tGD\tGD Contractor",
      '=HYPERLINK("https://tarkov.community/posts/12/post-12","Desync on Customs")\t812\t\t\t\t',
    ].join("\n")
  )
})

test("several sections are labelled, and the title row is optional", () => {
  const sheet = buildSheet(
    [
      { name: "Top bugs", rows: [row(1, "A", 5)] },
      { name: "Arena", rows: [row(2, "B", -3)] },
    ],
    { ...options, titleRow: false }
  )
  expect(
    toTSV(sheet)
      .split("\n")
      .map((line) => line.split("\t")[0])
  ).toEqual([
    "Post",
    "Top bugs",
    '=HYPERLINK("https://tarkov.community/posts/1/post-1","A")',
    "Arena",
    '=HYPERLINK("https://tarkov.community/posts/2/post-2","B")',
  ])
})

test("user text cannot become a formula, and quotes stay inside the link text", () => {
  const sheet = buildSheet(
    [
      { name: "=IMPORTXML(1)", rows: [row(3, 'Say "hi"', 1)] },
      { name: "-5 things", rows: [row(4, "=SUM(1)", 1)] },
    ],
    { ...options, titleRow: false }
  )
  const cells = toTSV(sheet)
    .split("\n")
    .map((line) => line.split("\t")[0])
  expect(cells[1]).toBe("'=IMPORTXML(1)")
  expect(cells[2]).toBe('=HYPERLINK("https://tarkov.community/posts/3/post-3","Say ""hi""")')
  expect(cells[3]).toBe("'-5 things")
  expect(cells[4]).toBe('=HYPERLINK("https://tarkov.community/posts/4/post-4","=SUM(1)")')
})

test("CSV quotes formulas that contain commas and HTML keeps links", () => {
  const sheet = buildSheet([{ name: "Bugs", rows: [row(5, "Crash, then <freeze>", 7)] }], options)
  expect(toCSV(sheet).split("\r\n")[2]).toBe('"=HYPERLINK(""https://tarkov.community/posts/5/post-5"",""Crash, then <freeze>"")",7,,,,')
  expect(toHTML(sheet)).toContain('<td><a href="https://tarkov.community/posts/5/post-5">Crash, then &lt;freeze&gt;</a></td><td>7</td>')
})

test("leading whitespace cannot turn a section label into a formula", () => {
  const sheet = buildSheet(
    [
      { name: " \t=1+1", rows: [row(1, "First post", 1)] },
      { name: "\r\n+1+1", rows: [row(2, "Second post", 2)] },
    ],
    { ...options, titleRow: false }
  )

  expect(toTSV(sheet).split("\n")[1]).toBe("'  =1+1")
  expect(toTSV(sheet).split("\n")[3]).toBe("' +1+1")
})
