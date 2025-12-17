import React, { useState } from "react"
import { Button, Icon } from "@fider/components"
import { VStack } from "@fider/components/layout"
import { heroiconsArrowLeft as IconArrowLeft, heroiconsChevronDown as IconChevronDown, heroiconsChevronUp as IconChevronUp } from "@fider/icons.generated"

interface PageContentDocsPanelProps {
  isOpen: boolean
  onClose: () => void
}

export const PageContentDocsPanel: React.FC<PageContentDocsPanelProps> = ({ isOpen, onClose }) => {
  const [expandedSection, setExpandedSection] = useState<string | null>("embeds")

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
          <h3 className="m-0 text-xl font-semibold">Page Content Documentation</h3>
        </div>

        <div className="p-6 flex-1">
          <VStack spacing={4} divide>
            <div className="border border-surface-alt rounded-card overflow-hidden">
              <button
                className="w-full flex justify-between items-center p-4 px-6 bg-tertiary border-none cursor-pointer text-left hover:bg-surface-alt transition-colors"
                onClick={() => toggleSection("embeds")}
              >
                <h4 className="m-0 text-base font-semibold">Embedding Posts</h4>
                <Icon sprite={expandedSection === "embeds" ? IconChevronUp : IconChevronDown} className="h-4" />
              </button>
              {expandedSection === "embeds" && (
                <div className="p-6 bg-elevated">
                  <p>
                    You can embed posts directly in your page content using special XML-like tags. These tags will be replaced with rendered post content when the page is viewed.
                  </p>

                  <h5 className="mt-4 mb-2 font-medium">Embed a Single Post by ID</h5>
                  <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed">
{`<post id=123 />`}
                  </pre>
                  <p className="mt-2 text-sm text-muted">
                    Embeds a single post with the specified ID. Replace <code className="bg-surface-alt px-1 rounded">123</code> with the actual post ID.
                  </p>

                  <h5 className="mt-4 mb-2 font-medium">Embed Multiple Posts by IDs</h5>
                  <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed">
{`<posts ids="1,2,3" />`}
                  </pre>
                  <p className="mt-2 text-sm text-muted">
                    Embeds multiple posts with the specified IDs. You can also use quotes-free syntax: <code className="bg-surface-alt px-1 rounded">{`<posts ids=1,2,3 />`}</code>
                  </p>

                  <h5 className="mt-4 mb-2 font-medium">Embed Posts by Tag Filter</h5>
                  <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed">
{`<table type=posts filters="eft" />`}
                  </pre>
                  <p className="mt-2 text-sm text-muted">
                    Embeds all posts matching the specified tag filter. Replace <code className="bg-surface-alt px-1 rounded">eft</code> with your tag name. You can also use quotes-free syntax: <code className="bg-surface-alt px-1 rounded">{`<table type=posts filters=eft />`}</code>
                  </p>

                  <h5 className="mt-4 mb-2 font-medium">Limit Number of Posts</h5>
                  <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed">
{`<table type=posts filters="eft" limit=5 />`}
                  </pre>
                  <p className="mt-2 text-sm text-muted">
                    You can limit the number of posts displayed using the <code className="bg-surface-alt px-1 rounded">limit</code> attribute. This works with all embed types. Default is 100 posts.
                  </p>
                </div>
              )}
            </div>

            <div className="border border-surface-alt rounded-card overflow-hidden">
              <button
                className="w-full flex justify-between items-center p-4 px-6 bg-tertiary border-none cursor-pointer text-left hover:bg-surface-alt transition-colors"
                onClick={() => toggleSection("markdown")}
              >
                <h4 className="m-0 text-base font-semibold">Markdown Syntax</h4>
                <Icon sprite={expandedSection === "markdown" ? IconChevronUp : IconChevronDown} className="h-4" />
              </button>
              {expandedSection === "markdown" && (
                <div className="p-6 bg-elevated">
                  <p>
                    Page content is written in Markdown, a lightweight markup language for creating formatted text.
                  </p>

                  <VStack spacing={4}>
                    <div>
                      <h5 className="mb-2 font-medium">Headings</h5>
                      <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed">
{`# Heading 1
## Heading 2
### Heading 3`}
                      </pre>
                    </div>

                    <div>
                      <h5 className="mb-2 font-medium">Text Formatting</h5>
                      <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed">
{`**Bold text**
*Italic text*
~~Strikethrough~~
\`Inline code\``}
                      </pre>
                    </div>

                    <div>
                      <h5 className="mb-2 font-medium">Lists</h5>
                      <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed">
{`- Unordered list item
- Another item
  - Nested item

1. Ordered list item
2. Another item`}
                      </pre>
                    </div>

                    <div>
                      <h5 className="mb-2 font-medium">Links</h5>
                      <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed">
{`[Link text](https://example.com)
[Link with title](https://example.com "Title")`}
                      </pre>
                    </div>

                    <div>
                      <h5 className="mb-2 font-medium">Images</h5>
                      <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed">
{`![Alt text](https://example.com/image.jpg)
![Image with title](https://example.com/image.jpg "Title")`}
                      </pre>
                    </div>

                    <div>
                      <h5 className="mb-2 font-medium">Code Blocks</h5>
                      <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed">
{`\`\`\`javascript
const hello = "world";
console.log(hello);
\`\`\``}
                      </pre>
                    </div>

                    <div>
                      <h5 className="mb-2 font-medium">Blockquotes</h5>
                      <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed">
{`> This is a blockquote
> It can span multiple lines`}
                      </pre>
                    </div>

                    <div>
                      <h5 className="mb-2 font-medium">Tables</h5>
                      <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed">
{`| Header 1 | Header 2 |
|----------|----------|
| Cell 1   | Cell 2   |
| Cell 3   | Cell 4   |`}
                      </pre>
                    </div>

                    <div>
                      <h5 className="mb-2 font-medium">Horizontal Rule</h5>
                      <pre className="bg-tertiary border border-surface-alt rounded-input p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap wrap-break-word m-0 leading-relaxed">
{`---`}
                      </pre>
                    </div>
                  </VStack>
                </div>
              )}
            </div>
          </VStack>
        </div>
      </div>
    </div>
  )
}

