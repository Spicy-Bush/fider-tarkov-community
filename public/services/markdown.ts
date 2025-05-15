import { marked } from "marked"
import DOMPurify from "dompurify"

marked.setOptions({
  headerIds: false,
  xhtml: true,
  smartLists: true,
  gfm: true,
  breaks: true,
})

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

const defaultLink = (href: string, title: string, text: string) => {
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
  // Match direct video links like https://vkvideo.ru/video-89771130_456241539
  // or https://vk.com/video-89771130_456242098
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

const fullRenderer = new marked.Renderer()
fullRenderer.image = () => ""

fullRenderer.link = (href: string | null, title: string, text: string) => {
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
}

fullRenderer.text = (text: string) => {
  // Handling mention links (they're in the format @{id:1234, name:'John Doe'})
  return text.replace(/@{([^}]+)}/g, (match) => {
    try {
      const json = match.substring(1).replace(/&quot;/g, '"')
      const mention = JSON.parse(json)
      return `<span class="text-blue-600">@${mention.name}</span>`
    } catch {
      return match
    }
  })
}

const plainTextRenderer = new marked.Renderer()
plainTextRenderer.link = (_href, _title, text) => text
plainTextRenderer.image = () => ""
plainTextRenderer.br = () => " "
plainTextRenderer.strong = (text) => text
plainTextRenderer.list = (body) => body
plainTextRenderer.listitem = (text) => `${text} `
plainTextRenderer.heading = (text) => text
plainTextRenderer.paragraph = (text) => ` ${text} `
plainTextRenderer.code = (code) => code
plainTextRenderer.codespan = (code) => code
plainTextRenderer.html = (html) => html
plainTextRenderer.del = (text) => text

const entities: { [key: string]: string } = {
  "<": "&lt;",
  ">": "&gt;",
}

const encodeHTML = (s: string) => s.replace(/[<>]/g, (tag) => entities[tag] || tag)
const sanitize = (input: string) => DOMPurify.isSupported ? DOMPurify.sanitize(input) : input

export const full = (input: string): string => {
  return sanitize(marked(encodeHTML(input), { renderer: fullRenderer }).trim())
}

export const plainText = (input: string): string => {
  return sanitize(marked(encodeHTML(input), { renderer: plainTextRenderer }).trim())
}
