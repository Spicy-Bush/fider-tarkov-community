import { full, plainText } from "./markdown"
import React from "react"
import { renderToStaticMarkup } from "react-dom/server.node"

const render = (source: string, embedImages = false): HTMLElement => {
  const element = document.createElement("div")
  element.innerHTML = renderToStaticMarkup(React.createElement(React.Fragment, null, full(source, embedImages)))
  return element
}

test.each([
  String.raw`@{"name":"\u003cimg src=x onerror=alert(1)\u003e"}`,
  String.raw`@{"name":"\u003cmeta http-equiv=refresh content=0\u003e"}`,
  `[<img src=x onerror=alert(1)>](https://example.test)`,
  `[link](https://example.test '" onmouseover="alert(1)')`,
  `![" onerror="alert(1)](https://example.test/image.png)`,
  `![image](https://example.test/image.png '" onload="alert(1)')`,
  `[link](https://example.test 'title @{"name":"boom"}')`,
  `[link](<https://example.test/"onmouseover="alert(1)>)`,
  `<iframe src="javascript:alert(1)"></iframe>`,
  `<meta http-equiv="refresh" content="0;url=/signout">`,
])("renders untrusted Markdown as inert content: %s", (source) => {
  const element = render(source, true)

  expect(element.querySelector("script, meta, object, form, iframe")).toBeNull()
  for (const node of element.querySelectorAll("*")) {
    expect(node.getAttributeNames().some((name) => name.startsWith("on"))).toBe(false)
  }
})

test("preserves completed and incomplete task lists", () => {
  const element = render("- [x] done\n- [ ] todo")
  const tasks = Array.from(element.querySelectorAll<HTMLInputElement>('input[type="checkbox"]'))

  expect(tasks.map((task) => task.checked)).toEqual([true, false])
  expect(tasks.every((task) => task.disabled)).toBe(true)
})

test("preserves headings, nested lists, aligned tables and code blocks", () => {
  const element = render([
    "## Heading",
    "3. Third\n4. Fourth\n   - nested",
    "| left | right |\n| :--- | ---: |\n| a | b |",
    "```js\nconst value = '<tag>';\n```",
  ].join("\n\n"))

  expect(element.querySelector("h2")?.textContent).toBe("Heading")
  expect(element.querySelector("ol")?.start).toBe(3)
  expect(element.querySelector("ol ul li")?.textContent).toBe("nested")
  expect(Array.from(element.querySelectorAll("th")).map(cell => cell.align)).toEqual(["left", "right"])
  expect(Array.from(element.querySelectorAll("td")).map(cell => cell.textContent)).toEqual(["a", "b"])
  expect(element.querySelector("pre code.language-js")?.textContent).toBe("const value = '<tag>';\n")
})

test.each([
  "javascript:alert(1)",
  "JaVaScRiPt:alert(1)",
  "javascript&colon;alert(1)",
  "jav&#x61;script:alert(1)",
  "java&#10;script:alert(1)",
  "java&Tab;script:alert(1)",
  "data:text/html;base64,PHNjcmlwdD4=",
  "vbscript:msgbox(1)",
])("rejects executable link and image schemes: %s", (href) => {
  const element = render(`[link](<${href}>) ![image](<${href}>)`, true)

  expect(element.querySelector("a, img, iframe")).toBeNull()
  expect(element.textContent).toContain("link")
  expect(element.textContent).toContain("image")
})

test("preserves formatting, link entities, image metadata and text mentions", () => {
  const source = [
    `**bold** and *italic* and ~~removed~~`,
    `[**formatted**](https://example.test/?a=1&amp;b=2 'A &quot;title&quot;')`,
    `![A &quot;picture&quot;](../image.png 'A &amp; B')`,
    `@{"id":4,"name":"A & B {staff}"}`,
    `\`@{"name":"code"}\``,
  ].join("\n\n")
  const element = render(source, true)

  expect(element.querySelector("strong")?.textContent).toBe("bold")
  expect(element.querySelector("em")?.textContent).toBe("italic")
  expect(element.querySelector("del")?.textContent).toBe("removed")
  expect(element.querySelector("a strong")?.textContent).toBe("formatted")
  expect(element.querySelector("a")?.getAttribute("href")).toBe("https://example.test/?a=1&b=2")
  expect(element.querySelector("a")?.title).toBe('A "title"')
  expect(element.querySelector("img")?.alt).toBe('A "picture"')
  expect(element.querySelector("img")?.title).toBe("A & B")
  expect(element.querySelector(".mention")?.textContent).toBe("@A & B {staff}")
  expect(element.querySelector("code")?.textContent).toBe('@{"name":"code"}')
  expect(element.querySelectorAll(".mention")).toHaveLength(1)
})

test.each(["/posts/1", "../page", "#comment-1", "?page=2", "https://example.test", "mailto:hello@example.test"])(
  "preserves safe link destinations: %s",
  (href) => {
    expect(render(`[link](${href})`).querySelector("a")?.getAttribute("href")).toBe(href)
  },
)

test.each([
  ["https://www.youtube.com/watch?v=abc_123-xyz&t=30s", "https://www.youtube.com/embed/abc_123-xyz?start=30"],
  ["https://youtu.be/abc_123-xyz?start=12", "https://www.youtube.com/embed/abc_123-xyz?start=12"],
  ["https://vk.com/video-123_456?t=30", "https://vk.com/video_ext.php?oid=-123&id=456&t=30"],
  ["https://vkvideo.ru/video123_456", "https://vk.com/video_ext.php?oid=123&id=456&t=0"],
])("embeds supported videos without a browser URL API: %s", (href, src) => {
  const original = global.URL
  Reflect.deleteProperty(global, "URL")

  try {
    const element = render(`[video](${href})`)
    expect(element.querySelector("iframe")?.getAttribute("src")).toBe(src)
    expect(element.querySelector("iframe")?.getAttribute("sandbox")).toBe("allow-same-origin allow-scripts allow-presentation")
  } finally {
    global.URL = original
  }
})

test.each([
  "https://evil.test/youtube.com/watch?v=abc",
  "https://youtube.com.evil.test/watch?v=abc",
  "https://youtube.com@evil.test/watch?v=abc",
  "youtube.com/watch?v=abc",
  "https://youtube.com/watch?v=%22%20onload%3Dalert(1)",
  "https://youtube.com/watch?v=abc&t=%22%20onload%3Dalert(1)",
  "https://youtube.com/watch?v=%E0%A4%A",
  "https://vk.com/video-123_456?t=%22%20onload%3Dalert(1)",
])("keeps unsupported or malformed videos as ordinary links: %s", (href) => {
  const element = render(`[video](<${href}>)`)

  expect(element.querySelector("iframe")).toBeNull()
  expect(element.querySelector("a")?.getAttribute("href")).toBe(href)
})

test("keeps nonembedded images as links and raw HTML inert in both styles", () => {
  const image = render(`![A & B](/static/images/example.png)`)
  expect(image.querySelector("img")).toBeNull()
  expect(image.querySelector("a")?.textContent).toBe("A & B")

  const element = document.createElement("div")
  element.textContent = plainText("<script>alert(1)</script> and **words**")
  expect(element.querySelector("script")).toBeNull()
  expect(element.textContent).toContain("<script>alert(1)</script>")
})
