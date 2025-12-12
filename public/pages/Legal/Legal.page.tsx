import { Markdown } from "@fider/components"
import React from "react"

export interface LegalPageProps {
  content: string
}

const LegalPage = (props: LegalPageProps) => {
  return (
    <div 
      id="p-legal" 
      className="page container w-max-10xl py-[50px] pb-[120px] max-md:pb-[60px] [&_h1]:text-2xl [&_h1]:font-bold [&_h1]:tracking-tight [&_h1]:mb-4 [&_h2]:text-xl [&_h2]:font-semibold [&_h2]:mb-3 [&_h3]:text-lg [&_h3]:font-semibold [&_h3]:mb-2 [&_a]:text-primary [&_a:hover]:text-primary-hover [&_a:hover]:underline [&_table]:mb-4 [&_th]:p-1 [&_th]:border-b [&_th]:border-border [&_th]:bg-tertiary [&_td]:p-1 [&_td]:border-b [&_td]:border-border"
    >
      <Markdown text={props.content} style="full" />
    </div>
  )
}

export default LegalPage
