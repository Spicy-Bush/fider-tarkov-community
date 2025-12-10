import { Marked } from "marked"
import DOMPurify from "dompurify"

if (DOMPurify.isSupported) {
  DOMPurify.setConfig({
    USE_PROFILES: { html: true },
    ADD_TAGS: ["iframe"],
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
    ]
  })
}

const defaultLink = (href: string, title: string | null | undefined, text: string) => {
  const titleAttr = title ? ` title="${title}"` : ""
  return `<a class="text-link" href="${href}"${titleAttr} rel="noopener nofollow" target="_blank">${text}</a>`
}

const parseYouTubeLink = (href: string) => {
  if (href.includes('youtube.com/watch')) {
    const url = new URL(href);
    const videoId = url.searchParams.get('v');
    let timestamp = url.searchParams.get('t') || url.searchParams.get('start') || '0';

    if (typeof timestamp === 'string' && timestamp.endsWith('s')) {
      timestamp = timestamp.slice(0, -1);
    }

    return videoId ? { videoId, timestamp } : null;
  }
  
  if (href.includes('youtu.be/')) {
    const url = new URL(href);
    const pathParts = url.pathname.split('/').filter(Boolean);
    const videoId = pathParts[0];
    
    let timestamp = url.searchParams.get('t') || '0';
    
    if (typeof timestamp === 'string' && timestamp.endsWith('s')) {
      timestamp = timestamp.slice(0, -1);
    }
    
    return { videoId, timestamp };
  }
  
  return null;
}

const parseVKVideoLink = (href: string) => {
  try {
    const url = new URL(href);
    let oid, id, timestamp;
    
    if (url.hostname === 'vk.com' && url.pathname.startsWith('/video')) {
      const videoPath = url.pathname.substring(6);
      
      if (videoPath.includes('_')) {
        [oid, id] = videoPath.split('_');
        timestamp = url.searchParams.get('t');
        return { oid, id, timestamp };
      }
    }
    
    if (url.hostname === 'vkvideo.ru' && url.pathname.startsWith('/video')) {
      const videoPath = url.pathname.substring(6);
      
      if (videoPath.includes('_')) {
        [oid, id] = videoPath.split('_');
        timestamp = url.searchParams.get('t');
        return { oid, id, timestamp };
      }
    }
  } catch (e) {}
  
  return null;
}

const fullMarked = new Marked({
  gfm: true,
  breaks: true,
})

fullMarked.use({
  renderer: {
    image() {
      return ""
    },

    link({ href, title, text }) {
      if (!href) return text

      if ((href.includes('youtube.com/watch') || href.includes('youtu.be/'))) {
        const parsedLink = parseYouTubeLink(href)
        
        if (parsedLink) {
          const { videoId, timestamp } = parsedLink
          const embedUrl = `https://www.youtube.com/embed/${videoId}?start=${timestamp}`
          
          return `<iframe style="width: 100%; height: auto; aspect-ratio: 16/9;" src="${embedUrl}" frameborder="0" allow="accelerometer; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen sandbox="allow-same-origin allow-scripts allow-presentation" title="YouTube video"></iframe>`
        }
      }
      
      if (href.includes('vk.com/video') || href.includes('vkvideo.ru/video')) {
        const parsedLink = parseVKVideoLink(href)
        
        if (parsedLink) {
          const { oid, id, timestamp } = parsedLink
          let embedUrl = `https://vk.com/video_ext.php?oid=${oid}&id=${id}`;
          
          if (timestamp) {
            embedUrl += `&t=${timestamp}`;
          }
          
          return `<iframe style="width: 100%; height: auto; aspect-ratio: 16/9;" src="${embedUrl}" frameborder="0" allow="autoplay; encrypted-media; fullscreen; picture-in-picture;" allowfullscreen sandbox="allow-same-origin allow-scripts allow-presentation" title="VK video"></iframe>`
        }
      }
      
      return defaultLink(href, title, text)
    },

    text({ raw }) {
      return raw.replace(/@{([^}]+)}/g, (match) => {
        try {
          const json = match.substring(1).replace(/&quot;/g, '"')
          const mention = JSON.parse(json)
          return `<span class="text-blue-600">@${mention.name}</span>`
        } catch {
          return match
        }
      })
    },
  },
})

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

const entities: { [key: string]: string } = {
  "<": "&lt;",
  ">": "&gt;",
}

const encodeHTML = (s: string) => s.replace(/[<>]/g, (tag) => entities[tag] || tag)
const sanitize = (input: string) => DOMPurify.isSupported ? DOMPurify.sanitize(input) : input

export const full = (input: string): string => {
  return sanitize(fullMarked.parse(encodeHTML(input)) as string).trim()
}

export const plainText = (input: string): string => {
  return sanitize(plainTextMarked.parse(encodeHTML(input)) as string).trim()
}
