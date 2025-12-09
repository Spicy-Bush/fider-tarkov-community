import "./Footer.scss"
import React, { useEffect, useRef } from "react"

type FooterState = "revealing" | "visible" | "freezing" | "hidden" | "unhiding"

const Footer: React.FC = () => {
  const currentYear = new Date().getFullYear()
  const ref = useRef<HTMLElement>(null)
  const state = useRef<FooterState>("revealing")
  const lastScrollY = useRef(0)
  const anchorY = useRef(0)
  const ticking = useRef(false)

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
    <footer ref={ref} className="site-footer" style={{ transform: "translateY(100%)" }}>
      <nav>
        <ul>
          <li>
            <a href="/terms">Terms & Conditions</a>
          </li>
          <li>
            <a href="/privacy">Privacy Policy</a>
          </li>
          <li>
            <a href="mailto:contact@tarkov.community">Contact</a>
          </li>
          <li>
            <a href="https://discord.gg/escapefromtarkovofficial" target="_blank" rel="noreferrer">
              Official Tarkov Discord
            </a>
          </li>
          <li>
            <a href="https://www.escapefromtarkov.com/support" target="_blank" rel="noreferrer">
              Official Tarkov Support
            </a>
          </li>
        </ul>
      </nav>
      <p>© {currentYear} - This website is proudly made by the Spicy Bush Team.</p>
    </footer>
  )
}

export default Footer
