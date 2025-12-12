import React, { useState, useRef, useEffect, useCallback } from "react"
import ReactDOM from "react-dom"
import { classSet } from "@fider/services"
import { UserNames } from "@fider/models"
import { actions } from "@fider/services"

interface CommentEditorProps {
  initialValue?: string
  placeholder?: string
  onChange?: (value: string) => void
  onFocus?: React.FocusEventHandler<HTMLDivElement>
  className?: string
  readOnly?: boolean
}

const Portal = ({ children }: { children?: React.ReactNode }) => {
  return typeof document === "object" ? ReactDOM.createPortal(children, document.body) : null
}

const serializeContent = (element: HTMLElement): string => {
  let result = ""
  element.childNodes.forEach((node) => {
    if (node.nodeType === Node.TEXT_NODE) {
      result += node.textContent || ""
    } else if (node.nodeType === Node.ELEMENT_NODE) {
      const el = node as HTMLElement
      if (el.dataset.mentionId && el.dataset.mentionName) {
        result += `@{"id":${el.dataset.mentionId},"name":"${el.dataset.mentionName}","isNew":true}`
      } else if (el.tagName === "BR") {
        result += "\n"
      } else if (el.tagName === "DIV" || el.tagName === "P") {
        result += "\n" + serializeContent(el)
      } else {
        result += serializeContent(el)
      }
    }
  })
  return result
}

const deserializeToHTML = (text: string): string => {
  return text.replace(/@\{"id":(\d+),"name":"([^"]+)"[^}]*\}/g, (_, id, name) => {
    return `<span class="mention" contenteditable="false" data-mention-id="${id}" data-mention-name="${name}">@${name}</span>`
  }).replace(/\n/g, "<br>")
}

export const CommentEditor: React.FC<CommentEditorProps> = (props) => {
  const [users, setUsers] = useState<UserNames[]>([])
  const [showDropdown, setShowDropdown] = useState(false)
  const [dropdownIndex, setDropdownIndex] = useState(0)
  const [search, setSearch] = useState("")
  const [dropdownPosition, setDropdownPosition] = useState({ top: 0, left: 0 })
  const [mentionRange, setMentionRange] = useState<Range | null>(null)
  
  const editorRef = useRef<HTMLDivElement>(null)
  const initializedRef = useRef(false)

  useEffect(() => {
    if (editorRef.current && !initializedRef.current) {
      if (props.initialValue) {
        editorRef.current.innerHTML = deserializeToHTML(props.initialValue)
      }
      initializedRef.current = true
    }
  }, [props.initialValue])

  const handleFocus = useCallback((e: React.FocusEvent<HTMLDivElement>) => {
    if (editorRef.current && editorRef.current.innerHTML === "") {
      const textNode = document.createTextNode("")
      editorRef.current.appendChild(textNode)
      const selection = window.getSelection()
      if (selection) {
        const range = document.createRange()
        range.setStart(textNode, 0)
        range.collapse(true)
        selection.removeAllRanges()
        selection.addRange(range)
      }
    }
    props.onFocus?.(e)
  }, [props.onFocus])

  useEffect(() => {
    if (showDropdown && search !== undefined) {
      const loadUsers = async () => {
        const result = await actions.getTaggableUsers(search)
        if (result.ok) {
          setUsers(result.data)
          setDropdownIndex(0)
        }
      }
      loadUsers()
    }
  }, [search, showDropdown])

  const updateDropdownPosition = useCallback(() => {
    const selection = window.getSelection()
    if (!selection || selection.rangeCount === 0) return

    const range = selection.getRangeAt(0)
    const rect = range.getBoundingClientRect()
    
    if (rect.top === 0 && rect.left === 0) {
      if (editorRef.current) {
        const editorRect = editorRef.current.getBoundingClientRect()
        setDropdownPosition({
          top: editorRect.top + 24 + window.scrollY,
          left: editorRect.left + window.scrollX,
        })
      }
      return
    }
    
    setDropdownPosition({
      top: rect.bottom + window.scrollY + 4,
      left: rect.left + window.scrollX,
    })
  }, [])

  const checkForMention = useCallback(() => {
    const selection = window.getSelection()
    if (!selection || selection.rangeCount === 0) return

    const range = selection.getRangeAt(0)
    if (!range.collapsed) {
      setShowDropdown(false)
      return
    }

    const node = range.startContainer
    if (node.nodeType !== Node.TEXT_NODE) {
      setShowDropdown(false)
      return
    }

    const text = node.textContent || ""
    const cursorPos = range.startOffset
    const textBeforeCursor = text.substring(0, cursorPos)
    
    const lastAtIndex = textBeforeCursor.lastIndexOf("@")
    if (lastAtIndex === -1) {
      setShowDropdown(false)
      return
    }

    const textAfterAt = textBeforeCursor.substring(lastAtIndex + 1)
    if (textAfterAt.includes(" ") || textAfterAt.includes("\n")) {
      setShowDropdown(false)
      return
    }

    const mentionStartRange = document.createRange()
    mentionStartRange.setStart(node, lastAtIndex)
    mentionStartRange.setEnd(node, cursorPos)
    
    setMentionRange(mentionStartRange)
    setSearch(textAfterAt)
    setShowDropdown(true)
    updateDropdownPosition()
  }, [updateDropdownPosition])

  const insertMention = useCallback((user: UserNames) => {
    if (!mentionRange || !editorRef.current) return

    mentionRange.deleteContents()
    
    const mentionSpan = document.createElement("span")
    mentionSpan.className = "mention"
    mentionSpan.contentEditable = "false"
    mentionSpan.dataset.mentionId = String(user.id)
    mentionSpan.dataset.mentionName = user.name
    mentionSpan.textContent = `@${user.name}`
    
    mentionRange.insertNode(mentionSpan)
    
    const space = document.createTextNode("\u00A0")
    mentionSpan.after(space)
    
    const selection = window.getSelection()
    if (selection) {
      const newRange = document.createRange()
      newRange.setStartAfter(space)
      newRange.collapse(true)
      selection.removeAllRanges()
      selection.addRange(newRange)
    }
    
    setShowDropdown(false)
    setMentionRange(null)
    
    if (props.onChange) {
      props.onChange(serializeContent(editorRef.current))
    }
  }, [mentionRange, props.onChange])

  const handleInput = useCallback(() => {
    if (!editorRef.current) return
    
    checkForMention()
    
    if (props.onChange) {
      props.onChange(serializeContent(editorRef.current))
    }
  }, [checkForMention, props.onChange])

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (!showDropdown || users.length === 0) return

    switch (e.key) {
      case "ArrowDown":
        e.preventDefault()
        setDropdownIndex((prev) => (prev >= users.length - 1 ? 0 : prev + 1))
        break
      case "ArrowUp":
        e.preventDefault()
        setDropdownIndex((prev) => (prev <= 0 ? users.length - 1 : prev - 1))
        break
      case "Tab":
      case "Enter":
        e.preventDefault()
        insertMention(users[dropdownIndex])
        break
      case "Escape":
        e.preventDefault()
        setShowDropdown(false)
        break
    }
  }, [showDropdown, users, dropdownIndex, insertMention])

  const handleBlur = useCallback(() => {
    setTimeout(() => {
      setShowDropdown(false)
    }, 200)
  }, [])

  return (
    <>
      <div
        ref={editorRef}
        contentEditable={!props.readOnly}
        onInput={handleInput}
        onKeyDown={handleKeyDown}
        onFocus={handleFocus}
        onBlur={handleBlur}
        data-placeholder={props.placeholder}
        className={classSet({
          "bg-elevated w-full min-w-0 leading-6 p-2 border border-border rounded-input appearance-none my-3 min-h-[80px] outline-none whitespace-pre-wrap wrap-break-word": true,
          "cursor-not-allowed opacity-45 pointer-events-none": props.readOnly,
          "empty:before:content-[attr(data-placeholder)] empty:before:text-muted empty:before:pointer-events-none": true,
        })}
      />
      
      {showDropdown && !props.readOnly && (
        <Portal>
          <div
            className="bg-elevated rounded shadow-lg border border-border"
            style={{
              position: "absolute",
              top: dropdownPosition.top,
              left: dropdownPosition.left,
              zIndex: 10000,
              maxHeight: "200px",
              overflowY: "auto",
              minWidth: "150px",
            }}
          >
            {users.length > 0 ? (
              users.map((user, i) => (
                <div
                  key={user.id}
                  onMouseDown={(e) => {
                    e.preventDefault()
                    insertMention(user)
                  }}
                  className={classSet({
                    "px-3 py-2 cursor-pointer text-sm": true,
                    "bg-accent-light": i === dropdownIndex,
                    "hover:bg-surface-alt": i !== dropdownIndex,
                  })}
                >
                  @{user.name}
                </div>
              ))
            ) : (
              <div className="px-3 py-2 text-sm text-muted">No users found</div>
            )}
          </div>
        </Portal>
      )}
    </>
  )
}
