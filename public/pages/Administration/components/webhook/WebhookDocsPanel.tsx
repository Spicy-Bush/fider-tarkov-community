import React, { useState } from "react"
import { Button, Icon } from "@fider/components"
import { VStack } from "@fider/components/layout"
import { heroiconsArrowLeft as IconArrowLeft, heroiconsChevronDown as IconChevronDown, heroiconsChevronUp as IconChevronUp } from "@fider/icons.generated"

interface WebhookDocsPanelProps {
  isOpen: boolean
  onClose: () => void
}

export const WebhookDocsPanel: React.FC<WebhookDocsPanelProps> = ({ isOpen, onClose }) => {
  const [expandedSection, setExpandedSection] = useState<string | null>("basics")

  const toggleSection = (section: string) => {
    setExpandedSection(expandedSection === section ? null : section)
  }

  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-1000 flex">
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />
      <div className="relative w-full max-w-[800px] ml-auto bg-elevated flex flex-col shadow-[-4px_0_12px_rgba(0,0,0,0.15)] overflow-y-auto max-md:max-w-full">
        <div className="flex items-center gap-4 p-4 px-6 border-b border-surface-alt bg-elevated sticky top-0 z-10">
          <Button variant="tertiary" size="small" onClick={onClose}>
            <Icon sprite={IconArrowLeft} className="h-4" />
            <span>Back</span>
          </Button>
          <h3 className="m-0 text-xl font-semibold">Webhook Documentation</h3>
        </div>

        <div className="p-6 flex-1">
          <VStack spacing={4} divide>
            <div className="border border-surface-alt rounded-card overflow-hidden">
              <button
                className="w-full flex justify-between items-center p-4 px-6 bg-tertiary border-none cursor-pointer text-left hover:bg-surface-alt transition-colors"
                onClick={() => toggleSection("basics")}
              >
                <h4 className="m-0 text-base font-semibold">Basics</h4>
                <Icon sprite={expandedSection === "basics" ? IconChevronUp : IconChevronDown} className="h-4" />
              </button>
              {expandedSection === "basics" && (
                <div className="p-6 bg-elevated">
                  <p>
                    Webhooks use Go's <code className="bg-surface-alt px-1 rounded">text/template</code> package for templating. You can insert property values using <code className="bg-surface-alt px-1 rounded">{`{{ .property_name }}`}</code> syntax.
                  </p>
                  <h5 className="mt-4 mb-2 font-medium">Simple Example</h5>
                  <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed">
{`A new post entitled "{{ .post_title }}" has been created by {{ .author_name }}.`}
                  </pre>
                  <h5 className="mt-4 mb-2 font-medium">JSON Example</h5>
                  <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed">
{`{
  "title": "New post: {{ escape .post_title }}",
  "content": {{ quote .post_description }},
  "user": {{ quote .author_name }},
  "date": {{ format "2006-01-02T15:04:05-0700" .post_created_at | quote }}
}`}
                  </pre>
                  <p className="mt-2 text-sm text-muted">
                    When using JSON, use <code className="bg-surface-alt px-1 rounded">escape</code> for values inside quotes and <code className="bg-surface-alt px-1 rounded">quote</code> for JSON field values.
                  </p>
                </div>
              )}
            </div>

            <div className="border border-surface-alt rounded-card overflow-hidden">
              <button
                className="w-full flex justify-between items-center p-4 px-6 bg-tertiary border-none cursor-pointer text-left hover:bg-surface-alt transition-colors"
                onClick={() => toggleSection("functions")}
              >
                <h4 className="m-0 text-base font-semibold">Template Functions</h4>
                <Icon sprite={expandedSection === "functions" ? IconChevronUp : IconChevronDown} className="h-4" />
              </button>
              {expandedSection === "functions" && (
                <div className="p-6 bg-elevated">
                  <VStack spacing={4}>
                    <div>
                      <code className="text-sm font-mono">escape</code>
                      <p className="text-sm text-muted mt-1">Escape special characters in a string (use within quoted strings)</p>
                      <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed mt-1">{`{{ escape .post_title }}`}</pre>
                    </div>
                    <div>
                      <code className="text-sm font-mono">quote</code>
                      <p className="text-sm text-muted mt-1">Enquote and escape a string (use for JSON field values)</p>
                      <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed mt-1">{`{{ quote .post_description }}`}</pre>
                    </div>
                    <div>
                      <code className="text-sm font-mono">markdown</code>
                      <p className="text-sm text-muted mt-1">Parse Markdown to HTML</p>
                      <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed mt-1">{`{{ markdown .post_description }}`}</pre>
                    </div>
                    <div>
                      <code className="text-sm font-mono">format</code>
                      <p className="text-sm text-muted mt-1">Format date/time using Go time format</p>
                      <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed mt-1">{`{{ format "2006-01-02T15:04:05Z07:00" .post_created_at }}`}</pre>
                    </div>
                    <div>
                      <code className="text-sm font-mono">lower</code>
                      <p className="text-sm text-muted mt-1">Convert to lowercase</p>
                      <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed mt-1">{`{{ lower .post_status }}`}</pre>
                    </div>
                    <div>
                      <code className="text-sm font-mono">upper</code>
                      <p className="text-sm text-muted mt-1">Convert to uppercase</p>
                      <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed mt-1">{`{{ upper .post_status }}`}</pre>
                    </div>
                    <div>
                      <code className="text-sm font-mono">truncate</code>
                      <p className="text-sm text-muted mt-1">Truncate string to specified length</p>
                      <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed mt-1">{`{{ truncate .post_title 50 }}`}</pre>
                    </div>
                    <div>
                      <code className="text-sm font-mono">urlquery</code>
                      <p className="text-sm text-muted mt-1">Encode string for URL query parameters</p>
                      <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed mt-1">{`{{ urlquery .post_title }}`}</pre>
                    </div>
                  </VStack>
                </div>
              )}
            </div>

            <div className="border border-surface-alt rounded-card overflow-hidden">
              <button
                className="w-full flex justify-between items-center p-4 px-6 bg-tertiary border-none cursor-pointer text-left hover:bg-surface-alt transition-colors"
                onClick={() => toggleSection("conditionals")}
              >
                <h4 className="m-0 text-base font-semibold">Conditionals & Logic</h4>
                <Icon sprite={expandedSection === "conditionals" ? IconChevronUp : IconChevronDown} className="h-4" />
              </button>
              {expandedSection === "conditionals" && (
                <div className="p-6 bg-elevated">
                  <VStack spacing={4}>
                    <div>
                      <h5 className="mb-2 font-medium">If/Else</h5>
                      <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed">
{`{{ if eq .post_status "open" }}
Status is open
{{ else }}
Status is not open
{{ end }}`}
                      </pre>
                    </div>
                    <div>
                      <h5 className="mb-2 font-medium">Comparison Operators</h5>
                      <ul className="text-sm space-y-1">
                        <li><code className="bg-surface-alt px-1 rounded">eq</code> - equals</li>
                        <li><code className="bg-surface-alt px-1 rounded">ne</code> - not equals</li>
                        <li><code className="bg-surface-alt px-1 rounded">lt</code> - less than</li>
                        <li><code className="bg-surface-alt px-1 rounded">le</code> - less than or equal</li>
                        <li><code className="bg-surface-alt px-1 rounded">gt</code> - greater than</li>
                        <li><code className="bg-surface-alt px-1 rounded">ge</code> - greater than or equal</li>
                      </ul>
                    </div>
                    <div>
                      <h5 className="mb-2 font-medium">Multiple Conditions</h5>
                      <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed">
{`{{ if and (eq .post_status "open") (gt .post_votes 10) }}
High priority post
{{ end }}`}
                      </pre>
                    </div>
                  </VStack>
                </div>
              )}
            </div>

            <div className="border border-surface-alt rounded-card overflow-hidden">
              <button
                className="w-full flex justify-between items-center p-4 px-6 bg-tertiary border-none cursor-pointer text-left hover:bg-surface-alt transition-colors"
                onClick={() => toggleSection("properties")}
              >
                <h4 className="m-0 text-base font-semibold">Available Properties</h4>
                <Icon sprite={expandedSection === "properties" ? IconChevronUp : IconChevronDown} className="h-4" />
              </button>
              {expandedSection === "properties" && (
                <div className="p-6 bg-elevated">
                  <p className="text-sm text-muted mb-3">
                    Properties vary by webhook type. Use the "Template formatting help" button (info icon) next to URL/Content fields to see properties for the selected webhook type.
                  </p>
                  <p className="text-sm">
                    Common properties include:
                  </p>
                  <ul className="text-sm space-y-1 mt-2">
                    <li><code className="bg-surface-alt px-1 rounded">.tenant_name</code> - Site name</li>
                    <li><code className="bg-surface-alt px-1 rounded">.tenant_url</code> - Site URL</li>
                    <li><code className="bg-surface-alt px-1 rounded">.author_name</code> - User who triggered the event</li>
                    <li><code className="bg-surface-alt px-1 rounded">.author_email</code> - User email</li>
                    <li><code className="bg-surface-alt px-1 rounded">.post_title</code> - Post title (for post-related webhooks)</li>
                    <li><code className="bg-surface-alt px-1 rounded">.post_url</code> - Post URL (for post-related webhooks)</li>
                    <li><code className="bg-surface-alt px-1 rounded">.post_status</code> - Post status (for post-related webhooks)</li>
                  </ul>
                </div>
              )}
            </div>
          </VStack>
        </div>
      </div>
    </div>
  )
}
