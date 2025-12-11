import { useState, useCallback } from "react"
import { Range, Editor, Transforms } from "slate"
import { ReactEditor } from "slate-react"
import { UserNames } from "@fider/models"

interface UseMentionPluginResult {
  target: Range | undefined
  setTarget: (target: Range | undefined) => void
  index: number
  setIndex: (index: number) => void
  search: string
  setSearch: (search: string) => void
  handleKeyDown: (event: React.KeyboardEvent<HTMLDivElement>, users: UserNames[]) => void
  handleChange: (editor: ReactEditor) => void
  insertMention: (user: UserNames) => void
}

export const useMentionPlugin = (editor: ReactEditor): UseMentionPluginResult => {
  const [target, setTarget] = useState<Range | undefined>()
  const [index, setIndex] = useState(0)
  const [search, setSearch] = useState("")

  const insertMention = useCallback(
    (user: UserNames) => {
      if (target) {
        Transforms.select(editor, target)
      }
      const mention = {
        type: "mention" as const,
        character: user.name,
        children: [{ text: "" }],
      }
      Transforms.insertNodes(editor, mention)
      Transforms.move(editor)
      setTarget(undefined)
    },
    [editor, target]
  )

  const handleKeyDown = useCallback(
    (event: React.KeyboardEvent<HTMLDivElement>, users: UserNames[]) => {
      if (target && users.length > 0) {
        switch (event.key) {
          case "ArrowDown": {
            event.preventDefault()
            const prevIndex = index >= users.length - 1 ? 0 : index + 1
            setIndex(prevIndex)
            break
          }
          case "ArrowUp": {
            event.preventDefault()
            const nextIndex = index <= 0 ? users.length - 1 : index - 1
            setIndex(nextIndex)
            break
          }
          case "Tab":
          case "Enter":
            event.preventDefault()
            insertMention(users[index])
            break
          case "Escape":
            event.preventDefault()
            setTarget(undefined)
            break
        }
      }
    },
    [target, index, insertMention]
  )

  const handleChange = useCallback(
    (editorInstance: ReactEditor) => {
      const { selection } = editorInstance

      if (selection && Range.isCollapsed(selection)) {
        const [start] = Range.edges(selection)
        const wordBefore = Editor.before(editorInstance, start, { unit: "word" })
        const before = wordBefore && Editor.before(editorInstance, wordBefore)
        const beforeRange = before && Editor.range(editorInstance, before, start)
        const beforeText = beforeRange && Editor.string(editorInstance, beforeRange)
        const beforeMatch = beforeText && beforeText.match(/^@(\w+)$/)

        const charBefore = Editor.before(editorInstance, start, { unit: "character" })
        const charBeforeRange = charBefore && Editor.range(editorInstance, charBefore, start)
        const charBeforeText = charBeforeRange && Editor.string(editorInstance, charBeforeRange)

        if (charBeforeText === "@") {
          setTarget({
            anchor: { path: start.path, offset: start.offset - 1 },
            focus: start,
          })
          setSearch("")
          setIndex(0)
        } else if (beforeMatch) {
          setTarget(beforeRange)
          setSearch(beforeMatch[1])
          setIndex(0)
        } else {
          setTarget(undefined)
        }
      }
    },
    []
  )

  return {
    target,
    setTarget,
    index,
    setIndex,
    search,
    setSearch,
    handleKeyDown,
    handleChange,
    insertMention,
  }
}

