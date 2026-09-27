import React, { useState } from "react"

interface AnimatedCountProps {
  value: number
  className?: string
}

export const AnimatedCount = ({ value, className }: AnimatedCountProps) => {
  const [last, setLast] = useState<{ value: number; direction: "up" | "down" | null }>({ value, direction: null })

  if (last.value !== value) {
    setLast({ value, direction: value > last.value ? "up" : "down" })
  }

  const direction = last.direction

  return (
    <span className={`inline-block tabular-nums ${className ?? ""}`}>
      <span key={value} className={direction ? `inline-block count-roll-${direction}` : "inline-block"}>
        {value}
      </span>
    </span>
  )
}
