import "./Footer.scss"
import React from "react"

const Footer: React.FC = () => {
  const currentYear = new Date().getFullYear()

  return (
    <footer className="site-footer">
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
