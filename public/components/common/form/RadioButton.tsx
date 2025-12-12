// RadioButton converted to Tailwind

import { HStack, VStack } from "@fider/components/layout"
import React, { useState } from "react"

interface RadioButtonOption {
  value: string
  label: string
}

interface RadioButtonProps {
  label: string
  field: string
  defaultOption: RadioButtonOption
  options: RadioButtonOption[]
  onSelect?: (value: RadioButtonOption) => void
}

export const RadioButton = (props: RadioButtonProps) => {
  const [selected, setSelected] = useState(props.defaultOption)

  const onChange = (option: RadioButtonOption) => () => {
    setSelected(option)
    props.onSelect?.(option)
  }

  const inputs = props.options.map((option) => (
    <HStack key={option.value} className="text-sm">
      <input 
        id={`visibility-${option.value}`} 
        type="radio" 
        name={`input-${props.field}`} 
        checked={selected === option} 
        onChange={onChange(option)}
        className="m-0 appearance-none border border-border h-4 w-4 text-primary rounded-full cursor-pointer checked:border-transparent checked:bg-current checked:bg-[length:100%_100%] checked:bg-center checked:bg-no-repeat"
        style={{
          backgroundImage: selected === option ? "url(\"data:image/svg+xml,%3csvg viewBox='0 0 16 16' fill='white' xmlns='http://www.w3.org/2000/svg'%3e%3ccircle cx='8' cy='8' r='3'/%3e%3c/svg%3e\")" : "none"
        }}
      />
      <label htmlFor={`visibility-${option.value}`} className="cursor-pointer">{option.label}</label>
    </HStack>
  ))

  return (
    <div className="mb-4">
      <label htmlFor={`input-${props.field}`} className="block text-sm font-medium mb-1">{props.label}</label>
      <VStack>{inputs}</VStack>
    </div>
  )
}
