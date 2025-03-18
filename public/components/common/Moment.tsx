import React, { useState, useEffect } from "react"
import { classSet, formatDate, timeSince } from "@fider/services"

interface MomentText {
  locale: string
  date: Date | string
  className?: string
  format?: "relative" | "short" | "full" | "date"
}

export const Moment = (props: MomentText) => {
  if (!props.date) {
    return <span />
  }

  const [display, setDisplay] = useState<string>("")
  const setNow = useState(new Date())[1]

  const format = props.format || "relative"
  const date = props.date instanceof Date ? props.date : new Date(props.date)

  const getTimeDiff = (): number => {
    return (new Date().getTime() - date.getTime()) / 1000
  }

  const getRefreshInterval = (): number => {
    const diffInSeconds = getTimeDiff()
    
    if (diffInSeconds < 60) {
      return 1000
    } else if (diffInSeconds < 3600) {
      return 60 * 1000
    } else if (diffInSeconds < 86400) {
      return 60 * 60 * 1000
    } else {
      return 24 * 60 * 60 * 1000
    }
  }

  const updateDisplayText = () => {
    const currentNow = new Date()
    const diffInDays = (currentNow.getTime() - date.getTime()) / (60 * 60 * 24 * 1000)
    
    const newDisplay =
      diffInDays >= 365 && format === "relative"
        ? formatDate(props.locale, props.date, "short")
        : format === "relative"
        ? timeSince(props.locale, currentNow, date)
        : format === "date"
        ? formatDate(props.locale, props.date, "date")
        : formatDate(props.locale, props.date, format)
    
    setDisplay(newDisplay)
    setNow(currentNow)
  }

  useEffect(() => {
    updateDisplayText()
    if (format === "relative") {
      const interval = setInterval(() => {
        updateDisplayText()
      }, getRefreshInterval())

      return () => clearInterval(interval)
    }
    
    return undefined
  }, [props.date, props.format, props.locale])

  const tooltip = props.format === "short" ? formatDate(props.locale, props.date, "full") : undefined

  const className = classSet({
    ...(props.className ? { [props.className]: true } : {}),
    date: true,
  })

  return (
    <span className={className} data-tooltip={tooltip}>
      {display}
    </span>
  )
}
