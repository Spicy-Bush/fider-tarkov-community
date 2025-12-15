import React, { useState, useEffect, useRef, useCallback } from "react"
import { Button, Form, Icon, Input, ImageUploader } from "@fider/components"
import { HStack } from "@fider/components/layout"
import { Failure, actions, notify, classSet, copyToClipboard } from "@fider/services"
import { ImageUpload } from "@fider/models"
import { PageConfig } from "@fider/components/layouts"

import { heroiconsTrash as IconTrash, heroiconsPencilAlt as IconPencilAlt, heroiconsDownload as IconDownload, heroiconsUpload as IconUpload, heroiconsEye as IconEye, heroiconsX as IconX, heroiconsSearch as IconSearch, heroiconsDuplicate as IconCopy, heroiconsCheck as IconCheck, heroiconsExternalLink as IconExternalLink } from "@fider/icons.generated"

export const pageConfig: PageConfig = {
  title: "Media Library",
  subtitle: "Manage your site's images, logos, avatars and attachments",
  sidebarItem: "files",
}

enum MediaType {
  ALL = "all",
  ADMIN = "files",
  ATTACHMENT = "attachments",
  AVATAR = "avatars",
  LOGO = "logos"
}

interface MediaAsset {
  name: string
  blobKey: string
  size: number
  contentType: string
  createdAt: string
  isInUse: boolean
  usedIn?: string[]
  mediaType: MediaType
}

const Loader: React.FC = () => {
  return (
    <div className="p-8 flex justify-center">
      <div className="h-8 w-8 rounded-full border-2 border-surface-alt border-b-primary animate-spin"></div>
    </div>
  );
};

interface PaginationProps {
  currentPage: number
  totalPages: number
  onPageChange: (page: number) => void
}

const Pagination: React.FC<PaginationProps> = ({ currentPage, totalPages, onPageChange }) => {
  if (totalPages <= 1) return null;
  
  const maxVisiblePages = 5;
  
  let startPage = Math.max(1, currentPage - Math.floor(maxVisiblePages / 2));
  let endPage = Math.min(totalPages, startPage + maxVisiblePages - 1);
  
  if (endPage - startPage + 1 < maxVisiblePages) {
    startPage = Math.max(1, endPage - maxVisiblePages + 1);
  }

  return (
    <div className="flex justify-center mt-6 gap-1">
      <Button
        variant="tertiary"
        size="small"
        onClick={() => { if (currentPage > 1) onPageChange(currentPage - 1); }}
        disabled={currentPage === 1}
      >
        &lt;
      </Button>
      
      {Array.from({ length: endPage - startPage + 1 }, (_, i) => {
        const page = startPage + i;
        return (
          <Button
            key={page}
            variant={page === currentPage ? "primary" : "tertiary"}
            size="small"
            onClick={() => onPageChange(page)}
            className="min-w-8 h-8 p-0 flex items-center justify-center"
          >
            {page}
          </Button>
        );
      })}
      
      <Button
        variant="tertiary"
        size="small"
        onClick={() => { if (currentPage < totalPages) onPageChange(currentPage + 1); }}
        disabled={currentPage === totalPages}
      >
        &gt;
      </Button>
    </div>
  );
};

const getMediaTypeFromBlobKey = (blobKey: string): MediaType => {
  if (blobKey.startsWith('files/')) return MediaType.ADMIN;
  if (blobKey.startsWith('attachments/')) return MediaType.ATTACHMENT;
  if (blobKey.startsWith('avatars/')) return MediaType.AVATAR;
  if (blobKey.startsWith('logos/')) return MediaType.LOGO;
  return MediaType.ALL;
}

const mapApiResponseToAssets = (apiAssets: any[]): MediaAsset[] => {
  return apiAssets.map(asset => ({
    ...asset,
    mediaType: getMediaTypeFromBlobKey(asset.blobKey)
  }));
}

const getAssetURL = (asset: MediaAsset): string => {
  if (asset.blobKey.startsWith('logos/')) {
    return `/static/favicon/${asset.blobKey}`;
  }
  return `/static/images/${asset.blobKey}`;
}

const formatFileSize = (bytes: number): string => {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

const getMediaTypeLabel = (mediaType: MediaType): string => {
  switch(mediaType) {
    case MediaType.ADMIN: return "Admin";
    case MediaType.ATTACHMENT: return "Attachment";
    case MediaType.AVATAR: return "Avatar";
    case MediaType.LOGO: return "Logo";
    default: return "Unknown";
  }
}

const getBadgeClasses = (mediaType: MediaType): string => {
  switch(mediaType) {
    case MediaType.ADMIN: return "bg-info-medium text-primary";
    case MediaType.ATTACHMENT: return "bg-success-medium text-success";
    case MediaType.AVATAR: return "bg-info-medium text-primary";
    case MediaType.LOGO: return "bg-warning-medium text-warning";
    default: return "bg-surface-alt text-muted";
  }
}

const FileManagementPage: React.FC = () => {
  const [view, setView] = useState<"browse" | "upload">("browse")
  const [assets, setAssets] = useState<MediaAsset[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [selectedAsset, setSelectedAsset] = useState<MediaAsset | undefined>()
  const [searchQuery, setSearchQuery] = useState("")
  const [assetToEdit, setAssetToEdit] = useState<MediaAsset | undefined>()
  const [newAssetName, setNewAssetName] = useState("")
  const [imageUpload, setImageUpload] = useState<ImageUpload | undefined>()
  const [uploadType, setUploadType] = useState<"file" | "attachment">("file")
  const [error, setError] = useState<Failure | undefined>()
  const [showUsageModal, setShowUsageModal] = useState(false)
  const [mediaTypeFilter, setMediaTypeFilter] = useState<MediaType>(MediaType.ALL)
  const [pagination, setPagination] = useState({
    page: 1,
    pageSize: 20,
    total: 0,
    totalPages: 0
  })
  const [sortBy, setSortBy] = useState("createdAt")
  const [sortDir, setSortDir] = useState("desc")
  const [confirmDeleteOpen, setConfirmDeleteOpen] = useState(false)
  const [assetToDelete, setAssetToDelete] = useState<MediaAsset | undefined>()
  const [copiedAssetKey, setCopiedAssetKey] = useState<string | null>(null)
  const [showImageGallery, setShowImageGallery] = useState(false)
  const [galleryImageUrl, setGalleryImageUrl] = useState<string>("")
  
  const searchTimeout = useRef<ReturnType<typeof setTimeout> | null>(null)

  const loadAssets = useCallback(async () => {
    setIsLoading(true)
    
    const params = new URLSearchParams();
    params.append('page', String(pagination.page));
    params.append('pageSize', String(pagination.pageSize));
    
    if (searchQuery) {
      params.append('search', searchQuery);
    }
    
    params.append('sortBy', sortBy);
    params.append('sortDir', sortDir);
    
    if (mediaTypeFilter !== MediaType.ALL) {
      params.append('type', mediaTypeFilter);
    }
    
    const url = `/api/v1/admin/files?${params.toString()}`;
    const result = await actions.listFiles(url);
    
    if (result.ok) {
      const mappedAssets = mapApiResponseToAssets(result.data.files);
      
      setAssets(mappedAssets)
      setIsLoading(false)
      setPagination({
        page: result.data.page,
        pageSize: result.data.pageSize,
        total: result.data.total,
        totalPages: result.data.totalPages
      })
    } else {
      setError(result.error)
      setIsLoading(false)
      notify.error("Could not load media assets")
    }
  }, [pagination.page, pagination.pageSize, searchQuery, sortBy, sortDir, mediaTypeFilter])

  useEffect(() => {
    loadAssets()
  }, [])

  useEffect(() => {
    loadAssets()
  }, [pagination.page, sortBy, sortDir, mediaTypeFilter])

  const handlePageChange = (page: number) => {
    setPagination(prev => ({ ...prev, page }))
  }

  const handleSearchChange = (value: string) => {
    setSearchQuery(value)
    setPagination(prev => ({ ...prev, page: 1 }))
  
    if (searchTimeout.current !== null) {
      clearTimeout(searchTimeout.current);
    }
  
    searchTimeout.current = setTimeout(() => {
      loadAssets();
    }, 200);
  }

  const handleSortChange = (newSortBy: string) => {
    const newSortDir = sortBy === newSortBy && sortDir === "asc" ? "desc" : "asc";
    setSortBy(newSortBy)
    setSortDir(newSortDir)
    setPagination(prev => ({ ...prev, page: 1 }))
  }

  const handleMediaTypeFilter = (newMediaType: MediaType) => {
    setMediaTypeFilter(newMediaType)
    setPagination(prev => ({ ...prev, page: 1 }))
  }

  const handleImageUpload = async () => {
    if (!imageUpload) {
      notify.error("Please select an image to upload")
      return
    }
  
    const assetName = newAssetName.trim() || imageUpload.upload?.fileName || "unnamed image"
    
    const result = await actions.uploadFile({
      name: assetName,
      file: imageUpload,
      uploadType: uploadType
    })
  
    if (result.ok) {
      notify.success("Image uploaded successfully")
      setView("browse")
      setImageUpload(undefined)
      setNewAssetName("")
      setUploadType("file")
      await loadAssets()
    } else {
      setError(result.error)
      notify.error("Failed to upload image")
    }
  }

  const openDeleteConfirmation = (asset: MediaAsset) => {
    setAssetToDelete(asset)
    setConfirmDeleteOpen(true)
  }

  const closeDeleteConfirmation = () => {
    setAssetToDelete(undefined)
    setConfirmDeleteOpen(false)
  }

  const deleteAsset = async (forceDelete: boolean = false) => {
    if (!assetToDelete) return;

    closeDeleteConfirmation();

    const endpoint = `/api/v1/admin/files/${assetToDelete.blobKey}${forceDelete ? '?force=true' : ''}`;

    const result = await fetch(endpoint, {
      method: 'DELETE',
      credentials: 'same-origin',
      headers: {
        'Content-Type': 'application/json',
      }
    });

    if (result.ok) {
      notify.success(`Image ${forceDelete ? 'forcefully ' : ''}deleted successfully`);
      await loadAssets();
    } else {
      const data = await result.json();
      notify.error(data.message || "Failed to delete image");
    }
  }

  const handleEditAsset = async () => {
    if (!assetToEdit) return;

    const name = newAssetName.trim();
    if (!name) {
      notify.error("Name cannot be empty");
      return;
    }

    const result = await fetch(`/api/v1/admin/files/${assetToEdit.blobKey}`, {
      method: 'PUT',
      credentials: 'same-origin',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ name })
    });

    if (result.ok) {
      notify.success("Image renamed successfully");
      setAssetToEdit(undefined)
      setNewAssetName("")
      await loadAssets();
    } else {
      const data = await result.json();
      notify.error(data.message || "Failed to rename image");
    }
  }

  const handleImageSelection = (image: ImageUpload) => {
    setImageUpload(image)
  }

  const startEdit = (asset: MediaAsset) => {
    setAssetToEdit(asset)
    setNewAssetName(asset.name)
  }

  const downloadAsset = (asset: MediaAsset) => {
    const url = getAssetURL(asset);
    window.open(url, "_blank");
  }

  const copyImageLocation = async (asset: MediaAsset) => {
    const url = getAssetURL(asset);
    const fullUrl = `${window.location.origin}${url}`;
    try {
      await copyToClipboard(fullUrl);
      setCopiedAssetKey(asset.blobKey);
      notify.success("Image location copied to clipboard");
      setTimeout(() => setCopiedAssetKey(null), 2000);
    } catch {
      notify.error("Failed to copy to clipboard");
    }
  }

  const openImageGallery = (asset: MediaAsset) => {
    const url = getAssetURL(asset);
    setGalleryImageUrl(url);
    setShowImageGallery(true);
    document.body.style.overflow = "hidden";
  }

  const closeImageGallery = () => {
    setShowImageGallery(false);
    document.body.style.overflow = "";
  }

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (showImageGallery && e.key === "Escape") {
        closeImageGallery();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [showImageGallery]);

  const showAssetUsage = (asset: MediaAsset) => {
    setSelectedAsset(asset)
    setShowUsageModal(true)
  }

  const renderUploadView = () => {
    return (
      <Form error={error}>
        <Input
          field="fileName"
          label="Image Name"
          placeholder="Enter a name for this image"
          value={newAssetName}
          onChange={(value) => setNewAssetName(value)}
        />

        <div className="mb-4">
          <label className="block mb-2 font-medium text-muted">Upload Type</label>
          <div className="flex gap-4">
            <label className="flex items-center cursor-pointer">
              <input
                type="radio"
                name="uploadType"
                checked={uploadType === "file"}
                onChange={() => setUploadType("file")}
                className="mr-2"
              />
              <span>Admin Upload (files/)</span>
            </label>
            <label className="flex items-center cursor-pointer">
              <input
                type="radio"
                name="uploadType"
                checked={uploadType === "attachment"}
                onChange={() => setUploadType("attachment")}
                className="mr-2"
              />
              <span>Public Attachment (attachments/)</span>
            </label>
          </div>
          <p className="text-sm text-muted mt-2">
            {uploadType === "file" 
              ? "Use this option for images managed by administrators."
              : "Use this option for images attached to posts/comments."}
          </p>
        </div>

        <ImageUploader
          field="imageUpload"
          label="Upload Image"
          onChange={handleImageSelection}
          bkey=""
        >
          <p className="text-muted">Select an image to upload. JPG, PNG, GIF, and SVG formats are supported.</p>
        </ImageUploader>

        <div className="mt-4">
          <HStack>
            <Button variant="primary" onClick={handleImageUpload}>
              Upload Image
            </Button>
            <Button variant="tertiary" onClick={() => setView("browse")}>
              Cancel
            </Button>
          </HStack>
        </div>
      </Form>
    )
  }

  const renderEditView = () => {
    if (!assetToEdit) return null;

    return (
      <div className="fixed inset-0 z-modal flex items-center justify-center">
        <div className="fixed inset-0 bg-black/50" 
             onClick={() => { setAssetToEdit(undefined); setNewAssetName(""); }}></div>
        <div className="bg-elevated rounded-modal shadow-xl p-6 relative max-w-[400px] w-[90%] z-modal-content">
          <h2 className="text-lg font-semibold text-foreground mb-4">Rename Image</h2>
          <Form error={error}>
            <Input
              field="newAssetName"
              label={`New name for "${assetToEdit.name}"`}
              placeholder="Enter new name"
              value={newAssetName}
              onChange={(value) => setNewAssetName(value)}
            />

            <div className="flex justify-end gap-2 mt-4">
              <Button 
                variant="tertiary" 
                onClick={() => { setAssetToEdit(undefined); setNewAssetName(""); }}
              >
                Cancel
              </Button>
              <Button variant="primary" onClick={handleEditAsset}>
                Save
              </Button>
            </div>
          </Form>
        </div>
      </div>
    );
  }

  const renderDeleteConfirmation = () => {
    if (!confirmDeleteOpen || !assetToDelete) return null;

    const isInUse = assetToDelete.isInUse;

    return (
      <div className="fixed inset-0 z-modal flex items-center justify-center">
        <div className="fixed inset-0 bg-black/50" onClick={closeDeleteConfirmation}></div>
        <div className="bg-elevated rounded-modal shadow-xl p-6 relative max-w-[400px] w-[90%] z-modal-content">
          <h2 className="text-lg font-semibold text-foreground mb-4">Delete Image</h2>
          
          <div className="mb-4">
            <p className="mb-2">Are you sure you want to delete <strong>{assetToDelete.name}</strong>?</p>
            
            {isInUse && (
              <div className="bg-warning-medium border-l-4 border-warning text-warning p-3 rounded-r mb-3">
                <p className="font-semibold mb-1">Warning: This image is in use</p>
                <p className="text-sm">Forcing deletion will remove all references to this image from posts, comments, avatars, or logos.</p>
              </div>
            )}
          </div>
          
          <div className="flex justify-end gap-2 mt-4">
            <Button 
              variant="tertiary" 
              onClick={closeDeleteConfirmation}
            >
              Cancel
            </Button>
            
            {isInUse && (
              <Button 
                variant="danger" 
                onClick={() => deleteAsset(true)}
              >
                Force Delete
              </Button>
            )}
            
            <Button 
              variant={isInUse ? "secondary" : "danger"} 
              onClick={() => deleteAsset(false)}
              disabled={isInUse}
            >
              {isInUse ? "Cannot Delete (In Use)" : "Delete"}
            </Button>
          </div>
        </div>
      </div>
    );
  }

  const parseUsageLink = (usage: string): { text: string; href?: string; linkText?: string } => {
    const commentMatch = usage.match(/Comment #(\d+) on Post #(\d+)/i)
    if (commentMatch) {
      return { text: usage, href: `/posts/${commentMatch[2]}#comment-${commentMatch[1]}`, linkText: "View comment" }
    }
    const postMatch = usage.match(/Post #(\d+)/i)
    if (postMatch) {
      return { text: usage, href: `/posts/${postMatch[1]}`, linkText: "View post" }
    }
    const userMatch = usage.match(/User #(\d+): (.+)/i)
    if (userMatch) {
      return { text: usage, href: `/profile/${userMatch[1]}`, linkText: "View profile" }
    }
    return { text: usage, href: undefined }
  }

  const renderUsageModal = () => {
    if (!selectedAsset || !showUsageModal) return null;

    return (
      <div className="fixed inset-0 z-modal flex items-center justify-center">
        <div className="fixed inset-0 bg-black/50"
             onClick={() => setShowUsageModal(false)}></div>
        <div className="bg-elevated rounded-modal shadow-xl p-6 relative max-w-[500px] w-[90%] z-modal-content">
          <div className="flex justify-between items-center mb-4">
            <h2 className="text-lg font-semibold text-foreground">Image Usage</h2>
            <button 
              type="button"
              className="p-1.5 rounded-badge text-muted hover:text-foreground hover:bg-surface-alt transition-colors cursor-pointer border-none bg-transparent"
              onClick={() => setShowUsageModal(false)}
            >
              <Icon sprite={IconX} className="w-5 h-5" />
            </button>
          </div>
          
          <div className="flex items-center gap-4 mb-4 p-3 bg-tertiary rounded-card">
            <div className="w-16 h-16 rounded-card overflow-hidden bg-surface-alt shrink-0">
              <img 
                src={getAssetURL(selectedAsset)} 
                alt={selectedAsset.name}
                className="w-full h-full object-cover"
              />
            </div>
            <div className="min-w-0 flex-1">
              <div className="font-medium text-foreground truncate">{selectedAsset.name}</div>
              <div className="flex items-center gap-2 mt-1">
                <span className={`text-2xs px-2 py-0.5 rounded-full font-medium ${getBadgeClasses(selectedAsset.mediaType)}`}>
                  {getMediaTypeLabel(selectedAsset.mediaType)}
                </span>
                <span className="text-xs text-muted">{formatFileSize(selectedAsset.size)}</span>
              </div>
            </div>
          </div>
          
          <div className="mb-4">
            <div className="text-sm font-medium text-foreground mb-2">Used in:</div>
            <div className="bg-tertiary rounded-card overflow-hidden">
                {selectedAsset.usedIn && selectedAsset.usedIn.length > 0 ? (
                <div className="max-h-[250px] overflow-y-auto divide-y divide-surface-alt">
                  {selectedAsset.usedIn.map((usage: string, index: number) => {
                    const { text, href, linkText } = parseUsageLink(usage)
                    return href ? (
                      <div 
                        key={index}
                        className="flex items-center justify-between px-3 py-2.5 text-sm text-foreground hover:bg-surface-alt transition-colors"
                      >
                        <span>{text}</span>
                        <a
                          href={href}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="flex items-center gap-1 text-primary text-xs hover:text-primary-hover transition-colors"
                          onClick={(e) => e.stopPropagation()}
                        >
                          <span>{linkText}</span>
                          <Icon sprite={IconExternalLink} className="w-3 h-3" />
                        </a>
                      </div>
                    ) : (
                      <div key={index} className="px-3 py-2.5 text-sm text-muted">{text}</div>
                    )
                  })}
                </div>
              ) : (
                <div className="px-3 py-6 text-center text-muted italic">
                  This image is not currently used anywhere.
                </div>
              )}
            </div>
          </div>
          
          <div className="flex justify-end gap-2">
            <Button 
              variant="tertiary" 
              size="small"
              onClick={() => setShowUsageModal(false)}
            >
              Close
            </Button>
          </div>
        </div>
      </div>
    );
  }

  const renderMediaCard = (asset: MediaAsset) => {
    return (
      <div key={asset.blobKey} className="bg-elevated rounded-card overflow-hidden border border-surface-alt transition-all hover:shadow-lg hover:border-border-strong group">
        <div 
          className="relative aspect-square bg-surface-alt flex items-center justify-center overflow-hidden cursor-pointer"
          onClick={() => openImageGallery(asset)}
        >
          <img 
            src={getAssetURL(asset)} 
            alt={asset.name}
            className="w-full h-full object-cover transition-transform duration-75 group-hover:scale-105"
            loading="lazy"
          />
          
          <div className="absolute top-2 left-2 flex flex-col gap-1">
            <span className={`text-2xs px-2 py-0.5 rounded-full font-medium ${getBadgeClasses(asset.mediaType)}`}>
              {getMediaTypeLabel(asset.mediaType)}
            </span>
            
            {asset.isInUse && (
              <span className="text-2xs px-2 py-0.5 rounded-full font-medium bg-success-light text-success">
                In Use
              </span>
            )}
          </div>
          
          <div className="absolute bottom-2 right-2 bg-black/70 text-white text-2xs px-2 py-0.5 rounded-full backdrop-blur-sm">
            {formatFileSize(asset.size)}
          </div>
        </div>
        
        <div className="p-3">
          <div className="mb-2">
            <span className="font-medium text-sm overflow-hidden text-ellipsis whitespace-nowrap text-foreground block" title={asset.name}>
              {asset.name}
            </span>
            <span className="text-2xs text-muted">
              {new Date(asset.createdAt).toLocaleDateString()}
            </span>
          </div>
          
          <div className="flex gap-1">
            <button 
              type="button"
              onClick={() => downloadAsset(asset)} 
              className="flex-1 p-2 min-w-0 rounded-badge border-none bg-transparent text-muted hover:bg-surface-alt hover:text-foreground transition-colors cursor-pointer flex items-center justify-center"
              title="Download"
            >
              <Icon sprite={IconDownload} className="w-4 h-4" />
            </button>
            
            <button 
              type="button"
              onClick={() => copyImageLocation(asset)} 
              className="flex-1 p-2 min-w-0 rounded-badge border-none bg-transparent text-muted hover:bg-surface-alt hover:text-foreground transition-colors cursor-pointer flex items-center justify-center"
              title="Copy image location"
            >
              <Icon sprite={copiedAssetKey === asset.blobKey ? IconCheck : IconCopy} className="w-4 h-4" />
            </button>
            
            {asset.isInUse && (
              <button 
                type="button"
                onClick={() => showAssetUsage(asset)} 
                className="flex-1 p-2 min-w-0 rounded-badge border-none bg-transparent text-muted hover:bg-accent-light hover:text-primary transition-colors cursor-pointer flex items-center justify-center"
                title="View usage"
              >
                <Icon sprite={IconEye} className="w-4 h-4" />
              </button>
            )}
            
            <button 
              type="button"
              onClick={() => startEdit(asset)} 
              className="flex-1 p-2 min-w-0 rounded-badge border-none bg-transparent text-muted hover:bg-surface-alt hover:text-foreground transition-colors cursor-pointer flex items-center justify-center"
              title="Rename"
            >
              <Icon sprite={IconPencilAlt} className="w-4 h-4" />
            </button>
            
            <button 
              type="button"
              onClick={() => openDeleteConfirmation(asset)} 
              className="flex-1 p-2 min-w-0 rounded-badge border-none bg-transparent text-muted hover:bg-danger-light hover:text-danger transition-colors cursor-pointer flex items-center justify-center"
              title="Delete"
            >
              <Icon sprite={IconTrash} className="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>
    );
  }

  const renderMediaGrid = () => {
    if (isLoading) {
      return <Loader />;
    }

    if (assets.length === 0) {
      return (
        <div className="flex flex-col items-center justify-center p-12 px-4 text-center">
          <div className="text-border mb-4 [&_svg]:w-12 [&_svg]:h-12">
            <Icon sprite={IconEye} />
          </div>
          <p className="text-muted text-lg font-medium mb-2">No images found</p>
          <p className="text-muted text-sm mb-4">
            {searchQuery || mediaTypeFilter !== MediaType.ALL
              ? "Try different search or filter settings"
              : ""}
          </p>
          {(searchQuery || mediaTypeFilter !== MediaType.ALL) && (
            <Button
              variant="secondary"
              onClick={() => {
                setSearchQuery("")
                setMediaTypeFilter(MediaType.ALL)
                setPagination(prev => ({ ...prev, page: 1 }))
              }}
            >
              Clear Filters
            </Button>
          )}
        </div>
      );
    }

    return (
      <div className="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-4">
        {assets.map(asset => renderMediaCard(asset))}
      </div>
    );
  }

  const renderBrowseView = () => {
    if (assetToEdit) {
      return renderEditView();
    }

    return (
      <>
        {renderUsageModal()}
        {renderDeleteConfirmation()}
        
        <div className="flex flex-col sm:flex-row justify-between mb-4 gap-3">
          <div className="relative flex-1 max-w-[350px] [&_.c-form-field]:mb-0">
            <Input
              field="search"
              icon={searchQuery ? IconX : IconSearch}
              onIconClick={searchQuery ? () => handleSearchChange("") : undefined}
              placeholder="Search images..."
              value={searchQuery}
              onChange={handleSearchChange}
            />
          </div>
          
          <Button variant="primary" onClick={() => setView("upload")}>
            <Icon sprite={IconUpload} className="w-4 h-4" />
            <span>Upload</span>
          </Button>
        </div>
        
        <div className="bg-elevated border border-surface-alt rounded-card p-4 mb-5">
          <div className="flex flex-wrap items-center gap-2 mb-3 pb-3 border-b border-surface-alt">
            <span className="text-sm font-medium text-muted shrink-0">Type:</span>
            {[
              { value: MediaType.ALL, label: "All" },
              { value: MediaType.ADMIN, label: "Admin" },
              { value: MediaType.ATTACHMENT, label: "Attachments" },
              { value: MediaType.AVATAR, label: "Avatars" },
              { value: MediaType.LOGO, label: "Logos" },
            ].map(({ value, label }) => (
              <button 
                key={value}
                type="button"
                onClick={() => handleMediaTypeFilter(value)}
                className={classSet({
                  "px-3 py-1.5 text-sm rounded-button border-none cursor-pointer transition-colors": true,
                  "bg-primary text-white": mediaTypeFilter === value,
                  "bg-transparent text-muted hover:bg-surface-alt hover:text-foreground": mediaTypeFilter !== value,
                })}
              >
                {label}
              </button>
            ))}
          </div>
          
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm font-medium text-muted shrink-0">Sort:</span>
            {[
              { value: "name", label: "Name" },
              { value: "size", label: "Size" },
              { value: "createdAt", label: "Date" },
            ].map(({ value, label }) => (
              <button 
                key={value}
                type="button"
                onClick={() => handleSortChange(value)}
                className={classSet({
                  "px-3 py-1.5 text-sm rounded-button border-none cursor-pointer transition-colors inline-flex items-center gap-1": true,
                  "bg-accent-light text-primary font-medium": sortBy === value,
                  "bg-transparent text-muted hover:bg-surface-alt hover:text-foreground": sortBy !== value,
                })}
              >
                {label}
                {sortBy === value && (
                  <span className="text-xs">{sortDir === "asc" ? "\u2191" : "\u2193"}</span>
                )}
              </button>
            ))}
            
            <button 
              type="button"
              onClick={loadAssets}
              className="ml-auto px-3 py-1.5 text-sm rounded-input border border-border bg-elevated text-muted hover:bg-surface-alt hover:text-foreground transition-colors cursor-pointer"
            >
              Refresh
            </button>
          </div>
        </div>
        
        {renderMediaGrid()}

        {pagination.totalPages > 1 && (
          <Pagination
            currentPage={pagination.page}
            totalPages={pagination.totalPages}
            onPageChange={handlePageChange}
          />
        )}

        {pagination.total > 0 && (
          <div className="mt-4 text-sm text-muted text-center">
            Showing {Math.min((pagination.page - 1) * pagination.pageSize + 1, pagination.total)} 
            -{Math.min(pagination.page * pagination.pageSize, pagination.total)} of {pagination.total} image
            {pagination.total !== 1 ? 's' : ''}
          </div>
        )}
      </>
    );
  }

  const renderImageGallery = () => {
    if (!showImageGallery || !galleryImageUrl) return null;

    return (
      <div 
        className="fixed inset-0 z-modal bg-black/90 flex items-center justify-center"
        onClick={closeImageGallery}
      >
        <button
          className="absolute top-4 right-4 z-10 p-2 rounded-full bg-white/10 hover:bg-white/20 text-white cursor-pointer border-0 transition-colors"
          onClick={closeImageGallery}
        >
          <Icon sprite={IconX} className="w-6 h-6" />
        </button>
        <div 
          className="flex items-center justify-center max-w-[90vw] max-h-[90vh]"
          onClick={(e) => e.stopPropagation()}
        >
          <img 
            src={galleryImageUrl}
            alt=""
            className="max-w-full max-h-[85vh] object-contain rounded-panel"
          />
        </div>
      </div>
    );
  }

  return (
    <div>
      {renderImageGallery()}
      {view === "upload" ? renderUploadView() : renderBrowseView()}
    </div>
  );
}

export default FileManagementPage
