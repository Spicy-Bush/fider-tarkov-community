import React, { ReactNode, useRef, useEffect } from "react"
import ReactDOM from "react-dom"
import { UserNames } from "@fider/models"
import { Range } from "slate"
import { ReactEditor } from "slate-react"

const Portal = ({ children }: { children?: ReactNode }) => {
  return typeof document === "object" ? ReactDOM.createPortal(children, document.body) : null
}

interface MentionPortalProps {
  editor: ReactEditor
  target: Range | undefined
  users: UserNames[]
  selectedIndex: number
  onSelect: (user: UserNames) => void
}

export const MentionPortal: React.FC<MentionPortalProps> = ({
  editor,
  target,
  users,
  selectedIndex,
  onSelect,
}) => {
  const ref = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    if (target && users.length > 0 && ref.current) {
      const el = ref.current
      try {
        const domRange = ReactEditor.toDOMRange(editor, target)
        const rect = domRange.getBoundingClientRect()

        const viewportHeight = window.innerHeight
        const spaceBelow = viewportHeight - rect.bottom
        const spaceAbove = rect.top
        const portalHeight = Math.min(users.length * 40, 200)

        if (spaceBelow >= portalHeight || spaceBelow >= spaceAbove) {
          el.style.top = `${rect.bottom + window.scrollY + 4}px`
          el.style.bottom = "auto"
        } else {
          el.style.bottom = `${viewportHeight - rect.top - window.scrollY + 4}px`
          el.style.top = "auto"
        }

        el.style.left = `${rect.left + window.scrollX}px`
      } catch {}
    }
  }, [target, users.length, editor])

  if (!target || users.length === 0) {
    return null
  }

  return (
    <Portal>
      <div
        ref={ref}
        className="mention-portal bg-elevated"
        style={{
          position: "absolute",
          zIndex: 1000,
          padding: "3px",
          borderRadius: "4px",
          boxShadow: "0 1px 5px rgba(0,0,0,.2)",
          maxHeight: "200px",
          overflowY: "auto",
        }}
      >
        {users.map((user, i) => (
          <div
            key={user.id}
            onClick={() => onSelect(user)}
            style={{
              padding: "4px 8px",
              borderRadius: "3px",
              cursor: "pointer",
              background: i === selectedIndex ? "var(--color-surface-alt)" : "transparent",
            }}
          >
            @{user.name}
          </div>
        ))}
      </div>
    </Portal>
  )
}

