import React from "react"

const MentionSelector: React.FC<{ names: string[]; cursorPosition: { top: number; left: number } }> = ({ names, cursorPosition }) => {
  return (
    <div
      className="absolute z-1000 w-[200px] rounded bg-elevated border border-surface-alt p-2"
      style={{
        top: cursorPosition.top,
        left: cursorPosition.left,
      }}
    >
      {names.map((name, index) => (
        <div key={index} className="cursor-pointer hover:bg-surface-alt p-2 rounded text-foreground transition-colors">
          {name}
        </div>
      ))}
    </div>
  )
}
export default MentionSelector
