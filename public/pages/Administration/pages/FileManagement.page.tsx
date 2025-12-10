import "./FileManagement.scss"
import React, { useState, useEffect, useRef, useCallback } from "react"
import { Button, Form, Icon, Input, ImageUploader, ImageGallery } from "@fider/components"
import { HStack } from "@fider/components/layout"
import { Failure, actions, notify } from "@fider/services"
import { ImageUpload } from "@fider/models"
import { PageConfig } from "@fider/components/layouts"

import { heroiconsTrash as IconTrash, heroiconsPencilAlt as IconPencilAlt, heroiconsDownload as IconDownload, heroiconsUpload as IconUpload, heroiconsEye as IconEye, heroiconsX as IconX } from "@fider/icons.generated"

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
    <div className="c-loader">
      <div className="c-loader__spinner"></div>
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

  const handlePrevious = () => {
    if (currentPage > 1) {
      onPageChange(currentPage - 1);
    }
  };

  const handleNext = () => {
    if (currentPage < totalPages) {
      onPageChange(currentPage + 1);
    }
  };

  return (
    <div className="c-media-library__pagination">
      <Button
        variant="tertiary"
        size="small"
        onClick={handlePrevious}
        disabled={currentPage === 1}
        className="c-media-library__pagination-btn"
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
            className="c-media-library__pagination-btn c-media-library__pagination-btn--page"
          >
            {page}
          </Button>
        );
      })}
      
      <Button
        variant="tertiary"
        size="small"
        onClick={handleNext}
        disabled={currentPage === totalPages}
        className="c-media-library__pagination-btn"
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
    return `/static/favicon/${asset.blobKey.replace('logos/', '')}`;
  } else if (asset.blobKey.startsWith('avatars/')) {
    return `/static/avatars/gravatar/${asset.blobKey.replace('avatars/', '')}`;
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

const getMediaTypeBadgeClass = (mediaType: MediaType): string => {
  switch(mediaType) {
    case MediaType.ADMIN: return "c-media-library__badge--admin";
    case MediaType.ATTACHMENT: return "c-media-library__badge--attachment";
    case MediaType.AVATAR: return "c-media-library__badge--avatar";
    case MediaType.LOGO: return "c-media-library__badge--logo";
    default: return "";
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
  
        <div className="c-media-library__upload-type">
          <label className="c-media-library__upload-type-label">Upload Type</label>
          <div className="c-media-library__upload-type-options">
            <label className="c-media-library__upload-type-option">
              <input
                type="radio"
                name="uploadType"
                checked={uploadType === "file"}
                onChange={() => setUploadType("file")}
              />
              <span>Admin Upload (files/)</span>
            </label>
            <label className="c-media-library__upload-type-option">
              <input
                type="radio"
                name="uploadType"
                checked={uploadType === "attachment"}
                onChange={() => setUploadType("attachment")}
              />
              <span>Public Attachment (attachments/)</span>
            </label>
          </div>
          <p className="c-media-library__upload-type-description">
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
      <div className="c-media-library__modal">
        <div className="c-media-library__modal-backdrop" 
             onClick={() => { setAssetToEdit(undefined); setNewAssetName(""); }}></div>
        <div className="c-media-library__modal-content">
          <h2 className="c-media-library__modal-header-title">Rename Image</h2>
          <Form error={error}>
            <Input
              field="newAssetName"
              label={`New name for "${assetToEdit.name}"`}
              placeholder="Enter new name"
              value={newAssetName}
              onChange={(value) => setNewAssetName(value)}
            />

            <div className="c-media-library__modal-actions">
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
      <div className="c-media-library__modal">
        <div className="c-media-library__modal-backdrop" onClick={closeDeleteConfirmation}></div>
        <div className="c-media-library__modal-content">
          <h2 className="c-media-library__modal-header-title">Delete Image</h2>
          
          <div className="c-media-library__modal-section">
            <p className="mb-2">Are you sure you want to delete <strong>{assetToDelete.name}</strong>?</p>
            
            {isInUse && (
              <div className="c-media-library__modal-warning">
                <p className="c-media-library__modal-warning-title">Warning: This image is in use</p>
                <p className="c-media-library__modal-warning-desc">Forcing deletion will remove all references to this image from posts, comments, avatars, or logos.</p>
              </div>
            )}
          </div>
          
          <div className="c-media-library__modal-actions">
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

  const renderUsageModal = () => {
    if (!selectedAsset || !showUsageModal) return null;

    return (
      <div className="c-media-library__modal">
        <div className="c-media-library__modal-backdrop"
             onClick={() => setShowUsageModal(false)}></div>
        <div className="c-media-library__modal-content">
          <div className="c-media-library__modal-header">
            <h2 className="c-media-library__modal-header-title">Image Usage</h2>
            <Button 
              variant="tertiary" 
              size="small" 
              onClick={() => setShowUsageModal(false)}
            >
              <Icon sprite={IconX} />
            </Button>
          </div>
          
          <div className="c-media-library__modal-section">
            <div className="c-media-library__modal-section-label">Image:</div>
            <div className="c-media-library__modal-section-value">{selectedAsset.name}</div>
            <span className={`c-media-library__badge ${getMediaTypeBadgeClass(selectedAsset.mediaType)}`}>
              {getMediaTypeLabel(selectedAsset.mediaType)}
            </span>
          </div>
          
          <div className="c-media-library__modal-section">
            <div className="c-media-library__modal-section-label">Used in:</div>
            <div className="c-media-library__modal-usage">
              {selectedAsset.usedIn && selectedAsset.usedIn.length > 0 ? (
                <ul className="c-media-library__modal-usage-list">
                  {selectedAsset.usedIn.map((usage: string, index: number) => (
                    <li key={index} className="c-media-library__modal-usage-list-item">{usage}</li>
                  ))}
                </ul>
              ) : (
                <p className="c-media-library__modal-usage-empty">This image is not currently used anywhere.</p>
              )}
            </div>
          </div>
          
          <div className="c-media-library__modal-actions">
            <Button 
              variant="tertiary" 
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
    const badgeClass = getMediaTypeBadgeClass(asset.mediaType);
    const typeLabel = getMediaTypeLabel(asset.mediaType);
    
    return (
      <div key={asset.blobKey} className="c-media-library__card">
        <div className="c-media-library__card-preview">
          <div className="c-media-library__card-preview-image">
            <ImageGallery bkeys={[asset.blobKey]} />
          </div>
          
          <div className="c-media-library__card-preview-badges">
            <span className={`c-media-library__badge ${badgeClass}`}>
              {typeLabel}
            </span>
            
            {asset.isInUse && (
              <span className="c-media-library__badge c-media-library__badge--in-use">
                In Use
              </span>
            )}
          </div>
          
          <div className="c-media-library__card-preview-size">
            {formatFileSize(asset.size)}
          </div>
        </div>
        
        <div className="c-media-library__card-info">
          <div className="c-media-library__card-info-meta">
            <span className="c-media-library__card-info-meta-name" title={asset.name}>
              {asset.name}
            </span>
            <span className="c-media-library__card-info-meta-date">
              {new Date(asset.createdAt).toLocaleDateString()}
            </span>
          </div>
          
          <div className="c-media-library__card-info-actions">
            <Button 
              size="small" 
              variant="tertiary" 
              onClick={() => downloadAsset(asset)} 
              className="c-media-library__card-info-actions-btn"
            >
              <Icon sprite={IconDownload} />
            </Button>
            
            {asset.isInUse && (
              <Button 
                size="small" 
                variant="tertiary" 
                onClick={() => showAssetUsage(asset)} 
                className="c-media-library__card-info-actions-btn"
              >
                <Icon sprite={IconEye} />
              </Button>
            )}
            
            <Button 
              size="small" 
              variant="tertiary" 
              onClick={() => startEdit(asset)} 
              className="c-media-library__card-info-actions-btn"
            >
              <Icon sprite={IconPencilAlt} />
            </Button>
            
            <Button 
              size="small" 
              variant="tertiary" 
              onClick={() => openDeleteConfirmation(asset)} 
              className="c-media-library__card-info-actions-btn c-media-library__card-info-actions-btn--delete"
            >
              <Icon sprite={IconTrash} />
            </Button>
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
        <div className="c-media-library__empty">
          <div className="c-media-library__empty-icon">
            <Icon sprite={IconEye} />
          </div>
          <p className="c-media-library__empty-title">No images found</p>
          <p className="c-media-library__empty-subtitle">
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
      <div className="c-media-library__grid">
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
        
        <div className="c-media-library__header">
          <div className="c-media-library__header-search">
            <div className="c-media-library__header-search-icon">
              <Icon sprite={IconEye} />
            </div>
            <Input
              field="search"
              placeholder="Search images..."
              value={searchQuery}
              onChange={handleSearchChange}
            />
          </div>
          
          <Button variant="primary" onClick={() => setView("upload")}>
            <Icon sprite={IconUpload} />
            <span>Upload New Image</span>
          </Button>
        </div>
        
        <div className="c-media-library__filters">
          <div className="c-media-library__filters-section">
            <span className="c-media-library__filters-section-label">Type:</span>
            <Button 
              variant={mediaTypeFilter === MediaType.ALL ? "primary" : "tertiary"} 
              size="small"
              onClick={() => handleMediaTypeFilter(MediaType.ALL)}
              className="c-media-library__filters-btn"
            >
              All
            </Button>
            
            <Button 
              variant={mediaTypeFilter === MediaType.ADMIN ? "primary" : "tertiary"} 
              size="small"
              onClick={() => handleMediaTypeFilter(MediaType.ADMIN)}
              className="c-media-library__filters-btn"
            >
              Admin Uploads
            </Button>
            
            <Button 
              variant={mediaTypeFilter === MediaType.ATTACHMENT ? "primary" : "tertiary"} 
              size="small"
              onClick={() => handleMediaTypeFilter(MediaType.ATTACHMENT)}
              className="c-media-library__filters-btn"
            >
              Attachments
            </Button>
            
            <Button 
              variant={mediaTypeFilter === MediaType.AVATAR ? "primary" : "tertiary"} 
              size="small"
              onClick={() => handleMediaTypeFilter(MediaType.AVATAR)}
              className="c-media-library__filters-btn"
            >
              Avatars
            </Button>
            
            <Button 
              variant={mediaTypeFilter === MediaType.LOGO ? "primary" : "tertiary"} 
              size="small"
              onClick={() => handleMediaTypeFilter(MediaType.LOGO)}
              className="c-media-library__filters-btn"
            >
              Logos
            </Button>
          </div>
          
          <div className="c-media-library__filters-section">
            <span className="c-media-library__filters-section-label">Sort by:</span>
            <Button 
              variant={sortBy === "name" ? "primary" : "tertiary"} 
              size="small"
              onClick={() => handleSortChange("name")}
              className="c-media-library__filters-btn"
            >
              Name 
              {sortBy === "name" && (
                <span className="c-media-library__filters-btn-sort-icon">
                  {sortDir === "asc" ? "^" : "v"}
                </span>
              )}
            </Button>
            
            <Button 
              variant={sortBy === "size" ? "primary" : "tertiary"} 
              size="small"
              onClick={() => handleSortChange("size")}
              className="c-media-library__filters-btn"
            >
              Size 
              {sortBy === "size" && (
                <span className="c-media-library__filters-btn-sort-icon">
                  {sortDir === "asc" ? "^" : "v"}
                </span>
              )}
            </Button>
            
            <Button 
              variant={sortBy === "createdAt" ? "primary" : "tertiary"} 
              size="small"
              onClick={() => handleSortChange("createdAt")}
              className="c-media-library__filters-btn"
            >
              Date 
              {sortBy === "createdAt" && (
                <span className="c-media-library__filters-btn-sort-icon">
                  {sortDir === "asc" ? "^" : "v"}
                </span>
              )}
            </Button>
            
            <Button 
              variant="secondary" 
              size="small"
              onClick={loadAssets}
              className="c-media-library__filters-section-auto"
            >
              Refresh
            </Button>
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
          <div className="c-media-library__counter">
            Showing {Math.min((pagination.page - 1) * pagination.pageSize + 1, pagination.total)} 
            -{Math.min(pagination.page * pagination.pageSize, pagination.total)} of {pagination.total} image
            {pagination.total !== 1 ? 's' : ''}
          </div>
        )}
      </>
    );
  }

  return (
    <div className="c-media-library">
      {view === "upload" ? renderUploadView() : renderBrowseView()}
    </div>
  );
}

export default FileManagementPage
