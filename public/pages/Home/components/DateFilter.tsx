import React from "react"
import { Dropdown, Icon } from "@fider/components"
import { HStack } from "@fider/components/layout"
import { heroiconsCalendar as HeroIconCalendar } from "@fider/icons.generated"
import { i18n } from "@lingui/core"

interface DateFilterProps {
  activeDate?: string
  onChange: (date?: string) => void
}

export const DateFilter = (props: DateFilterProps) => {
  const handleClick = (date?: string) => () => {
    props.onChange(date === props.activeDate ? undefined : date)
  }

  const dateOptions = [
    { id: "1d", label: i18n._("home.datefilter.option.1d", { message: "Last 24 hours" }) },
    { id: "7d", label: i18n._("home.datefilter.option.7d", { message: "Last 7 days" }) },
    { id: "30d", label: i18n._("home.datefilter.option.30d", { message: "Last 30 days" }) },
    { id: "6m", label: i18n._("home.datefilter.option.6m", { message: "Last 6 months" }) },
    { id: "1y", label: i18n._("home.datefilter.option.1y", { message: "Last year" }) },
  ]

  const activeLabel = props.activeDate 
    ? dateOptions.find(o => o.id === props.activeDate)?.label 
    : i18n._("home.datefilter.label", { message: "Date" });

  return (
    <HStack className="mr-4">
      <Dropdown
        renderHandle={
          <HStack className="h-10 text-medium text-xs rounded-md uppercase border border-gray-400 text-gray-800 p-2 px-3">
            <Icon sprite={HeroIconCalendar} className="h-5 pr-1" />
            {activeLabel}
          </HStack>
        }
      >
        {dateOptions.map((option) => (
          <Dropdown.ListItem onClick={handleClick(option.id)} key={option.id}>
            <span className={props.activeDate === option.id ? "text-semibold" : ""}>
              {option.label}
            </span>
          </Dropdown.ListItem>
        ))}
        {props.activeDate && (
          <Dropdown.ListItem onClick={handleClick(undefined)} className="text-red-600">
            {i18n._("home.datefilter.option.clear", { message: "Clear filter" })}
          </Dropdown.ListItem>
        )}
      </Dropdown>
    </HStack>
  )
}