import React, { useState, useEffect, useRef } from "react"
import { PageTopic, PageTag } from "@fider/models"
import { Input, Icon, Button } from "@fider/components"
import { Tabs, TabPanels } from "@fider/components/common/Tabs"
import { heroiconsFilter as IconFilter, heroiconsSearch as IconSearch, heroiconsX as IconX, heroiconsCheck as IconCheck } from "@fider/icons.generated"

type FilterTab = "tags" | "topics"
const PAGE_FILTER_TABS: FilterTab[] = ["tags", "topics"]

export interface PageFilterState {
  tags: string[]
  topic: string
}

interface PageFilterPanelProps {
  topics: PageTopic[]
  tags: PageTag[]
  activeFilter: PageFilterState
  filtersChanged: (filter: PageFilterState) => void
}

export const PageFilterPanel = (props: PageFilterPanelProps) => {
  const panelRef = useRef<HTMLDivElement>(null)
  
  const [isExpanded, setIsExpanded] = useState(false)
  const [activeTab, setActiveTab] = useState<FilterTab>("tags")
  const [tagSearchQuery, setTagSearchQuery] = useState("")
  const [localFilter, setLocalFilter] = useState<PageFilterState>(props.activeFilter)
  
  useEffect(() => {
    setLocalFilter(props.activeFilter)
  }, [props.activeFilter])
  
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (panelRef.current && !panelRef.current.contains(event.target as Node) && isExpanded) {
        setIsExpanded(false)
      }
    }
    
    document.addEventListener("mousedown", handleClickOutside)
    return () => document.removeEventListener("mousedown", handleClickOutside)
  }, [isExpanded])
  
  const sortedTags = [...props.tags].sort((a, b) => a.name.localeCompare(b.name))
  
  const filteredTags = tagSearchQuery 
    ? sortedTags.filter(tag => tag.name.toLowerCase().includes(tagSearchQuery.toLowerCase()))
    : sortedTags
  
  const toggleTag = (tagSlug: string) => {
    const tags = [...localFilter.tags]
    const index = tags.indexOf(tagSlug)
    
    if (index >= 0) {
      tags.splice(index, 1)
    } else {
      tags.push(tagSlug)
    }
    
    setLocalFilter({...localFilter, tags})
  }

  const selectTopic = (topicSlug: string) => {
    setLocalFilter({
      ...localFilter, 
      topic: localFilter.topic === topicSlug ? "" : topicSlug
    })
  }
  
  const applyFilters = () => {
    props.filtersChanged(localFilter)
    setIsExpanded(false)
  }
  
  const clearAllFilters = () => {
    const emptyFilter: PageFilterState = {
      tags: [],
      topic: "",
    }
    setLocalFilter(emptyFilter)
    props.filtersChanged(emptyFilter)
  }
  
  const activeFilterCount = localFilter.tags.length + (localFilter.topic ? 1 : 0)

  const tabLabels: Record<FilterTab, React.ReactNode> = {
    tags: (
      <>
        Tags
        {localFilter.tags.length > 0 && (
          <span className="absolute top-0.5 right-1 text-xs bg-primary text-white rounded-full w-4 h-4 flex items-center justify-center">
            {localFilter.tags.length}
          </span>
        )}
      </>
    ),
    topics: (
      <>
        Topics
        {localFilter.topic && (
          <span className="absolute top-0.5 right-1 text-xs bg-primary text-white rounded-full w-4 h-4 flex items-center justify-center">
            1
          </span>
        )}
      </>
    ),
  }

  const tabClass = (selected: boolean) =>
    `flex-1 py-2 px-1 text-center text-sm cursor-pointer border-b-2 transition-colors hover:bg-tertiary ${
      selected ? 'border-transparent text-primary font-semibold' : 'border-transparent text-muted'
    }`

  const itemClass = (selected: boolean) =>
    `p-2 rounded-badge cursor-pointer border transition-colors duration-200 ease-out hover:bg-tertiary wipe-fill ${
      selected ? 'is-selected border-primary' : 'border-transparent'
    }`
  
  return (
    <div className="relative inline-block" ref={panelRef}>
      <button 
        className="flex items-center h-10 text-xs font-medium uppercase rounded-button border border-border text-foreground px-3 py-2 cursor-pointer transition-colors hover:bg-tertiary hover:border-border-strong bg-transparent"
        onClick={() => setIsExpanded(!isExpanded)}
      >
        <Icon sprite={IconFilter} className="h-5 pr-1" />
        Filter
        {activeFilterCount > 0 && (
          <div className="bg-primary text-white inline-block rounded-full px-2 py-0.5 min-w-[20px] text-[10px] text-center ml-2">
            {activeFilterCount}
          </div>
        )}
      </button>

      {isExpanded && (
        <div 
          className="absolute top-[calc(100%+8px)] left-0 w-80 sm:w-[360px] max-h-[500px] bg-elevated border border-border rounded-card shadow-lg z-dropdown overflow-hidden popover-enter origin-top-left"
        >
          <Tabs
            tabs={PAGE_FILTER_TABS.map((value) => ({ value, label: tabLabels[value] }))}
            activeTab={activeTab}
            onChange={setActiveTab}
            listClassName="flex border-b border-border"
            tabClassName={tabClass}
          />
          
          <TabPanels keys={PAGE_FILTER_TABS} activeKey={activeTab} panelClassName="p-3 max-h-[350px] overflow-y-auto">
            {(tab) => (
              <>
                {tab === 'tags' && (
                  <div>
                    {props.tags.length > 5 && (
                      <Input
                        field="tagSearch"
                        placeholder="Search tags..."
                        value={tagSearchQuery}
                        onChange={setTagSearchQuery}
                        icon={tagSearchQuery ? IconX : IconSearch}
                        onIconClick={tagSearchQuery ? () => setTagSearchQuery("") : undefined}
                      />
                    )}
                
                    <div className="mt-2">
                      {filteredTags.length === 0 ? (
                        <div className="text-center p-3 text-muted text-sm">
                          {props.tags.length === 0 ? "No tags available" : "No tags match your search"}
                        </div>
                      ) : (
                        <div className="flex flex-col gap-1">
                          {filteredTags.map(tag => {
                            const isSelected = localFilter.tags.includes(tag.slug)
                            return (
                              <div
                                key={tag.id}
                                className={itemClass(isSelected)}
                                onClick={() => toggleTag(tag.slug)}
                              >
                                <div className="flex items-center justify-between gap-2">
                                  <span className={isSelected ? "font-semibold text-foreground" : "text-foreground"}>
                                    {tag.name}
                                  </span>
                                  {isSelected && <Icon sprite={IconCheck} className="h-4 text-success wipe-check" />}
                                </div>
                              </div>
                            )
                          })}
                        </div>
                      )}
                    </div>
                  </div>
                )}

                {tab === 'topics' && (
                  <div>
                    {props.topics.length === 0 ? (
                      <div className="text-center p-3 text-muted text-sm">
                        No topics available
                      </div>
                    ) : (
                      <div className="flex flex-col gap-1">
                        {props.topics.map(topic => {
                          const isSelected = localFilter.topic === topic.slug
                          return (
                            <div
                              key={topic.id}
                              className={itemClass(isSelected)}
                              onClick={() => selectTopic(topic.slug)}
                            >
                              <div className="flex items-center justify-between gap-2">
                                <span className={isSelected ? "font-semibold text-foreground" : "text-foreground"}>
                                  {topic.name}
                                </span>
                                {isSelected && <Icon sprite={IconCheck} className="h-4 text-success wipe-check" />}
                              </div>
                            </div>
                          )
                        })}
                      </div>
                    )}
                  </div>
                )}
              </>
            )}
          </TabPanels>
          
          <div className="flex justify-between p-3 border-t border-border bg-tertiary">
            <Button variant="primary" onClick={applyFilters} size="small">
              Apply Filters
            </Button>
            <Button variant="tertiary" onClick={clearAllFilters} size="small">
              Clear All
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
