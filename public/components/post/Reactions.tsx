import React, { useEffect, useState } from "react"
import { ReactionCount } from "@fider/models"
import { Icon } from "@fider/components"
import { heroiconsSmile as IconSmile } from "@fider/icons.generated"
import { classSet } from "@fider/services"
import { useFider } from "@fider/hooks"

interface ReactionsProps {
  emojiSelectorRef: React.RefObject<HTMLDivElement>
  toggleReaction: (emoji: string) => void
  reactions?: ReactionCount[]
}

const availableEmojis = ["👍", "👎", "❤️", "🤔", "👏", "😂", "😲"]

export const Reactions: React.FC<ReactionsProps> = ({ emojiSelectorRef, toggleReaction, reactions }) => {
  const fider = useFider()
  const [isEmojiSelectorOpen, setIsEmojiSelectorOpen] = useState(false)

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (emojiSelectorRef.current && !emojiSelectorRef.current.contains(event.target as Node)) {
        setIsEmojiSelectorOpen(false)
      }
    }

    document.addEventListener("click", handleClickOutside)
    return () => {
      document.removeEventListener("click", handleClickOutside)
    }
  }, [])

  return (
    <div ref={emojiSelectorRef} className="relative">
      <div className="flex flex-wrap items-center gap-1.5 mt-3">
        {fider.session.isAuthenticated && (
          <button
            type="button"
            onClick={() => setIsEmojiSelectorOpen(!isEmojiSelectorOpen)}
            className={classSet({
              "inline-flex items-center justify-center w-7 h-7 rounded-button border transition-all duration-50 cursor-pointer": true,
              "bg-transparent border-border text-muted hover:text-foreground hover:border-border-strong": !isEmojiSelectorOpen,
              "bg-primary/20 border-primary text-primary": isEmojiSelectorOpen,
            })}
          >
            <Icon width="16" height="16" sprite={IconSmile} />
          </button>
        )}
        
        {reactions !== undefined && reactions.map((reaction) => (
          <button
            type="button"
            key={reaction.emoji}
            onClick={fider.session.isAuthenticated ? () => toggleReaction(reaction.emoji) : undefined}
            disabled={!fider.session.isAuthenticated}
            className={classSet({
              "inline-flex items-center gap-1 px-1.5 py-0.5 text-sm transition-all duration-100 rounded-badge": true,
              "cursor-pointer hover:scale-105": fider.session.isAuthenticated,
              "cursor-default": !fider.session.isAuthenticated,
              "reaction-active": reaction.includesMe,
            })}
          >
            <span className={classSet({
              "text-base": true,
              "emoji-shadow": !reaction.includesMe,
              "emoji-glow": reaction.includesMe,
            })}>{reaction.emoji}</span>
            <span className={classSet({
              "font-semibold text-xs": true,
              "text-primary": reaction.includesMe,
              "text-muted": !reaction.includesMe,
            })}>{reaction.count}</span>
          </button>
        ))}
      </div>
      
      {isEmojiSelectorOpen && (
        <div className="absolute left-0 bottom-full mb-2 flex gap-1 p-2 bg-elevated border border-border rounded-card shadow-lg z-50">
          {availableEmojis.map((emoji) => (
            <button
              type="button"
              key={emoji}
              className="w-8 h-8 flex items-center justify-center text-lg rounded-button hover:bg-tertiary transition-colors duration-100 cursor-pointer"
              onClick={() => {
                toggleReaction(emoji)
                setIsEmojiSelectorOpen(false)
              }}
            >
              {emoji}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

