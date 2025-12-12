// CommentEditor converted to Tailwind

import React, { useMemo, useCallback, useRef, useEffect, useState, Fragment, ReactNode } from "react"
import { Element, Editor, Transforms, Range, createEditor, Descendant, BaseEditor, BaseRange, Node } from "slate"
import { HistoryEditor, withHistory } from "slate-history"
import { Slate, Editable, ReactEditor, withReact, useSelected, useFocused, RenderPlaceholderProps } from "slate-react"
import { classSet } from "@fider/services"

export const IS_MAC = typeof navigator !== "undefined" && /Mac OS X/.test(navigator.userAgent)

import ReactDOM from "react-dom"
import { UserNames } from "@fider/models"
import { actions } from "@fider/services"

export type TextType = { text: string }

interface RenderElementProps {
  attributes: React.HTMLAttributes<HTMLElement>
  element: CustomElement
  children: React.ReactNode
}

type MentionElement = {
  type: "mention"
  character: string
  children: TextType[]
}

export type ParagraphElement = {
  type: "paragraph"
  children: Descendant[]
}

type CustomElement = MentionElement | ParagraphElement

type CustomEditor = BaseEditor &
  ReactEditor &
  HistoryEditor & {
    nodeToDecorations?: Map<Element, Range[]>
  }

const emptyValue: Descendant[] = [
  {
    type: "paragraph",
    children: [{ text: "" }],
  },
]

declare module "slate" {
  interface CustomTypes {
    Editor: CustomEditor
    Element: CustomElement
    Text: TextType
    Range: BaseRange & {
      [key: string]: unknown
    }
  }
}

const Portal = ({ children }: { children?: ReactNode }) => {
  return typeof document === "object" ? ReactDOM.createPortal(children, document.body) : null
}

interface CommentEditorProps {
  initialValue?: string
  placeholder?: string
  onChange?: (value: string) => void
  onFocus?: React.FocusEventHandler<HTMLDivElement>
  className?: string
  readOnly?: boolean
}

const Placeholder = ({ attributes, children }: RenderPlaceholderProps) => {
  return (
    <span
      {...attributes}
      className="absolute opacity-30 select-none pointer-events-none"
    >
      {children}
    </span>
  )
}

export const CommentEditor: React.FunctionComponent<CommentEditorProps> = (props) => {
  const [users, setUsers] = useState<UserNames[]>([])
  const ref = useRef<HTMLDivElement | null>(null)
  const [target, setTarget] = useState<Range | undefined>()
  const [index, setIndex] = useState(0)
  const [search, setSearch] = useState("")

  useEffect(() => {
    if (target) {
      const loadUsers = async () => {
        const result = await actions.getTaggableUsers(search)
        if (result.ok) {
          setUsers(result.data)
        }
      }
      loadUsers()
    }
  }, [search, target])
  const filteredUsers = users

  const renderElement = useCallback((props: RenderElementProps) => <SlateElement {...props} />, [])
  const editor = useMemo(() => withMentions(withReact(withHistory(createEditor()))), [])

  const onKeyDown = useCallback(
    (event: React.KeyboardEvent<HTMLDivElement>) => {
      if (target && filteredUsers.length > 0) {
        switch (event.key) {
          case "ArrowDown": {
            event.preventDefault()
            const prevIndex = index >= filteredUsers.length - 1 ? 0 : index + 1
            setIndex(prevIndex)
            break
          }
          case "ArrowUp": {
            event.preventDefault()
            const nextIndex = index <= 0 ? filteredUsers.length - 1 : index - 1
            setIndex(nextIndex)
            break
          }
          case "Tab":
          case "Enter":
            event.preventDefault()
            Transforms.select(editor, target)
            insertMention(editor, filteredUsers[index])
            setTarget(undefined)
            break
          case "Escape":
            event.preventDefault()
            setTarget(undefined)
            break
        }
      }
    },
    [filteredUsers, editor, index, target]
  )

  useEffect(() => {
    if (target && filteredUsers.length > 0 && ref.current) {
      const el = ref.current
      const domRange = ReactEditor.toDOMRange(editor, target)
      const rect = domRange.getBoundingClientRect()
      el.style.top = `${rect.top + window.pageYOffset + 24}px`
      el.style.left = `${rect.left + window.pageXOffset}px`
    }
  }, [filteredUsers.length, editor, index, search, target])

  const initialValue = props.initialValue ? deserialize(props.initialValue) : emptyValue

  return (
    <Slate
      editor={editor}
      initialValue={initialValue}
      onChange={(descendant) => {
        const { selection } = editor

        if (selection && Range.isCollapsed(selection)) {
          const [start] = Range.edges(selection)
          
          const currentPoint = { path: start.path, offset: start.offset }
          const [node] = Editor.node(editor, start.path)
          const textBeforeCursor = Node.string(node).slice(0, start.offset)
          const lastAtIndex = textBeforeCursor.lastIndexOf('@')
          if (lastAtIndex !== -1) {
            const potentialMention = textBeforeCursor.slice(lastAtIndex + 1)
            const after = Editor.after(editor, currentPoint)
            const isAtEndOrFollowedBySpace = !after || /^\s/.test(Editor.string(editor, Editor.range(editor, currentPoint, after)))
            
            if (!potentialMention.includes(' ') && isAtEndOrFollowedBySpace) {
              const mentionRange = {
                anchor: { path: start.path, offset: lastAtIndex },
                focus: currentPoint
              }
              
              setTarget(mentionRange)
              setSearch(potentialMention)
              setIndex(0)
              props.onChange && props.onChange(serialize(descendant))
              return
            }
          }

          props.onChange && props.onChange(serialize(descendant))
        }

        setTarget(undefined)
        props.onChange && props.onChange(serialize(descendant))
      }}
    >
      <Editable
        readOnly={props.readOnly || false}
        className={classSet({
          "bg-elevated w-full min-w-0 leading-6 p-2 resize-none border border-border rounded-input appearance-none my-3 break-all wrap-anywhere [&_p]:mb-0": true,
          "cursor-not-allowed opacity-45 pointer-events-none": props.readOnly,
        })}
        renderElement={renderElement}
        onKeyDown={onKeyDown}
        onFocus={props.onFocus}
        placeholder={props.placeholder}
        renderPlaceholder={Placeholder}
      />
      {target && filteredUsers.length > 0 && !props.readOnly && (
        <Portal>
          <div 
            ref={ref} 
            className="absolute z-10 p-1 bg-elevated rounded shadow-md"
            style={{ top: '-9999px', left: '-9999px' }}
            data-cy="mentions-portal"
          >
            {filteredUsers.map((user, i) => (
              <div
                key={user.id}
                onClick={() => {
                  Transforms.select(editor, target)
                  insertMention(editor, user)
                  setTarget(undefined)
                }}
                className={classSet({
                  "px-3 py-1.5 rounded cursor-pointer": true,
                  "bg-accent-light": i === index,
                })}
              >
                {user.name}
              </div>
            ))}
          </div>
        </Portal>
      )}
    </Slate>
  )
}

const withMentions = (editor: CustomEditor) => {
  const { isInline, isVoid, markableVoid } = editor

  editor.isInline = (element: CustomElement) => {
    return element.type === "mention" ? true : isInline(element)
  }

  editor.isVoid = (element: CustomElement) => {
    return element.type === "mention" ? true : isVoid(element)
  }

  editor.markableVoid = (element: CustomElement) => {
    return element.type === "mention" || markableVoid(element)
  }

  return editor
}

const insertMention = (editor: Editor, user: UserNames) => {
  const newMention = { ...user, isNew: true }
  const mention: MentionElement = {
    type: "mention",
    character: user.name,
    children: [{ text: "@" + JSON.stringify(newMention) }],
  }
  Transforms.insertNodes(editor, mention)
  Transforms.move(editor)
}

const SlateElement = (props: RenderElementProps) => {
  const { attributes, children, element } = props
  switch (element.type) {
    case "mention":
      return <Mention {...(props as { attributes: React.HTMLAttributes<HTMLSpanElement>; children: React.ReactNode; element: MentionElement })} />
    default:
      return <p {...attributes}>{children}</p>
  }
}

const Mention = ({
  attributes,
  children,
  element,
}: {
  attributes: React.HTMLAttributes<HTMLSpanElement>
  children: React.ReactNode
  element: MentionElement
}) => {
  const selected = useSelected()
  const focused = useFocused()
  
  return (
    <span 
      {...attributes} 
      className="mx-px align-baseline inline-block rounded bg-surface-alt text-sm"
      contentEditable={false} 
      data-cy={`mention-${element.character.replace(" ", "-")}`} 
      style={{ boxShadow: selected && focused ? "0 0 0 2px var(--color-primary)" : "none" }}
    >
      <div contentEditable={false}>
        {IS_MAC ? (
          <Fragment>
            {children}@{element.character}
          </Fragment>
        ) : (
          <Fragment>
            @{element.character}
            {children}
          </Fragment>
        )}
      </div>
    </span>
  )
}

const serialize = (nodes: Descendant[]): string => {
  return nodes.map((n) => Node.string(n)).join("\n")
}

const deserialize = (markdown: string): Descendant[] => {
  return markdown.split("\n").map((line) => {
    const children: Descendant[] = []
    const regex = /@{\\?"id\\?":\d+,\\?"name\\?":\\?"[^"]+\\?"(,\\?"isNew\\?":[^}]+)?}/g
    let lastIndex = 0

    let match
    while ((match = regex.exec(line)) !== null) {
      if (match.index > lastIndex) {
        children.push({ text: line.slice(lastIndex, match.index) })
      }

      try {
        const jsonStr = match[0].replace(/\\/g, "").slice(1)
        const mentionData = JSON.parse(jsonStr)
        children.push({
          type: "mention",
          character: mentionData.name,
          children: [{ text: match[0] }],
        })
      } catch (err) {
        console.error("Error parsing mention:", err)
        children.push({ text: line })
      }

      lastIndex = match.index + match[0].length
    }

    if (lastIndex <= line.length) {
      children.push({ text: line.slice(lastIndex) })
    }

    return {
      type: "paragraph",
      children,
    }
  })
}
