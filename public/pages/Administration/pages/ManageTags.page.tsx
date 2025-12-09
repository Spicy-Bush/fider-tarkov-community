import React, { useState, useCallback } from "react"
import { Button } from "@fider/components"
import { Tag } from "@fider/models"
import { actions, Failure, Fider } from "@fider/services"
import { TagFormState, TagForm } from "../components/TagForm"
import { TagListItem } from "../components/TagListItem"
import { VStack } from "@fider/components/layout"
import { PageConfig } from "@fider/components/layouts"

export const pageConfig: PageConfig = {
  title: "Tags",
  subtitle: "Manage your site tags",
  sidebarItem: "tags",
}

interface ManageTagsPageProps {
  tags: Tag[]
}

const tagSorter = (t1: Tag, t2: Tag) => {
  if (t1.name < t2.name) {
    return -1
  } else if (t1.name > t2.name) {
    return 1
  }
  return 0
}

const ManageTagsPage: React.FC<ManageTagsPageProps> = (props) => {
  const [isAdding, setIsAdding] = useState(false)
  const [allTags, setAllTags] = useState<Tag[]>(() => [...props.tags].sort(tagSorter))

  const addNew = () => {
    setIsAdding(true)
  }

  const cancelAdd = () => {
    setIsAdding(false)
  }

  const saveNewTag = async (data: TagFormState): Promise<Failure | undefined> => {
    const result = await actions.createTag(data.name, data.color, data.isPublic)
    if (result.ok) {
      setIsAdding(false)
      setAllTags(prev => [...prev, result.data].sort(tagSorter))
    } else {
      return result.error
    }
  }

  const handleTagDeleted = useCallback((tag: Tag) => {
    setAllTags(prev => prev.filter(t => t.id !== tag.id))
  }, [])

  const handleTagEdited = useCallback(() => {
    setAllTags(prev => [...prev].sort(tagSorter))
  }, [])

  const getTagList = (filter: (tag: Tag) => boolean) => {
    return allTags.filter(filter).map((t) => (
      <TagListItem key={t.id} tag={t} onTagDeleted={handleTagDeleted} onTagEdited={handleTagEdited} />
    ))
  }

  const publicTaglist = getTagList((t) => t.isPublic)
  const privateTagList = getTagList((t) => !t.isPublic)

  const form =
    Fider.session.user.isAdministrator &&
    (isAdding ? (
      <TagForm onSave={saveNewTag} onCancel={cancelAdd} />
    ) : (
      <Button variant="secondary" onClick={addNew}>
        Add new
      </Button>
    ))

  return (
    <VStack spacing={8}>
      <div>
        <h2 className="text-display">Public Tags</h2>
        <p className="text-muted">These tags are visible to all visitors.</p>
        <VStack spacing={4} divide={true}>
          {publicTaglist.length === 0 ? <p className="text-muted">There aren't any public tags yet.</p> : publicTaglist}
        </VStack>
      </div>
      <div>
        <h2 className="text-display">Private Tags</h2>
        <p className="text-muted">These tags are only visible for members of this site.</p>
        <VStack spacing={4} divide={true}>
          {privateTagList.length === 0 ? <p className="text-muted">There aren't any private tags yet.</p> : privateTagList}
        </VStack>
      </div>
      <div>{form}</div>
    </VStack>
  )
}

export default ManageTagsPage
