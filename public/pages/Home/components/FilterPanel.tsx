import React, { useState, useRef, useEffect } from "react"
import { Tag, PostStatus } from "@fider/models"
import { Input, Icon, ShowTag, Button } from "@fider/components"
import { HStack } from "@fider/components/layout"
import { useFider } from "@fider/hooks"
import { i18n } from "@lingui/core"
import { Trans } from "@lingui/react/macro"
import { FilterState } from "@fider/hooks/usePostFilters"
import HeroIconFilter from "@fider/assets/images/heroicons-filter.svg"
import IconSearch from "@fider/assets/images/heroicons-search.svg"
import IconX from "@fider/assets/images/heroicons-x.svg"
import IconCheck from "@fider/assets/images/heroicons-check.svg"
import "./FilterPanel.scss"

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
  
  return (
    <div className="c-filter-panel" ref={panelRef}>
      <div 
        className="c-filter-panel__button h-10 text-medium text-xs rounded-md uppercase border border-gray-400 text-gray-800 p-2 px-3 cursor-pointer"
        onClick={() => setIsExpanded(!isExpanded)}
      >
        <HStack>
          <Icon sprite={HeroIconFilter} className="h-5 pr-1" />
          <Trans id="home.filter.label">Filter</Trans>
          {activeFilterCount > 0 && (
            <div id="c-dropdown-buttoncount" className="bg-gray-200 inline-block rounded-full px-2 py-1 w-min-4 text-2xs text-center">
              {activeFilterCount}
            </div>
          )}
        </HStack>
      </div>
      
      {isExpanded && (
        <div className="c-filter-panel__content">
          <div className="c-filter-panel__tabs">
            <div 
              className={`c-filter-panel__tab ${activeTab === 'tags' ? 'c-filter-panel__tab--active' : ''}`}
              onClick={() => setActiveTab('tags')}
            >
              <Trans id="filter.tab.tags">Tags</Trans>
              {localFilter.tags.length > 0 && (
                <span className="c-filter-panel__tab-count">{localFilter.tags.length}</span>
              )}
            </div>
            <div 
              className={`c-filter-panel__tab ${activeTab === 'status' ? 'c-filter-panel__tab--active' : ''}`}
              onClick={() => setActiveTab('status')}
            >
              <Trans id="filter.tab.status">Status</Trans>
              {localFilter.statuses.length > 0 && (
                <span className="c-filter-panel__tab-count">{localFilter.statuses.length}</span>
              )}
            </div>
            {fider.session.isAuthenticated && (
              <div 
                className={`c-filter-panel__tab ${activeTab === 'user' ? 'c-filter-panel__tab--active' : ''}`}
                onClick={() => setActiveTab('user')}
              >
                <Trans id="filter.tab.user">User</Trans>
                {(localFilter.myVotes || localFilter.myPosts || localFilter.notMyVotes) && (
                  <span className="c-filter-panel__tab-count">
                    {(localFilter.myVotes ? 1 : 0) + (localFilter.myPosts ? 1 : 0) + (localFilter.notMyVotes ? 1 : 0)}
                  </span>
                )}
              </div>
            )}
            <div 
              className={`c-filter-panel__tab ${activeTab === 'date' ? 'c-filter-panel__tab--active' : ''}`}
              onClick={() => setActiveTab('date')}
            >
              <Trans id="filter.tab.date">Date</Trans>
              {localFilter.date && <span className="c-filter-panel__tab-count">1</span>}
            </div>
          </div>
          
          <div className="c-filter-panel__tab-content">
            {activeTab === 'tags' && (
              <div className="c-filter-panel__tags">
                <Input
                  field="tagSearch"
                  placeholder={i18n._("home.filter.tags.search", { message: "Search tags..." })}
                  value={tagSearchQuery}
                  onChange={setTagSearchQuery}
                  icon={tagSearchQuery ? IconX : IconSearch}
                  onIconClick={tagSearchQuery ? () => setTagSearchQuery("") : undefined}
                />
                
                <div className="c-filter-panel__tag-logic my-3">
                  <HStack spacing={2} justify="between">
                    <span className="text-sm">
                      <Trans id="filter.tags.logic">Tag Logic:</Trans>
                    </span>
                    <HStack spacing={1}>
                      <Button 
                        type="button"
                        variant={localFilter.tagLogic === "OR" ? "primary" : "secondary"}
                        onClick={() => setLocalFilter({...localFilter, tagLogic: "OR"})}
                        className="px-3 py-1 text-xs"
                      >
                        OR
                      </Button>
                      <Button 
                        type="button"
                        variant={localFilter.tagLogic === "AND" ? "primary" : "secondary"}
                        onClick={() => setLocalFilter({...localFilter, tagLogic: "AND"})}
                        className="px-3 py-1 text-xs"
                      >
                        AND
                      </Button>
                    </HStack>
                  </HStack>
                  <div className="text-xs text-gray-500 mt-1">
                    {localFilter.tagLogic === "OR" ? (
                      <Trans id="filter.tags.logic.or.description">Posts with ANY of the selected tags will be shown</Trans>
                    ) : (
                      <Trans id="filter.tags.logic.and.description">Only posts with ALL selected tags will be shown</Trans>
                    )}
                  </div>
                </div>
                
                <div className="c-filter-panel__tag-list">
                  {filteredTags.length === 0 ? (
                    <div className="c-filter-panel__no-results">
                      <Trans id="filter.tags.noresults">No tags match your search</Trans>
                    </div>
                  ) : (
                    <div className="c-filter-panel__tag-grid">
                      {filteredTags.map(tag => (
                        <div 
                          key={tag.id} 
                          className={`c-filter-panel__tag-item ${localFilter.tags.includes(tag.slug) ? 'c-filter-panel__tag-item--selected' : ''}`}
                          onClick={() => toggleTag(tag.slug)}
                        >
                          <HStack spacing={2} align="center">
                            <ShowTag tag={tag} />
                            {localFilter.tags.includes(tag.slug) && (
                              <Icon sprite={IconCheck} className="c-filter-panel__tag-check h-4 text-green-600" />
                            )}
                          </HStack>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            )}
            
            {activeTab === 'status' && (
              <div className="c-filter-panel__statuses">
                {PostStatus.All
                  .filter(s => s.filterable && props.countPerStatus[s.value])
                  .map(status => {
                    const id = `enum.poststatus.${status.value.toString()}`
                    return (
                      <div 
                        key={status.value} 
                        className={`c-filter-panel__status-item ${localFilter.statuses.includes(status.value) ? 'c-filter-panel__status-item--selected' : ''}`}
                        onClick={() => toggleStatus(status.value)}
                      >
                        <HStack spacing={2} justify="between">
                          <span className={localFilter.statuses.includes(status.value) ? "text-semibold" : ""}>
                            {i18n._(id, { message: status.title })}
                          </span>
                          {props.countPerStatus[status.value] > 0 && (
                            <span className="bg-gray-200 inline-block rounded-full px-2 py-1 text-2xs text-center min-w-[20px]">
                              {props.countPerStatus[status.value]}
                            </span>
                          )}
                        </HStack>
                      </div>
                    )
                  })
                }
              </div>
            )}
            
            {activeTab === 'user' && fider.session.isAuthenticated && (
              <div className="c-filter-panel__user-prefs">
                <div 
                  className={`c-filter-panel__user-item ${localFilter.myVotes ? 'c-filter-panel__user-item--selected' : ''}`}
                  onClick={() => toggleUserPreference('myVotes')}
                >
                  <HStack spacing={2} justify="between">
                    <span className={localFilter.myVotes ? "text-semibold" : ""}>
                      <Trans id="filter.user.myvotes">My Votes</Trans>
                    </span>
                    {localFilter.myVotes && <Icon sprite={IconCheck} className="h-4 text-green-600" />}
                  </HStack>
                </div>
                <div 
                  className={`c-filter-panel__user-item ${localFilter.myPosts ? 'c-filter-panel__user-item--selected' : ''}`}
                  onClick={() => toggleUserPreference('myPosts')}
                >
                  <HStack spacing={2} justify="between">
                    <span className={localFilter.myPosts ? "text-semibold" : ""}>
                      <Trans id="filter.user.myposts">My Posts</Trans>
                    </span>
                    {localFilter.myPosts && <Icon sprite={IconCheck} className="h-4 text-green-600" />}
                  </HStack>
                </div>
                <div 
                  className={`c-filter-panel__user-item ${localFilter.notMyVotes ? 'c-filter-panel__user-item--selected' : ''}`}
                  onClick={() => toggleUserPreference('notMyVotes')}
                >
                  <HStack spacing={2} justify="between">
                    <span className={localFilter.notMyVotes ? "text-semibold" : ""}>
                      <Trans id="filter.user.notmyvotes">Hide My Votes</Trans>
                    </span>
                    {localFilter.notMyVotes && <Icon sprite={IconCheck} className="h-4 text-green-600" />}
                  </HStack>
                </div>
              </div>
            )}
            
            {activeTab === 'date' && (
              <div className="c-filter-panel__dates">
                {[
                  { id: "1d", label: i18n._("home.datefilter.option.1d", { message: "Last 24 hours" }) },
                  { id: "7d", label: i18n._("home.datefilter.option.7d", { message: "Last 7 days" }) },
                  { id: "30d", label: i18n._("home.datefilter.option.30d", { message: "Last 30 days" }) },
                  { id: "6m", label: i18n._("home.datefilter.option.6m", { message: "Last 6 months" }) },
                  { id: "1y", label: i18n._("home.datefilter.option.1y", { message: "Last year" }) },
                ].map(option => (
                  <div 
                    key={option.id} 
                    className={`c-filter-panel__date-item ${localFilter.date === option.id ? 'c-filter-panel__date-item--selected' : ''}`}
                    onClick={() => setDateFilter(localFilter.date === option.id ? undefined : option.id)}
                  >
                    <HStack spacing={2} justify="between">
                      <span className={localFilter.date === option.id ? "text-semibold" : ""}>
                        {option.label}
                      </span>
                      {localFilter.date === option.id && <Icon sprite={IconCheck} className="h-4 text-green-600" />}
                    </HStack>
                  </div>
                ))}
              </div>
            )}
          </div>
          
          <div className="c-filter-panel__actions">
            <Button variant="primary" onClick={applyFilters} className="text-sm">
              <Trans id="filter.action.apply">Apply Filters</Trans>
            </Button>
            <Button variant="tertiary" onClick={clearAllFilters} className="text-sm">
              <Trans id="filter.action.clear">Clear All</Trans>
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}