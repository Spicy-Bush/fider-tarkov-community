import { createElement, Fragment, ReactNode } from "react"
import { Marked, MarkedToken, Token, Tokens } from "marked"
import { decodeHTML } from "entities"

const safeHref = (input: string): string | null => {
  const href = decodeHTML(input).trim()
  if (!href || /[\u0000-\u001f\u007f\\]/.test(href)) {
    return null
  }

  const scheme = /^([^/?#]*):/.exec(href)
  if (scheme && !/^(https?|mailto)$/i.test(scheme[1])) {
    return null
  }

  return href
}

const defaultLink = (href: string, title: string | null | undefined, children: ReactNode): ReactNode => {
  return createElement("a", {
    className: "text-link",
    href,
    title: title ? decodeHTML(title) : undefined,
    rel: "noopener nofollow",
    target: "_blank",
  }, children)
}

const videoEmbed = (href: string): { src: string; title: string; allow: string } | null => {
  const match = /^https?:\/\/(?:www\.)?(youtube\.com|youtu\.be|vk\.com|vkvideo\.ru)(\/[^?#]*)(?:\?([^#]*))?(?:#.*)?$/i.exec(href)
  if (!match) {
    return null
  }

  const host = match[1].toLowerCase()
  const path = match[2]
  const parameters = new Map<string, string>()

  try {
    for (const pair of (match[3] || "").split("&")) {
      const separator = pair.indexOf("=")
      const key = decodeURIComponent(separator < 0 ? pair : pair.slice(0, separator))
      const value = decodeURIComponent(separator < 0 ? "" : pair.slice(separator + 1).replace(/\+/g, " "))
      if (!parameters.has(key)) {
        parameters.set(key, value)
      }
    }
  } catch {
    return null
  }

  const timestamp = parameters.get("t") || parameters.get("start") || "0"
  if (!/^\d+s?$/.test(timestamp)) {
    return null
  }

  const seconds = timestamp.replace(/s$/, "")
  if (host === "youtube.com" || host === "youtu.be") {
    let videoId: string | undefined
    if (host === "youtu.be") {
      videoId = path.slice(1)
    } else if (path === "/watch") {
      videoId = parameters.get("v")
    }

    if (!videoId || !/^[\w-]+$/.test(videoId)) {
      return null
    }

    return {
      src: `https://www.youtube.com/embed/${videoId}?start=${seconds}`,
      title: "YouTube video",
      allow: "accelerometer; clipboard-write; encrypted-media; gyroscope; picture-in-picture",
    }
  }

  const video = /^\/video(-?\d+)_(\d+)\/?$/.exec(path)
  if (!video) {
    return null
  }

  return {
    src: `https://vk.com/video_ext.php?oid=${video[1]}&id=${video[2]}&t=${seconds}`,
    title: "VK video",
    allow: "autoplay; encrypted-media; fullscreen; picture-in-picture",
  }
}

const fullMarked = new Marked({ gfm: true, breaks: true })

fullMarked.use({
  extensions: [{
    name: "mention",
    level: "inline",
    start: (source) => source.indexOf("@{"),
    tokenizer(source) {
      const match = /^@\{(?:[^{}"\\]|"(?:\\.|[^"\\])*")*\}/.exec(source)
      if (!match) {
        return undefined
      }

      try {
        const mention = JSON.parse(match[0].slice(1))
        if (typeof mention.name === "string") {
          return { type: "mention", raw: match[0], name: mention.name }
        }
      } catch {
        return undefined
      }

      return undefined
    },
  }],
})

const plainTextMarked = new Marked({
  gfm: true,
  breaks: true,
})

const renderTokens = (tokens: Token[], embedImages: boolean): ReactNode =>
  createElement(Fragment, null, ...tokens.map(token => renderToken(token, embedImages)))

type Mention = { type: "mention"; name: string }

const renderToken = (source: Token, embedImages: boolean): ReactNode => {
  const token = source as MarkedToken | Mention

  switch (token.type) {
    case "space":
    case "def":
      return null

    case "mention":
      return createElement("span", { className: "mention" }, "@" + token.name)

    case "html":
      return token.text

    case "text":
    case "escape":
      return "tokens" in token && token.tokens ? renderTokens(token.tokens, embedImages) : decodeHTML(token.text)

    case "codespan":
      return createElement("code", null, token.text)

    case "code": {
      const language = (token.lang || "").split(/\s+/)[0]
      return createElement("pre", null, createElement("code", {
        className: language ? "language-" + language : undefined,
      }, token.text.replace(/\n$/, "") + "\n"))
    }

    case "hr":
    case "br":
      return createElement(token.type)

    case "checkbox":
      return createElement("input", { type: "checkbox", checked: token.checked, disabled: true })

    case "heading":
      return createElement(["h1", "h2", "h3", "h4", "h5", "h6"][token.depth - 1], null, renderTokens(token.tokens, embedImages))

    case "paragraph":
    case "blockquote":
    case "strong":
    case "em":
    case "del": {
      const tag = token.type === "paragraph" ? "p" : token.type
      return createElement(tag, null, renderTokens(token.tokens, embedImages))
    }

    case "list": {
      const start = token.ordered && token.start !== 1 ? token.start : undefined
      const items = token.items.map(item => createElement("li", null, renderTokens(item.tokens, embedImages)))
      return createElement(token.ordered ? "ol" : "ul", { start }, ...items)
    }

    case "table": {
      const row = (cells: Tokens.TableCell[]) => {
        const children = cells.map(cell => createElement(cell.header ? "th" : "td", {
          align: cell.align || undefined,
        }, renderTokens(cell.tokens, embedImages)))

        return createElement("tr", null, ...children)
      }

      return createElement("table", null,
        createElement("thead", null, row(token.header)),
        token.rows.length ? createElement("tbody", null, ...token.rows.map(row)) : null)
    }

    case "image": {
      const href = safeHref(token.href)
      const alt = decodeHTML(token.text)
      if (!href) {
        return alt
      }

      if (!embedImages) {
        return defaultLink(href, token.title, decodeHTML(token.title || token.text) || href)
      }

      return createElement("img", {
        src: href,
        title: token.title ? decodeHTML(token.title) : undefined,
        alt,
        className: "max-w-full rounded-card",
      })
    }

    case "link": {
      const children = renderTokens(token.tokens, embedImages)
      const href = safeHref(token.href)
      if (!href) {
        return children
      }

      const video = videoEmbed(href)
      if (video) {
        return createElement("iframe", {
          src: video.src,
          title: video.title,
          allow: video.allow,
          allowFullScreen: true,
          frameBorder: "0",
          sandbox: "allow-same-origin allow-scripts allow-presentation",
          style: { width: "100%", height: "auto", aspectRatio: "16/9" },
        })
      }

      const isImage = /\.(jpg|jpeg|png|gif|webp|svg|bmp)(?:[?#]|$)/i.test(href) || href.includes("/static/images/")
      if (embedImages && isImage && token.href === token.text) {
        return createElement("img", { src: href, className: "max-w-full rounded-card my-4" })
      }

      return defaultLink(href, token.title, children)
    }

    default:
      throw new Error("Unsupported Markdown token: " + token.type)
  }
}

export const full = (input: string, embedImages = false): ReactNode => renderTokens(fullMarked.lexer(input), embedImages)

const tokenText = (source: Token): string => {
  const token = source as MarkedToken

  switch (token.type) {
    case "space":
    case "hr":
    case "image":
    case "def":
    case "checkbox":
      return ""

    case "br":
      return " "

    case "list":
      return token.items.map(item => decodeHTML(item.text)).join(" ")

    case "table": {
      const rows = [token.header, ...token.rows]
      return rows.map(row => row.map(cell => cell.tokens.map(tokenText).join("")).join(" ")).join(" ")
    }

    case "paragraph":
      return " " + decodeHTML(token.text) + " "

    default:
      return decodeHTML(token.text)
  }
}

export const plainText = (input: string): string => {
  const escaped = input.replace(/</g, "&lt;").replace(/>/g, "&gt;")
  return plainTextMarked.lexer(escaped).map(tokenText).join("").trim()
}
