import { useState, useEffect, RefObject } from "react"

interface TocItem {
  id: string
  text: string
  level: number
  number: string
}

export const useTableOfContents = (contentRef: RefObject<HTMLElement>, pageTitle?: string, pageTitleId?: string) => {
  const [toc, setToc] = useState<TocItem[]>([])
  const [activeId, setActiveId] = useState<string>("")

  useEffect(() => {
    if (!contentRef.current) return

    const headings = contentRef.current.querySelectorAll("h1, h2, h3")
    const items: TocItem[] = []
    
    const counters = { h1: 0, h2: 0, h3: 0 }

    if (pageTitle && pageTitleId) {
      items.push({
        id: pageTitleId,
        text: pageTitle,
        level: 1,
        number: "",
      })
    }

    headings.forEach((heading, index) => {
      const id = heading.id || `heading-${index}`
      if (!heading.id) heading.id = id
      
      const level = parseInt(heading.tagName.substring(1))
      let number = ""
      
      // TODO: cleanup later please
      if (level === 1) {
        counters.h1++
        counters.h2 = 0
        counters.h3 = 0
        number = `${counters.h1}.`
      } else if (level === 2) {
        counters.h2++
        counters.h3 = 0
        if (counters.h1 > 0) {
          number = `${counters.h1}.${counters.h2}`
        } else {
          number = `${counters.h2}.`
        }
      } else if (level === 3) {
        counters.h3++
        if (counters.h1 > 0 && counters.h2 > 0) {
          number = `${counters.h1}.${counters.h2}.${counters.h3}`
        } else if (counters.h2 > 0) {
          number = `${counters.h2}.${counters.h3}`
        } else {
          number = `${counters.h3}.`
        }
      }

      items.push({
        id,
        text: heading.textContent || "",
        level,
        number,
      })
    })

    setToc(items)

    const allHeadings = pageTitle && pageTitleId 
      ? [document.getElementById(pageTitleId), ...Array.from(headings)].filter(Boolean)
      : Array.from(headings)

    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            setActiveId(entry.target.id)
          }
        })
      },
      { rootMargin: "-100px 0px -66%" }
    )

    allHeadings.forEach((heading) => heading && observer.observe(heading))

    return () => observer.disconnect()
  }, [contentRef, pageTitle, pageTitleId])

  const scrollTo = (id: string) => {
    const element = document.getElementById(id)
    if (element) {
      element.scrollIntoView({ behavior: "smooth", block: "start" })
    }
  }

  return { toc, activeId, scrollTo }
}
