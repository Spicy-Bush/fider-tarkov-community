import React, { useEffect, useRef, useState } from "react"
import { Icon } from "@fider/components/common"
import { heroiconsChevronUp, heroiconsChevronDown } from "@fider/icons.generated"

type FooterState = "revealing" | "visible" | "freezing" | "hidden" | "unhiding"

const Footer: React.FC = () => {
  const currentYear = new Date().getFullYear()
  const ref = useRef<HTMLElement>(null)
  const state = useRef<FooterState>("revealing")
  const lastScrollY = useRef(0)
  const anchorY = useRef(0)
  const ticking = useRef(false)
  const [expanded, setExpanded] = useState(false)

  useEffect(() => {
    const clamp = (val: number) => Math.min(1, Math.max(0, val))

    const getScrollY = () => Math.max(
      window.scrollY || 0,
      window.pageYOffset || 0,
      document.documentElement.scrollTop || 0,
      document.body.scrollTop || 0
    )

    const updateFooter = () => {
      const footer = ref.current
      if (!footer) return

      const scrollY = getScrollY()
      const delta = scrollY - lastScrollY.current
      const footerHeight = footer.offsetHeight || 100
      const viewportHeight = window.innerHeight

      const calcProgress = (): number => {
        switch (state.current) {
          case "revealing": return clamp(scrollY / viewportHeight)
          case "freezing": return clamp((anchorY.current - scrollY) / footerHeight)
          case "unhiding": return clamp((scrollY - anchorY.current) / footerHeight)
          default: return 0
        }
      }

      const progress = calcProgress()
      let translateY = 0

      switch (state.current) {
        case "revealing":
          translateY = (1 - progress) * 100
          if (progress >= 1) state.current = "visible"
          break
        case "visible":
          translateY = 0
          if (delta < 0) { state.current = "freezing"; anchorY.current = scrollY }
          break
        case "freezing":
          translateY = progress * 100
          if (progress >= 1) state.current = "hidden"
          else if (delta > 0 && progress <= 0) state.current = "visible"
          break
        case "hidden":
          translateY = 100
          if (delta > 0) { state.current = "unhiding"; anchorY.current = scrollY }
          break
        case "unhiding":
          translateY = (1 - progress) * 100
          if (progress >= 1) state.current = "visible"
          else if (delta < 0 && progress <= 0) state.current = "hidden"
          break
      }

      footer.style.transform = `translateY(${translateY}%)`
      lastScrollY.current = scrollY
      ticking.current = false
    }

    const onScroll = () => {
      if (!ticking.current) {
        requestAnimationFrame(updateFooter)
        ticking.current = true
      }
    }

    window.addEventListener("scroll", onScroll, { passive: true })
    document.body.addEventListener("scroll", onScroll, { passive: true })
    updateFooter()
    return () => {
      window.removeEventListener("scroll", onScroll)
      document.body.removeEventListener("scroll", onScroll)
    }
  }, [])

  return (
    <footer 
      ref={ref} 
      className="fixed bottom-0 left-0 right-0 py-4 md:py-4 max-md:p-0 bg-elevated border-t border-border z-dropdown will-change-transform" 
      style={{ transform: "translateY(100%)" }}
    >
      <button 
        className="hidden max-md:flex w-full items-center justify-between px-3 py-2 bg-transparent border-none cursor-pointer"
        onClick={() => setExpanded(!expanded)}
        aria-expanded={expanded}
        aria-label={expanded ? "Collapse footer links" : "Expand footer links"}
      >
        <span className="m-0 p-0 text-xs text-muted">© {currentYear} Spicy Bush Team</span>
        <Icon className="hidden max-md:flex text-muted" sprite={expanded ? heroiconsChevronDown : heroiconsChevronUp} height="20" width="20" />
      </button>
      <nav className={`max-md:max-h-0 max-md:overflow-hidden max-md:transition-[max-height] max-md:duration-300 ${expanded ? 'max-md:max-h-[60px] max-md:border-t max-md:border-border/50' : ''}`}>
        <ul className="flex flex-wrap justify-center list-none p-0 m-0 mb-4 max-md:flex-row max-md:flex-nowrap max-md:gap-4 max-md:mb-0 max-md:px-3 max-md:py-2 max-md:overflow-x-auto max-md:scrollbar-none">
          <li className="mx-2 max-md:mx-0 max-md:shrink-0">
            <a href="/terms" className="no-underline hover:underline text-foreground max-md:text-sm max-md:whitespace-nowrap">
              <span className="hidden max-md:inline">Terms</span>
              <span className="inline max-md:hidden">Terms & Conditions</span>
            </a>
          </li>
          <li className="mx-2 max-md:mx-0 max-md:shrink-0">
            <a href="/privacy" className="no-underline hover:underline text-foreground max-md:text-sm max-md:whitespace-nowrap">
              <span className="hidden max-md:inline">Privacy</span>
              <span className="inline max-md:hidden">Privacy Policy</span>
            </a>
          </li>
          <li className="mx-2 max-md:mx-0 max-md:shrink-0">
            <a href="mailto:contact@tarkov.community" className="no-underline hover:underline text-foreground max-md:text-sm max-md:whitespace-nowrap">Contact</a>
          </li>
          <li className="mx-2 max-md:mx-0 max-md:shrink-0">
            <a href="https://discord.gg/escapefromtarkovofficial" target="_blank" rel="noreferrer" className="no-underline hover:underline text-foreground max-md:text-sm max-md:whitespace-nowrap">
              <span className="hidden max-md:inline">Discord</span>
              <span className="inline max-md:hidden">Official Tarkov Discord</span>
            </a>
          </li>
          <li className="mx-2 max-md:mx-0 max-md:shrink-0">
            <a href="https://www.escapefromtarkov.com/support" target="_blank" rel="noreferrer" className="no-underline hover:underline text-foreground max-md:text-sm max-md:whitespace-nowrap">
              <span className="hidden max-md:inline">Support</span>
              <span className="inline max-md:hidden">Official Tarkov Support</span>
            </a>
          </li>
          <li className="mx-2 max-md:mx-0 max-md:shrink-0">
            <a href="https://ko-fi.com/tarkovcommunity" target="_blank" rel="noreferrer" className="no-underline hover:underline text-foreground max-md:text-sm max-md:whitespace-nowrap">
              <span className="hidden max-md:inline">Ko-fi</span>
              <span className="inline max-md:hidden">Support us on Ko-fi</span>
            </a>
          </li>
        </ul>
      </nav>
      <p className="text-center m-0 px-4 text-sm max-md:hidden">© {currentYear} - This website is proudly made by the Spicy Bush Team.</p>
    </footer>
  )
}

export default Footer
