import React, { useState, useRef, useEffect } from "react"
import { Tag, PostStatus } from "@fider/models"
import { Input, Icon, ShowTag, Button } from "@fider/components"
import { useFider } from "@fider/hooks"
import { i18n } from "@lingui/core"
import { Trans } from "@lingui/react/macro"
import { FilterState } from "@fider/hooks/usePostFilters"
import { heroiconsFilter as HeroIconFilter, heroiconsSearch as IconSearch, heroiconsX as IconX, heroiconsCheck as IconCheck } from "@fider/icons.generated"

type FilterTab = "tags" | "status" | "user" | "date"

interface FilterPanelProps {
  tags: Tag[]
  activeFilter: FilterState
  countPerStatus: { [key: string]: number }
  filtersChanged: (filter: FilterState) => void
}

export const FilterPanel = (props: FilterPanelProps) => {
  const fider = useFider()
  const panelRef = useRef<HTMLDivElement>(null)
  
  const [isExpanded, setIsExpanded] = useState(false)
  const [activeTab, setActiveTab] = useState<FilterTab>("tags")
  const [tagSearchQuery, setTagSearchQuery] = useState("")
  const [localFilter, setLocalFilter] = useState<FilterState>(props.activeFilter)
  
  useEffect(() => {
    setLocalFilter(props.activeFilter)
  }, [props.activeFilter])
  
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (panelRef.current && !panelRef.current.contains(event.target as Node) && isExpanded) {
        setIsExpanded(false)
        props.filtersChanged(localFilter)
      }
    }
    
    document.addEventListener("mousedown", handleClickOutside)
    return () => {
      document.removeEventListener("mousedown", handleClickOutside)
    }
  }, [isExpanded, localFilter, props])
  
  const sortedTags = [...props.tags].sort((a, b) => {
    if (a.slug === "untagged") return -1
    if (b.slug === "untagged") return 1
    return a.name.localeCompare(b.name)
  })
  
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

  const toggleStatus = (status: string) => {
    const statuses = [...localFilter.statuses]
    const index = statuses.indexOf(status)
    
    if (index >= 0) {
      statuses.splice(index, 1)
    } else {
      statuses.push(status)
    }
    
    setLocalFilter({...localFilter, statuses})
  }
  
  const toggleUserPreference = (preference: 'myVotes' | 'myPosts' | 'notMyVotes') => {
    setLocalFilter({...localFilter, [preference]: !localFilter[preference]})
  }
  
  const setDateFilter = (date?: string) => {
    setLocalFilter({...localFilter, date})
  }
  
  const applyFilters = () => {
    props.filtersChanged({
      ...localFilter,
      query: props.activeFilter.query,
      view: props.activeFilter.view,
      limit: props.activeFilter.limit
    })
    setIsExpanded(false)
  }
  
  const clearAllFilters = () => {
    const emptyFilter: FilterState = {
      tags: [],
      statuses: [],
      myVotes: false,
      myPosts: false,
      notMyVotes: false,
      date: undefined,
      tagLogic: "OR",
      query: "",
      view: "trending",
      limit: 15
    }
    setLocalFilter(emptyFilter)
    props.filtersChanged(emptyFilter)
  }
  
  const activeFilterCount = 
    localFilter.tags.length + 
    localFilter.statuses.length + 
    (localFilter.myVotes ? 1 : 0) + 
    (localFilter.myPosts ? 1 : 0) + 
    (fider.session.isAuthenticated && localFilter.notMyVotes ? 1 : 0) + 
    (localFilter.date ? 1 : 0)

  const tabClass = (tab: FilterTab) => 
    `flex-1 py-2 px-1 text-center text-sm cursor-pointer border-b-2 relative transition-colors hover:bg-tertiary ${
      activeTab === tab ? 'border-primary text-primary font-semibold' : 'border-transparent text-muted'
    }`

  const itemClass = (selected: boolean) =>
    `p-2 rounded-badge cursor-pointer transition-colors hover:bg-tertiary ${
      selected ? 'bg-accent-light border border-primary' : ''
    }`
  
  return (
    <div className="relative inline-block" ref={panelRef}>
      <div 
        className="flex items-center h-10 text-xs font-medium uppercase rounded-button border border-border text-foreground px-3 py-2 cursor-pointer transition-colors hover:bg-tertiary hover:border-border-strong"
        onClick={() => setIsExpanded(!isExpanded)}
      >
        <Icon sprite={HeroIconFilter} className="h-5 pr-1" />
        <Trans id="home.filter.label">Filter</Trans>
        {activeFilterCount > 0 && (
          <div className="bg-primary text-white inline-block rounded-full px-2 py-0.5 min-w-[20px] text-[10px] text-center ml-2">
            {activeFilterCount}
          </div>
        )}
      </div>
      
      {isExpanded && (
        <div className="absolute top-[calc(100%+8px)] left-0 w-80 sm:w-[360px] max-h-[500px] bg-elevated border border-border rounded-card shadow-lg z-dropdown overflow-hidden">
          <div className="flex border-b border-border">
            <div className={tabClass('tags')} onClick={() => setActiveTab('tags')}>
              <Trans id="filter.tab.tags">Tags</Trans>
              {localFilter.tags.length > 0 && (
                <span className="absolute top-0.5 right-1 text-xs bg-primary text-white rounded-full w-4 h-4 flex items-center justify-center">
                  {localFilter.tags.length}
                </span>
              )}
            </div>
            <div className={tabClass('status')} onClick={() => setActiveTab('status')}>
              <Trans id="filter.tab.status">Status</Trans>
              {localFilter.statuses.length > 0 && (
                <span className="absolute top-0.5 right-1 text-xs bg-primary text-white rounded-full w-4 h-4 flex items-center justify-center">
                  {localFilter.statuses.length}
                </span>
              )}
            </div>
            {fider.session.isAuthenticated && (
              <div className={tabClass('user')} onClick={() => setActiveTab('user')}>
                <Trans id="filter.tab.user">User</Trans>
                {(localFilter.myVotes || localFilter.myPosts || localFilter.notMyVotes) && (
                  <span className="absolute top-0.5 right-1 text-xs bg-primary text-white rounded-full w-4 h-4 flex items-center justify-center">
                    {(localFilter.myVotes ? 1 : 0) + (localFilter.myPosts ? 1 : 0) + (localFilter.notMyVotes ? 1 : 0)}
                  </span>
                )}
              </div>
            )}
            <div className={tabClass('date')} onClick={() => setActiveTab('date')}>
              <Trans id="filter.tab.date">Date</Trans>
              {localFilter.date && (
                <span className="absolute top-0.5 right-1 text-xs bg-primary text-white rounded-full w-4 h-4 flex items-center justify-center">1</span>
              )}
            </div>
          </div>
          
          <div className="p-3 max-h-[350px] overflow-y-auto">
            {activeTab === 'tags' && (
              <div>
                <Input
                  field="tagSearch"
                  placeholder={i18n._("home.filter.tags.search", { message: "Search tags..." })}
                  value={tagSearchQuery}
                  onChange={setTagSearchQuery}
                  icon={tagSearchQuery ? IconX : IconSearch}
                  onIconClick={tagSearchQuery ? () => setTagSearchQuery("") : undefined}
                />
                
                <div className="my-3">
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-sm text-foreground">
                      <Trans id="filter.tags.logic">Tag Logic:</Trans>
                    </span>
                    <div className="flex gap-1">
                      <Button 
                        type="button"
                        variant={localFilter.tagLogic === "OR" ? "primary" : "secondary"}
                        onClick={() => setLocalFilter({...localFilter, tagLogic: "OR"})}
                        size="small"
                      >
                        OR
                      </Button>
                      <Button 
                        type="button"
                        variant={localFilter.tagLogic === "AND" ? "primary" : "secondary"}
                        onClick={() => setLocalFilter({...localFilter, tagLogic: "AND"})}
                        size="small"
                      >
                        AND
                      </Button>
                    </div>
                  </div>
                  <div className="text-xs text-muted mt-1">
                    {localFilter.tagLogic === "OR" ? (
                      <Trans id="filter.tags.logic.or.description">Posts with ANY of the selected tags will be shown</Trans>
                    ) : (
                      <Trans id="filter.tags.logic.and.description">Only posts with ALL selected tags will be shown</Trans>
                    )}
                  </div>
                </div>
                
                <div>
                  {filteredTags.length === 0 ? (
                    <div className="text-center p-3 text-muted text-sm">
                      <Trans id="filter.tags.noresults">No tags match your search</Trans>
                    </div>
                  ) : (
                    <div className="flex flex-wrap gap-1.5">
                      {filteredTags.map(tag => (
                        <div 
                          key={tag.id} 
                          className="cursor-pointer"
                          onClick={() => toggleTag(tag.slug)}
                        >
                          <ShowTag tag={tag} selectable selected={localFilter.tags.includes(tag.slug)} />
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            )}
            
            {activeTab === 'status' && (
              <div>
                {PostStatus.All
                  .filter(s => s.filterable && props.countPerStatus[s.value])
                  .map(status => {
                    const id = `enum.poststatus.${status.value.toString()}`
                    return (
                      <div 
                        key={status.value} 
                        className={itemClass(localFilter.statuses.includes(status.value))}
                        onClick={() => toggleStatus(status.value)}
                      >
                        <div className="flex items-center justify-between gap-2">
                          <span className={localFilter.statuses.includes(status.value) ? "font-semibold text-foreground" : "text-foreground"}>
                            {i18n._(id, { message: status.title })}
                          </span>
                          {props.countPerStatus[status.value] > 0 && (
                            <span className="bg-surface-alt inline-block rounded-full px-2 py-0.5 text-[10px] text-muted text-center min-w-[20px]">
                              {props.countPerStatus[status.value]}
                            </span>
                          )}
                        </div>
                      </div>
                    )
                  })
                }
              </div>
            )}
            
            {activeTab === 'user' && fider.session.isAuthenticated && (
              <div>
                <div 
                  className={itemClass(localFilter.myVotes)}
                  onClick={() => toggleUserPreference('myVotes')}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className={localFilter.myVotes ? "font-semibold text-foreground" : "text-foreground"}>
                      <Trans id="filter.user.myvotes">My Votes</Trans>
                    </span>
                    {localFilter.myVotes && <Icon sprite={IconCheck} className="h-4 text-success" />}
                  </div>
                </div>
                <div 
                  className={itemClass(localFilter.myPosts)}
                  onClick={() => toggleUserPreference('myPosts')}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className={localFilter.myPosts ? "font-semibold text-foreground" : "text-foreground"}>
                      <Trans id="filter.user.myposts">My Posts</Trans>
                    </span>
                    {localFilter.myPosts && <Icon sprite={IconCheck} className="h-4 text-success" />}
                  </div>
                </div>
                <div 
                  className={itemClass(localFilter.notMyVotes)}
                  onClick={() => toggleUserPreference('notMyVotes')}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className={localFilter.notMyVotes ? "font-semibold text-foreground" : "text-foreground"}>
                      <Trans id="filter.user.notmyvotes">Hide My Votes</Trans>
                    </span>
                    {localFilter.notMyVotes && <Icon sprite={IconCheck} className="h-4 text-success" />}
                  </div>
                </div>
              </div>
            )}
            
            {activeTab === 'date' && (
              <div>
                {[
                  { id: "1d", label: i18n._("home.datefilter.option.1d", { message: "Last 24 hours" }) },
                  { id: "7d", label: i18n._("home.datefilter.option.7d", { message: "Last 7 days" }) },
                  { id: "30d", label: i18n._("home.datefilter.option.30d", { message: "Last 30 days" }) },
                  { id: "6m", label: i18n._("home.datefilter.option.6m", { message: "Last 6 months" }) },
                  { id: "1y", label: i18n._("home.datefilter.option.1y", { message: "Last year" }) },
                ].map(option => (
                  <div 
                    key={option.id} 
                    className={itemClass(localFilter.date === option.id)}
                    onClick={() => setDateFilter(localFilter.date === option.id ? undefined : option.id)}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <span className={localFilter.date === option.id ? "font-semibold text-foreground" : "text-foreground"}>
                        {option.label}
                      </span>
                      {localFilter.date === option.id && <Icon sprite={IconCheck} className="h-4 text-success" />}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
          
          <div className="flex justify-between p-3 border-t border-border bg-tertiary">
            <Button variant="primary" onClick={applyFilters} size="small">
              <Trans id="filter.action.apply">Apply Filters</Trans>
            </Button>
            <Button variant="tertiary" onClick={clearAllFilters} size="small">
              <Trans id="filter.action.clear">Clear All</Trans>
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
