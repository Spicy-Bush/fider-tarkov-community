import { Descendant, Element, Text } from "slate"

interface MentionElement {
  type: "mention"
  character: string
  children: { text: string }[]
}

interface ParagraphElement {
  type: "paragraph"
  children: Descendant[]
}

type CustomElement = MentionElement | ParagraphElement

const isMention = (node: CustomElement): node is MentionElement => {
  return node.type === "mention"
}

export const serialize = (nodes: Descendant[]): string => {
  return nodes
    .map((n) => {
      if (Text.isText(n)) {
        return n.text
      }

      const element = n as CustomElement
      if (isMention(element)) {
        return `@${element.character}`
      }

      if (Element.isElement(n)) {
        return serialize(n.children)
      }

      return ""
    })
    .join("\n")
}

export const deserialize = (text: string): Descendant[] => {
  if (!text) {
    return [{ type: "paragraph", children: [{ text: "" }] }]
  }

  const lines = text.split("\n")
  return lines.map((line) => {
    const children: Descendant[] = []
    const mentionRegex = /@([a-zA-Z0-9_-]+)/g
    let lastIndex = 0
    let match: RegExpExecArray | null

    while ((match = mentionRegex.exec(line)) !== null) {
      if (match.index > lastIndex) {
        children.push({ text: line.slice(lastIndex, match.index) })
      }

      children.push({
        type: "mention",
        character: match[1],
        children: [{ text: "" }],
      } as MentionElement)

      lastIndex = match.index + match[0].length
    }

    if (lastIndex < line.length) {
      children.push({ text: line.slice(lastIndex) })
    }

    if (children.length === 0) {
      children.push({ text: "" })
    }

    return { type: "paragraph", children } as ParagraphElement
  })
}

export const emptyValue: Descendant[] = [
  {
    type: "paragraph",
    children: [{ text: "" }],
  },
]

