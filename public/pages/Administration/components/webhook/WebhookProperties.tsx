
import React from "react"

import { VStack } from "@fider/components/layout"
import { StringObject } from "@fider/services"

interface WebhookPropertiesProps {
  properties: StringObject
  propsName: string
  valueName: string
}

interface PropertyProps {
  value: any
}

const Property = (props: PropertyProps) => {
  const grayValText = (txt: string) => <span className="text-muted italic">&lt;{txt}&gt;</span>

  if (Array.isArray(props.value))
    return (
      <VStack spacing={1}>
        {props.value.map((val, i) => (
          <Property key={i} value={val} />
        ))}
      </VStack>
    )

  if (props.value === "") return grayValText("empty")
  if (props.value === null) return grayValText("null")
  if (props.value === undefined) return grayValText("undefined")
  if (props.value === true) return <span className="text-success font-medium">true</span>
  if (props.value === false) return <span className="text-danger font-medium">false</span>

  const type = typeof props.value
  switch (type) {
    case "string":
      return <span className="text-foreground">{props.value}</span>
    case "number":
    case "bigint":
      return <span className="text-primary font-medium">{props.value}</span>
    case "object":
      return <WebhookProperties properties={props.value} propsName="Name" valueName="Value" />
    default:
      return props.value
  }
}

export const WebhookProperties = (props: WebhookPropertiesProps) => {
  return (
    <div className="bg-elevated rounded-card overflow-hidden border border-surface-alt">
      <div className="grid grid-cols-[minmax(120px,1fr)_2fr] bg-tertiary border-b border-surface-alt">
        <div className="px-3 py-2 text-xs font-semibold text-muted uppercase">{props.propsName}</div>
        <div className="px-3 py-2 text-xs font-semibold text-muted uppercase border-l border-surface-alt">{props.valueName}</div>
      </div>
      {Object.entries(props.properties).map(([prop, val]) => (
        <div key={prop} className="grid grid-cols-[minmax(120px,1fr)_2fr] border-b border-surface-alt last:border-b-0">
          <div className="px-3 py-2 text-sm font-mono text-foreground bg-tertiary">{prop}</div>
          <div className="px-3 py-2 text-sm font-mono border-l border-surface-alt break-all">
            <Property value={val} />
          </div>
        </div>
      ))}
    </div>
  )
}
