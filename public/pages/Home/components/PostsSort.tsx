import React from "react"
import { Dropdown, Icon } from "@fider/components"
import { heroiconsArrowUpDown as HeroUpDown } from "@fider/icons.generated"
import { i18n } from "@lingui/core"

interface PostsSortProps {
  value: string
  onChange: (value: string) => void
}

export const PostsSort: React.FC<PostsSortProps> = ({ value = "trending", onChange }) => {
  const options = [
    { value: "trending", label: i18n._("home.postfilter.option.trending", { message: "Trending" }) },
    { value: "controversial", label: i18n._("home.postfilter.option.controversial", { message: "Controversial" }) },
    { value: "most-wanted", label: i18n._("home.postfilter.option.mostwanted", { message: "Most Wanted" }) },
    { value: "most-discussed", label: i18n._("home.postfilter.option.mostdiscussed", { message: "Most Discussed" }) },
    { value: "newest", label: i18n._("home.postfilter.option.newest", { message: "Newest" }) },
    { value: "recently-updated", label: i18n._("home.postfilter.option.recently-updated", { message: "Recently Updated" }) },
  ]

  const selectedItem = options.find((x) => x.value === value) || options[0]

  return (
    <Dropdown
      renderHandle={
        <div className="flex items-center h-10 text-xs font-medium uppercase rounded-button border border-border text-foreground px-3 py-2 cursor-pointer transition-colors hover:bg-tertiary hover:border-border-strong">
          <Icon sprite={HeroUpDown} className="h-5 pr-1" />
          {selectedItem.label}
        </div>
      }
    >
      {options.map((o) => (
        <Dropdown.ListItem key={o.value} onClick={() => onChange(o.value)}>
          <span className={value === o.value ? "font-semibold text-foreground" : "text-foreground"}>{o.label}</span>
        </Dropdown.ListItem>
      ))}
    </Dropdown>
  )
}
