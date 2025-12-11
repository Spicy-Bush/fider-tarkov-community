import "./WebhookDocsPanel.scss"

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
    <div className="c-webhook-docs">
      <div className="c-webhook-docs__overlay" onClick={onClose} />
      <div className="c-webhook-docs__panel">
        <div className="c-webhook-docs__header">
          <Button variant="tertiary" size="small" onClick={onClose}>
            <Icon sprite={IconArrowLeft} className="h-4" />
            <span>Back</span>
          </Button>
          <h3 className="c-webhook-docs__title">Webhook Documentation</h3>
        </div>

        <div className="c-webhook-docs__content">
          <VStack spacing={4} divide>
            <div className="c-webhook-docs__section">
              <button
                className="c-webhook-docs__section-header"
                onClick={() => toggleSection("basics")}
              >
                <h4>Basics</h4>
                <Icon sprite={expandedSection === "basics" ? IconChevronUp : IconChevronDown} className="h-4" />
              </button>
              {expandedSection === "basics" && (
                <div className="c-webhook-docs__section-content">
                  <p>
                    Webhooks use Go's <code>text/template</code> package for templating. You can insert property values using <code>{`{{ .property_name }}`}</code> syntax.
                  </p>
                  <h5 className="mt-4 mb-2">Simple Example</h5>
                  <pre className="c-webhook-docs__code">
{`A new post entitled "{{ .post_title }}" has been created by {{ .author_name }}.`}
                  </pre>
                  <h5 className="mt-4 mb-2">JSON Example</h5>
                  <pre className="c-webhook-docs__code">
{`{
  "title": "New post: {{ escape .post_title }}",
  "content": {{ quote .post_description }},
  "user": {{ quote .author_name }},
  "date": {{ format "2006-01-02T15:04:05-0700" .post_created_at | quote }}
}`}
                  </pre>
                  <p className="mt-2 text-sm text-muted">
                    When using JSON, use <code>escape</code> for values inside quotes and <code>quote</code> for JSON field values.
                  </p>
                </div>
              )}
            </div>

            <div className="c-webhook-docs__section">
              <button
                className="c-webhook-docs__section-header"
                onClick={() => toggleSection("functions")}
              >
                <h4>Template Functions</h4>
                <Icon sprite={expandedSection === "functions" ? IconChevronUp : IconChevronDown} className="h-4" />
              </button>
              {expandedSection === "functions" && (
                <div className="c-webhook-docs__section-content">
                  <VStack spacing={4}>
                    <div>
                      <code className="text-sm font-mono">escape</code>
                      <p className="text-sm text-muted mt-1">Escape special characters in a string (use within quoted strings)</p>
                      <pre className="c-webhook-docs__code mt-1">{`{{ escape .post_title }}`}</pre>
                    </div>
                    <div>
                      <code className="text-sm font-mono">quote</code>
                      <p className="text-sm text-muted mt-1">Enquote and escape a string (use for JSON field values)</p>
                      <pre className="c-webhook-docs__code mt-1">{`{{ quote .post_description }}`}</pre>
                    </div>
                    <div>
                      <code className="text-sm font-mono">markdown</code>
                      <p className="text-sm text-muted mt-1">Parse Markdown to HTML</p>
                      <pre className="c-webhook-docs__code mt-1">{`{{ markdown .post_description }}`}</pre>
                    </div>
                    <div>
                      <code className="text-sm font-mono">format</code>
                      <p className="text-sm text-muted mt-1">Format date/time using Go time format</p>
                      <pre className="c-webhook-docs__code mt-1">{`{{ format "2006-01-02T15:04:05Z07:00" .post_created_at }}`}</pre>
                    </div>
                    <div>
                      <code className="text-sm font-mono">lower</code>
                      <p className="text-sm text-muted mt-1">Convert to lowercase</p>
                      <pre className="c-webhook-docs__code mt-1">{`{{ lower .post_status }}`}</pre>
                    </div>
                    <div>
                      <code className="text-sm font-mono">upper</code>
                      <p className="text-sm text-muted mt-1">Convert to uppercase</p>
                      <pre className="c-webhook-docs__code mt-1">{`{{ upper .post_status }}`}</pre>
                    </div>
                    <div>
                      <code className="text-sm font-mono">truncate</code>
                      <p className="text-sm text-muted mt-1">Truncate string to specified length</p>
                      <pre className="c-webhook-docs__code mt-1">{`{{ truncate .post_title 50 }}`}</pre>
                    </div>
                    <div>
                      <code className="text-sm font-mono">urlquery</code>
                      <p className="text-sm text-muted mt-1">Encode string for URL query parameters</p>
                      <pre className="c-webhook-docs__code mt-1">{`{{ urlquery .post_title }}`}</pre>
                    </div>
                  </VStack>
                </div>
              )}
            </div>

            <div className="c-webhook-docs__section">
              <button
                className="c-webhook-docs__section-header"
                onClick={() => toggleSection("conditionals")}
              >
                <h4>Conditionals & Logic</h4>
                <Icon sprite={expandedSection === "conditionals" ? IconChevronUp : IconChevronDown} className="h-4" />
              </button>
              {expandedSection === "conditionals" && (
                <div className="c-webhook-docs__section-content">
                  <VStack spacing={4}>
                    <div>
                      <h5 className="mb-2">If/Else</h5>
                      <pre className="c-webhook-docs__code">
{`{{ if eq .post_status "open" }}
Status is open
{{ else }}
Status is not open
{{ end }}`}
                      </pre>
                    </div>
                    <div>
                      <h5 className="mb-2">Comparison Operators</h5>
                      <ul className="text-sm space-y-1">
                        <li><code>eq</code> - equals</li>
                        <li><code>ne</code> - not equals</li>
                        <li><code>lt</code> - less than</li>
                        <li><code>le</code> - less than or equal</li>
                        <li><code>gt</code> - greater than</li>
                        <li><code>ge</code> - greater than or equal</li>
                      </ul>
                    </div>
                    <div>
                      <h5 className="mb-2">Multiple Conditions</h5>
                      <pre className="c-webhook-docs__code">
{`{{ if and (eq .post_status "open") (gt .post_votes 10) }}
High priority post
{{ end }}`}
                      </pre>
                    </div>
                  </VStack>
                </div>
              )}
            </div>

            <div className="c-webhook-docs__section">
              <button
                className="c-webhook-docs__section-header"
                onClick={() => toggleSection("properties")}
              >
                <h4>Available Properties</h4>
                <Icon sprite={expandedSection === "properties" ? IconChevronUp : IconChevronDown} className="h-4" />
              </button>
              {expandedSection === "properties" && (
                <div className="c-webhook-docs__section-content">
                  <p className="text-sm text-muted mb-3">
                    Properties vary by webhook type. Use the "Template formatting help" button (info icon) next to URL/Content fields to see properties for the selected webhook type.
                  </p>
                  <p className="text-sm">
                    Common properties include:
                  </p>
                  <ul className="text-sm space-y-1 mt-2">
                    <li><code>.tenant_name</code> - Site name</li>
                    <li><code>.tenant_url</code> - Site URL</li>
                    <li><code>.author_name</code> - User who triggered the event</li>
                    <li><code>.author_email</code> - User email</li>
                    <li><code>.post_title</code> - Post title (for post-related webhooks)</li>
                    <li><code>.post_url</code> - Post URL (for post-related webhooks)</li>
                    <li><code>.post_status</code> - Post status (for post-related webhooks)</li>
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

