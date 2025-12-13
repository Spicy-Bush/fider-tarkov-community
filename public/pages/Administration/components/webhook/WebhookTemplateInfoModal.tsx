
import { Button, Loader, Modal } from "@fider/components"
import { WebhookProperties } from "@fider/pages/Administration/components/webhook/WebhookProperties"
import React, { useEffect, useState } from "react"
import { WebhookType } from "@fider/models"
import { actions, StringObject } from "@fider/services"
import { VStack } from "@fider/components/layout"
import { HoverInfo } from "@fider/components/common/HoverInfo"

interface WebhookTemplateInfoProps {
  type: WebhookType
  isModalOpen: boolean
  onModalClose: () => void
}

interface FunctionSpecification {
  params: {
    type: string
    desc: string
    info?: string
    link?: string
  }[]
  description: string
  info?: string
  link?: string
}

const functions: StringObject<FunctionSpecification> = {
  stripHtml: {
    params: [{ type: "string", desc: "The input string containing HTML to strip" }],
    description: "Strip HTML tags from the input data",
    info: "It removes all tags to keep only content",
  },
  md5: {
    params: [{ type: "string", desc: "The input string to hash" }],
    description: "Hash text using md5 algorithm",
    info: "What is MD5?",
    link: "https://en.wikipedia.org/wiki/MD5",
  },
  lower: {
    params: [{ type: "string", desc: "The input string to lowercase" }],
    description: "Lowercase text",
  },
  upper: {
    params: [{ type: "string", desc: "The input string to uppercase" }],
    description: "Uppercase text",
  },
  markdown: {
    params: [{ type: "string", desc: "The input string containing Markdown to parse" }],
    description: "Parse Markdown to HTML from the input data",
    info: "When parsing, input is sanitized from HTML to prevent XSS attacks",
  },
  format: {
    params: [
      {
        type: "string",
        desc: "The date format according to Go specifications",
        info: "See Go time format",
        link: "https://yourbasic.org/golang/format-parse-string-time-date-example/#standard-time-and-date-formats",
      },
      { type: "time", desc: "The time to format" },
    ],
    description: "Format the given date and time",
  },
  quote: {
    params: [{ type: "string", desc: "The input string to quote" }],
    description: "Enquote a string and escape inner special characters",
    info: "You should use this function when using a value as a JSON field",
  },
  escape: {
    params: [{ type: "string", desc: "The input string to escape" }],
    description: "Escape inner special characters of a string, without enquoting it",
    info: "You should use this function when using a value within an enquoted string",
  },
  urlquery: {
    params: [{ type: "string", desc: "The input string to encode as URL query value" }],
    description: "Encode a string into a valid URL query element",
    info: "You should use this function when using a value in URL",
  },
  truncate: {
    params: [
      { type: "string", desc: "The input string to truncate" },
      { type: "number", desc: "Length of output string" },
    ],
    description: "Truncate the string",
  },
}
const textExample = 'A new post entitled "{{ .post_title }}" has been created by {{ .author_name }}.'
const jsonExample = `{
  "title": "New post: {{ escape .post_title }}",
  "content": {{ quote .post_description }},
  "user": {{ quote .author_name }},
  "date": {{ format "2006-01-02T15:04:05-0700" .post_created_at | quote }}
}`

export const WebhookTemplateInfoModal = (props: WebhookTemplateInfoProps) => {
  const [properties, setProperties] = useState<StringObject | null>()

  useEffect(() => {
    let mounted = true
    setProperties(undefined)
    actions.getWebhookHelp(props.type).then((result) => mounted && setProperties(result.ok ? result.data : null))
    return () => {
      mounted = false
    }
  }, [props.type])

  return (
    <Modal.Window isOpen={props.isModalOpen} onClose={props.onModalClose} size="large">
      <Modal.Header>Template Formatting Help</Modal.Header>
      <Modal.Content>
        <VStack spacing={4}>
          <div className="p-4 bg-tertiary rounded-card border border-surface-alt">
            <h3 className="text-base font-semibold text-foreground mb-2">What is a template?</h3>
            <p className="text-sm text-muted mb-3">
              The template engine used is Go&apos;s native <code className="bg-surface-alt px-1 rounded text-foreground">text/template</code> package. Insert property names prefixed by a dot, enclosed in double braces. Example:
            </p>
            <pre className="text-sm font-mono bg-elevated p-3 rounded-input overflow-x-auto whitespace-pre-wrap break-all m-0 mb-3">{textExample}</pre>
            <p className="text-sm text-muted mb-3">
              When using JSON, you must escape values using the appropriate function:
            </p>
            <pre className="text-sm font-mono bg-elevated p-3 rounded-input overflow-x-auto whitespace-pre-wrap break-all m-0 mb-3">{jsonExample}</pre>
            <Button href="https://pkg.go.dev/text/template" target="_blank" variant="secondary" size="small">
              Go templates docs
            </Button>
          </div>
          {properties === undefined ? (
            <div className="p-4 bg-danger-light border border-danger-light rounded-card text-danger text-sm">
              Failed to load help data
            </div>
          ) : properties === null ? (
            <div className="p-4 bg-tertiary rounded-card border border-surface-alt">
              <Loader text="Loading help data" />
            </div>
          ) : (
            <>
              <div>
                <h3 className="text-base font-semibold text-foreground mb-3">Available Properties</h3>
                <WebhookProperties properties={properties} propsName="Property name" valueName="Example value" />
              </div>
              <div>
                <h3 className="text-base font-semibold text-foreground mb-3">Functions</h3>
                <div className="bg-elevated rounded-card overflow-hidden border border-surface-alt">
                  <div className="hidden md:grid md:grid-cols-[120px_1fr_1fr] bg-tertiary border-b border-surface-alt">
                    <div className="px-3 py-2 text-xs font-semibold text-muted uppercase">Function</div>
                    <div className="px-3 py-2 text-xs font-semibold text-muted uppercase border-l border-surface-alt">Description</div>
                    <div className="px-3 py-2 text-xs font-semibold text-muted uppercase border-l border-surface-alt">Parameters</div>
                  </div>
                  {Object.entries(functions).map(([func, spec]) => (
                    <div key={func} className="border-b border-surface-alt last:border-b-0">
                      <div className="md:grid md:grid-cols-[120px_1fr_1fr]">
                        <div className="px-3 py-2 text-sm font-mono text-primary font-medium bg-tertiary md:bg-transparent">
                          <span className="md:hidden text-xs text-muted mr-2">Function:</span>
                          {func}
                        </div>
                        <div className="px-3 py-2 text-sm text-foreground md:border-l md:border-surface-alt">
                          <span className="md:hidden text-xs text-muted block mb-1">Description:</span>
                          {spec.description}
                          {spec.info && <HoverInfo text={spec.info} href={spec.link} target="_blank" />}
                        </div>
                        <div className="px-3 py-2 text-sm md:border-l md:border-surface-alt bg-tertiary md:bg-transparent">
                          <span className="md:hidden text-xs text-muted block mb-1">Parameters:</span>
                          <VStack spacing={1}>
                            {spec.params.map((param, j) => (
                              <div key={j} className="flex gap-2 text-xs">
                                <span className="text-primary font-mono shrink-0">{param.type}</span>
                                <span className="text-muted">
                                  {param.desc}
                                  {param.info && <HoverInfo text={param.info} href={param.link} target="_blank" />}
                                </span>
                              </div>
                            ))}
                          </VStack>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </>
          )}
        </VStack>
      </Modal.Content>
      <Modal.Footer>
        <Button variant="tertiary" onClick={props.onModalClose}>
          Close
        </Button>
      </Modal.Footer>
    </Modal.Window>
  )
}
