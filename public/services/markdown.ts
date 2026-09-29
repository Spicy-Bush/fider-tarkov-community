import { Marked } from "marked"
import DOMPurify from "dompurify"
import { decodeHTML, escapeUTF8 as escapeHTML } from "entities"

if (DOMPurify.isSupported) {
  DOMPurify.setConfig({
    USE_PROFILES: { html: true },
    ADD_TAGS: ["iframe", "img"],
    ADD_ATTR: [
      "allow",
      "allowfullscreen",
      "frameborder",
      "sandbox",
      "src",
      "width",
      "height",
      "title",
      "target",
      "href",
      "alt",
      "class",
    ],
  })
}

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

const defaultLink = (href: string, title: string | null | undefined, html: string): string => {
  const titleAttr = title ? ` title="${escapeHTML(decodeHTML(title))}"` : ""
  return `<a class="text-link" href="${escapeHTML(href)}"${titleAttr} rel="noopener nofollow" target="_blank">${html}</a>`
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

const createFullMarked = (embedImages: boolean) => {
  const marked = new Marked({
    gfm: true,
    breaks: true,
  })

  marked.use({
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
      renderer: (token) => `<span class="mention">@${escapeHTML(token.name)}</span>`,
    }],
    renderer: {
      html({ text }) {
        return escapeHTML(text)
      },
      image({ href: input, title, text }) {
        const href = safeHref(input)
        if (!href) {
          return escapeHTML(decodeHTML(text))
        }

        if (!embedImages) {
          const titleText = decodeHTML(title || text) || href
          return defaultLink(href, title, escapeHTML(titleText))
        }

        const titleAttr = title ? ` title="${escapeHTML(decodeHTML(title))}"` : ""
        const altAttr = text ? ` alt="${escapeHTML(decodeHTML(text))}"` : ""
        return `<img src="${escapeHTML(href)}"${titleAttr}${altAttr} class="max-w-full rounded-card" />`
      },
      link({ href: input, title, text, tokens }) {
        const html = this.parser.parseInline(tokens)
        const href = safeHref(input)
        if (!href) {
          return html
        }

        const video = videoEmbed(href)
        if (video) {
          return `<iframe style="width: 100%; height: auto; aspect-ratio: 16/9;" src="${escapeHTML(video.src)}" frameborder="0" allow="${video.allow}" allowfullscreen sandbox="allow-same-origin allow-scripts allow-presentation" title="${video.title}"></iframe>`
        }

        const isImage = /\.(jpg|jpeg|png|gif|webp|svg|bmp)(?:[?#]|$)/i.test(href) || href.includes("/static/images/")
        if (embedImages && isImage && input === text) {
          return `<img src="${escapeHTML(href)}" class="max-w-full rounded-card my-4" />`
        }

        return defaultLink(href, title, html)
      },
    },
  })

  return marked
}

const fullMarked = createFullMarked(false)
const fullMarkedWithEmbeds = createFullMarked(true)

const plainTextMarked = new Marked({
  gfm: true,
  breaks: true,
})

plainTextMarked.use({
  renderer: {
    link({ text }) {
      return text
    },
    image() {
      return ""
    },
    br() {
      return " "
    },
    strong({ text }) {
      return text
    },
    list({ items }) {
      return items.map(item => item.text).join(" ")
    },
    listitem({ text }) {
      return `${text} `
    },
    heading({ text }) {
      return text
    },
    paragraph({ text }) {
      return ` ${text} `
    },
    code({ text }) {
      return text
    },
    codespan({ text }) {
      return text
    },
    html({ text }) {
      return text
    },
    del({ text }) {
      return text
    },
  },
})

const sanitize = (input: string) => DOMPurify.isSupported ? DOMPurify.sanitize(input) : input

export const full = (input: string, embedImages = false): string => {
  const marked = embedImages ? fullMarkedWithEmbeds : fullMarked
  const parsed = marked.parse(input) as string
  return sanitize(parsed).trim()
}

export const plainText = (input: string): string => {
  const escaped = input.replace(/</g, "&lt;").replace(/>/g, "&gt;")
  return sanitize(plainTextMarked.parse(escaped) as string).trim()
}
