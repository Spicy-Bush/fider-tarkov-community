import "./FileManagement.scss"
import React from "react"
import { Button, Form, Icon, Input, ImageUploader, ImageGallery } from "@fider/components"
import { HStack } from "@fider/components/layout"
import { Failure, actions, notify } from "@fider/services"
import { AdminBasePage } from "../components/AdminBasePage"
import { ImageUpload } from "@fider/models"

import IconTrash from "@fider/assets/images/heroicons-trash.svg"
import IconPencilAlt from "@fider/assets/images/heroicons-pencil-alt.svg"
import IconDownload from "@fider/assets/images/heroicons-download.svg"
import IconUpload from "@fider/assets/images/heroicons-upload.svg"
import IconEye from "@fider/assets/images/heroicons-eye.svg"
import IconX from "@fider/assets/images/heroicons-x.svg"

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

interface MediaManagementPageState {
  view: "browse" | "upload"
  assets: MediaAsset[]
  isLoading: boolean
  selectedAsset?: MediaAsset
  searchQuery: string
  assetToEdit?: MediaAsset
  newAssetName: string
  imageUpload?: ImageUpload
  uploadType: "file" | "attachment"
  error?: Failure
  showUsageModal: boolean
  mediaTypeFilter: MediaType
  pagination: {
    page: number
    pageSize: number
    total: number
    totalPages: number
  }
  sortBy: string
  sortDir: string
  confirmDeleteOpen: boolean
  assetToDelete?: MediaAsset
}

class FileManagementPage extends AdminBasePage<any, MediaManagementPageState> {
  public id = "p-admin-files"
  public name = "media"
  public title = "Media Library"
  public subtitle = "Manage your site's images, logos, avatars and attachments"
  private searchTimeout: ReturnType<typeof setTimeout> | null = null;

  constructor(props: any) {
    super(props)
  
    this.state = {
      view: "browse",
      assets: [],
      isLoading: true,
      searchQuery: "",
      newAssetName: "",
      uploadType: "file",
      showUsageModal: false,
      mediaTypeFilter: MediaType.ALL,
      pagination: {
        page: 1,
        pageSize: 20,
        total: 0,
        totalPages: 0
      },
      sortBy: "createdAt",
      sortDir: "desc",
      confirmDeleteOpen: false
    }
  }

  public async componentDidMount() {
    await this.loadAssets()
  }

  private getMediaTypeFromBlobKey(blobKey: string): MediaType {
    if (blobKey.startsWith('files/')) return MediaType.ADMIN;
    if (blobKey.startsWith('attachments/')) return MediaType.ATTACHMENT;
    if (blobKey.startsWith('avatars/')) return MediaType.AVATAR;
    if (blobKey.startsWith('logos/')) return MediaType.LOGO;
    return MediaType.ALL;
  }

  private mapApiResponseToAssets(apiAssets: any[]): MediaAsset[] {
    return apiAssets.map(asset => ({
      ...asset,
      mediaType: this.getMediaTypeFromBlobKey(asset.blobKey)
    }));
  }

  private loadAssets = async () => {
    this.setState({ isLoading: true })
    
    const params = new URLSearchParams();
    params.append('page', String(this.state.pagination.page));
    params.append('pageSize', String(this.state.pagination.pageSize));
    
    if (this.state.searchQuery) {
      params.append('search', this.state.searchQuery);
    }
    
    params.append('sortBy', this.state.sortBy);
    params.append('sortDir', this.state.sortDir);
    
    if (this.state.mediaTypeFilter !== MediaType.ALL) {
      params.append('type', this.state.mediaTypeFilter);
    }
    
    const url = `/api/v1/admin/files?${params.toString()}`;
    const result = await actions.listFiles(url);
    
    if (result.ok) {
      const assets = this.mapApiResponseToAssets(result.data.files);
      
      this.setState({ 
        assets: assets, 
        isLoading: false,
        pagination: {
          page: result.data.page,
          pageSize: result.data.pageSize,
          total: result.data.total,
          totalPages: result.data.totalPages
        }
      })
    } else {
      this.setState({ error: result.error, isLoading: false })
      notify.error("Could not load media assets")
    }
  }

  private handlePageChange = (page: number) => {
    this.setState(
      { pagination: { ...this.state.pagination, page } },
      () => this.loadAssets()
    )
  }

  // What the fuck is this, a hackathon? Why doesn't JS have a proper event throttle built in?
  // Hold on.. PM is calling, he said we can't use Lodash because the build size is already 10GB
  private handleSearchChange = (value: string) => {
    this.setState({ 
      searchQuery: value,
      pagination: { ...this.state.pagination, page: 1 } 
    });
  
    if (this.searchTimeout !== null) {
      clearTimeout(this.searchTimeout);
    }
  
    this.searchTimeout = setTimeout(() => {
      this.loadAssets();
    }, 200);
  }

  private handleSortChange = (sortBy: string) => {
    const sortDir = 
      this.state.sortBy === sortBy && this.state.sortDir === "asc" 
        ? "desc" 
        : "asc";
    
    this.setState(
      { sortBy, sortDir, pagination: { ...this.state.pagination, page: 1 } },
      () => this.loadAssets()
    )
  }

  private handleMediaTypeFilter = (mediaType: MediaType) => {
    this.setState(
      { mediaTypeFilter: mediaType, pagination: { ...this.state.pagination, page: 1 } },
      () => this.loadAssets()
    )
  }

  private handleImageUpload = async () => {
    if (!this.state.imageUpload) {
      notify.error("Please select an image to upload")
      return
    }
  
    const assetName = this.state.newAssetName.trim() || this.state.imageUpload.upload?.fileName || "unnamed image"
    
    const result = await actions.uploadFile({
      name: assetName,
      file: this.state.imageUpload,
      uploadType: this.state.uploadType
    })
  
    if (result.ok) {
      notify.success("Image uploaded successfully")
      this.setState({
        view: "browse",
        imageUpload: undefined,
        newAssetName: "",
        uploadType: "file"
      })
      await this.loadAssets()
    } else {
      this.setState({ error: result.error })
      notify.error("Failed to upload image")
    }
  }

  private openDeleteConfirmation = (asset: MediaAsset) => {
    this.setState({
      assetToDelete: asset,
      confirmDeleteOpen: true
    });
  }

  private closeDeleteConfirmation = () => {
    this.setState({
      assetToDelete: undefined,
      confirmDeleteOpen: false
    });
  }

  private deleteAsset = async (forceDelete: boolean = false) => {
    const { assetToDelete } = this.state;
    if (!assetToDelete) return;
  
    this.closeDeleteConfirmation();
  
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
      await this.loadAssets();
    } else {
      const data = await result.json();
      notify.error(data.message || "Failed to delete image");
    }
  }

  private handleEditAsset = async () => {
    if (!this.state.assetToEdit) return;

    const newName = this.state.newAssetName.trim();
    if (!newName) {
      notify.error("Name cannot be empty");
      return;
    }

    const result = await fetch(`/api/v1/admin/files/${this.state.assetToEdit.blobKey}`, {
      method: 'PUT',
      credentials: 'same-origin',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ name: newName })
    });

    if (result.ok) {
      notify.success("Image renamed successfully");
      this.setState({
        assetToEdit: undefined,
        newAssetName: ""
      });
      await this.loadAssets();
    } else {
      const data = await result.json();
      notify.error(data.message || "Failed to rename image");
    }
  }

  private handleImageSelection = (image: ImageUpload) => {
    this.setState({ imageUpload: image })
  }

  private renderUploadView() {
    return (
      <Form error={this.state.error}>
        <Input
          field="fileName"
          label="Image Name"
          placeholder="Enter a name for this image"
          value={this.state.newAssetName}
          onChange={(value) => this.setState({ newAssetName: value })}
        />
  
        <div className="c-media-library__upload-type">
          <label className="c-media-library__upload-type-label">Upload Type</label>
          <div className="c-media-library__upload-type-options">
            <label className="c-media-library__upload-type-option">
              <input
                type="radio"
                name="uploadType"
                checked={this.state.uploadType === "file"}
                onChange={() => this.setState({ uploadType: "file" })}
              />
              <span>Admin Upload (files/)</span>
            </label>
            <label className="c-media-library__upload-type-option">
              <input
                type="radio"
                name="uploadType"
                checked={this.state.uploadType === "attachment"}
                onChange={() => this.setState({ uploadType: "attachment" })}
              />
              <span>Public Attachment (attachments/)</span>
            </label>
          </div>
          <p className="c-media-library__upload-type-description">
            {this.state.uploadType === "file" 
              ? "Use this option for images managed by administrators."
              : "Use this option for images attached to posts/comments."}
          </p>
        </div>
  
        <ImageUploader
          field="imageUpload"
          label="Upload Image"
          onChange={this.handleImageSelection}
          bkey=""
        >
          <p className="text-muted">Select an image to upload. JPG, PNG, GIF, and SVG formats are supported.</p>
        </ImageUploader>
  
        <div className="mt-4">
          <HStack>
            <Button variant="primary" onClick={this.handleImageUpload}>
              Upload Image
            </Button>
            <Button variant="tertiary" onClick={() => this.setState({ view: "browse" })}>
              Cancel
            </Button>
          </HStack>
        </div>
      </Form>
    )
  }

  private renderEditView() {
    if (!this.state.assetToEdit) return null;

    return (
      <div className="c-media-library__modal">
        <div className="c-media-library__modal-backdrop" 
             onClick={() => this.setState({ assetToEdit: undefined, newAssetName: "" })}></div>
        <div className="c-media-library__modal-content">
          <h2 className="c-media-library__modal-header-title">Rename Image</h2>
          <Form error={this.state.error}>
            <Input
              field="newAssetName"
              label={`New name for "${this.state.assetToEdit.name}"`}
              placeholder="Enter new name"
              value={this.state.newAssetName}
              onChange={(value) => this.setState({ newAssetName: value })}
            />

            <div className="c-media-library__modal-actions">
              <Button 
                variant="tertiary" 
                onClick={() => this.setState({ assetToEdit: undefined, newAssetName: "" })}
              >
                Cancel
              </Button>
              <Button variant="primary" onClick={this.handleEditAsset}>
                Save
              </Button>
            </div>
          </Form>
        </div>
      </div>
    );
  }

  private renderDeleteConfirmation() {
    if (!this.state.confirmDeleteOpen || !this.state.assetToDelete) return null;

    const { assetToDelete } = this.state;
    const isInUse = assetToDelete.isInUse;

    return (
      <div className="c-media-library__modal">
        <div className="c-media-library__modal-backdrop" onClick={this.closeDeleteConfirmation}></div>
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
              onClick={this.closeDeleteConfirmation}
            >
              Cancel
            </Button>
            
            {isInUse && (
              <Button 
                variant="danger" 
                onClick={() => this.deleteAsset(true)}
              >
                Force Delete
              </Button>
            )}
            
            <Button 
              variant={isInUse ? "secondary" : "danger"} 
              onClick={() => this.deleteAsset(false)}
              disabled={isInUse}
            >
              {isInUse ? "Cannot Delete (In Use)" : "Delete"}
            </Button>
          </div>
        </div>
      </div>
    );
  }

  private startEdit = (asset: MediaAsset) => {
    this.setState({
      assetToEdit: asset,
      newAssetName: asset.name
    });
  }

  private downloadAsset = (asset: MediaAsset) => {
    const url = this.getAssetURL(asset);
    window.open(url, "_blank");
  }

  private getAssetURL = (asset: MediaAsset): string => {
    if (asset.blobKey.startsWith('logos/')) {
      return `/static/favicon/${asset.blobKey.replace('logos/', '')}`;
    } else if (asset.blobKey.startsWith('avatars/')) {
      return `/static/avatars/gravatar/${asset.blobKey.replace('avatars/', '')}`;
    }
    return `/static/images/${asset.blobKey}`;
  }

  private formatFileSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }

  private getMediaTypeLabel(mediaType: MediaType): string {
    switch(mediaType) {
      case MediaType.ADMIN: return "Admin";
      case MediaType.ATTACHMENT: return "Attachment";
      case MediaType.AVATAR: return "Avatar";
      case MediaType.LOGO: return "Logo";
      default: return "Unknown";
    }
  }

  private getMediaTypeBadgeClass(mediaType: MediaType): string {
    switch(mediaType) {
      case MediaType.ADMIN: return "c-media-library__badge--admin";
      case MediaType.ATTACHMENT: return "c-media-library__badge--attachment";
      case MediaType.AVATAR: return "c-media-library__badge--avatar";
      case MediaType.LOGO: return "c-media-library__badge--logo";
      default: return "";
    }
  }

  private showAssetUsage = (asset: MediaAsset) => {
    this.setState({ 
      selectedAsset: asset,
      showUsageModal: true 
    });
  }

  private renderUsageModal() {
    const { selectedAsset, showUsageModal } = this.state;
    if (!selectedAsset || !showUsageModal) return null;

    return (
      <div className="c-media-library__modal">
        <div className="c-media-library__modal-backdrop"
             onClick={() => this.setState({ showUsageModal: false })}></div>
        <div className="c-media-library__modal-content">
          <div className="c-media-library__modal-header">
            <h2 className="c-media-library__modal-header-title">Image Usage</h2>
            <Button 
              variant="tertiary" 
              size="small" 
              onClick={() => this.setState({ showUsageModal: false })}
            >
              <Icon sprite={IconX} />
            </Button>
          </div>
          
          <div className="c-media-library__modal-section">
            <div className="c-media-library__modal-section-label">Image:</div>
            <div className="c-media-library__modal-section-value">{selectedAsset.name}</div>
            <span className={`c-media-library__badge ${this.getMediaTypeBadgeClass(selectedAsset.mediaType)}`}>
              {this.getMediaTypeLabel(selectedAsset.mediaType)}
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
              onClick={() => this.setState({ showUsageModal: false })}
            >
              Close
            </Button>
          </div>
        </div>
      </div>
    );
  }

  private renderMediaGrid() {
    const { assets, isLoading } = this.state;

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
            {this.state.searchQuery || this.state.mediaTypeFilter !== MediaType.ALL
              ? "Try different search or filter settings"
              : ""}
          </p>
          {(this.state.searchQuery || this.state.mediaTypeFilter !== MediaType.ALL) && (
            <Button
              variant="secondary"
              onClick={() => {
                this.setState({
                  searchQuery: "",
                  mediaTypeFilter: MediaType.ALL,
                  pagination: { ...this.state.pagination, page: 1 }
                }, () => this.loadAssets())
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
        {assets.map(asset => this.renderMediaCard(asset))}
      </div>
    );
  }
  
  private renderMediaCard(asset: MediaAsset) {
    const badgeClass = this.getMediaTypeBadgeClass(asset.mediaType);
    const typeLabel = this.getMediaTypeLabel(asset.mediaType);
    
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
            {this.formatFileSize(asset.size)}
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
          
          {/* Actions */}
          <div className="c-media-library__card-info-actions">
            <Button 
              size="small" 
              variant="tertiary" 
              onClick={() => this.downloadAsset(asset)} 
              className="c-media-library__card-info-actions-btn"
            >
              <Icon sprite={IconDownload} />
            </Button>
            
            {asset.isInUse && (
              <Button 
                size="small" 
                variant="tertiary" 
                onClick={() => this.showAssetUsage(asset)} 
                className="c-media-library__card-info-actions-btn"
              >
                <Icon sprite={IconEye} />
              </Button>
            )}
            
            <Button 
              size="small" 
              variant="tertiary" 
              onClick={() => this.startEdit(asset)} 
              className="c-media-library__card-info-actions-btn"
            >
              <Icon sprite={IconPencilAlt} />
            </Button>
            
            <Button 
              size="small" 
              variant="tertiary" 
              onClick={() => this.openDeleteConfirmation(asset)} 
              className="c-media-library__card-info-actions-btn c-media-library__card-info-actions-btn--delete"
            >
              <Icon sprite={IconTrash} />
            </Button>
          </div>
        </div>
      </div>
    );
  }

  private renderBrowseView() {
    if (this.state.assetToEdit) {
      return this.renderEditView();
    }

    const { pagination } = this.state;

    return (
      <>
        {this.renderUsageModal()}
        {this.renderDeleteConfirmation()}
        
        <div className="c-media-library__header">
          <div className="c-media-library__header-search">
            <div className="c-media-library__header-search-icon">
              <Icon sprite={IconEye} />
            </div>
            <Input
              field="search"
              placeholder="Search images..."
              value={this.state.searchQuery}
              onChange={this.handleSearchChange}
            />
          </div>
          
          <Button variant="primary" onClick={() => this.setState({ view: "upload" })}>
            <Icon sprite={IconUpload} />
            <span>Upload New Image</span>
          </Button>
        </div>
        
        <div className="c-media-library__filters">
          <div className="c-media-library__filters-section">
            <span className="c-media-library__filters-section-label">Type:</span>
            <Button 
              variant={this.state.mediaTypeFilter === MediaType.ALL ? "primary" : "tertiary"} 
              size="small"
              onClick={() => this.handleMediaTypeFilter(MediaType.ALL)}
              className="c-media-library__filters-btn"
            >
              All
            </Button>
            
            <Button 
              variant={this.state.mediaTypeFilter === MediaType.ADMIN ? "primary" : "tertiary"} 
              size="small"
              onClick={() => this.handleMediaTypeFilter(MediaType.ADMIN)}
              className="c-media-library__filters-btn"
            >
              Admin Uploads
            </Button>
            
            <Button 
              variant={this.state.mediaTypeFilter === MediaType.ATTACHMENT ? "primary" : "tertiary"} 
              size="small"
              onClick={() => this.handleMediaTypeFilter(MediaType.ATTACHMENT)}
              className="c-media-library__filters-btn"
            >
              Attachments
            </Button>
            
            <Button 
              variant={this.state.mediaTypeFilter === MediaType.AVATAR ? "primary" : "tertiary"} 
              size="small"
              onClick={() => this.handleMediaTypeFilter(MediaType.AVATAR)}
              className="c-media-library__filters-btn"
            >
              Avatars
            </Button>
            
            <Button 
              variant={this.state.mediaTypeFilter === MediaType.LOGO ? "primary" : "tertiary"} 
              size="small"
              onClick={() => this.handleMediaTypeFilter(MediaType.LOGO)}
              className="c-media-library__filters-btn"
            >
              Logos
            </Button>
          </div>
          
          <div className="c-media-library__filters-section">
            <span className="c-media-library__filters-section-label">Sort by:</span>
            <Button 
              variant={this.state.sortBy === "name" ? "primary" : "tertiary"} 
              size="small"
              onClick={() => this.handleSortChange("name")}
              className="c-media-library__filters-btn"
            >
              Name 
              {this.state.sortBy === "name" && (
                <span className="c-media-library__filters-btn-sort-icon">
                  {this.state.sortDir === "asc" ? "↑" : "↓"}
                </span>
              )}
            </Button>
            
            <Button 
              variant={this.state.sortBy === "size" ? "primary" : "tertiary"} 
              size="small"
              onClick={() => this.handleSortChange("size")}
              className="c-media-library__filters-btn"
            >
              Size 
              {this.state.sortBy === "size" && (
                <span className="c-media-library__filters-btn-sort-icon">
                  {this.state.sortDir === "asc" ? "↑" : "↓"}
                </span>
              )}
            </Button>
            
            <Button 
              variant={this.state.sortBy === "createdAt" ? "primary" : "tertiary"} 
              size="small"
              onClick={() => this.handleSortChange("createdAt")}
              className="c-media-library__filters-btn"
            >
              Date 
              {this.state.sortBy === "createdAt" && (
                <span className="c-media-library__filters-btn-sort-icon">
                  {this.state.sortDir === "asc" ? "↑" : "↓"}
                </span>
              )}
            </Button>
            
            <Button 
              variant="secondary" 
              size="small"
              onClick={this.loadAssets}
              className="c-media-library__filters-section-auto"
            >
              Refresh
            </Button>
          </div>
        </div>
        
        {this.renderMediaGrid()}

        {pagination.totalPages > 1 && (
          <Pagination
            currentPage={pagination.page}
            totalPages={pagination.totalPages}
            onPageChange={this.handlePageChange}
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

  public content() {
    return (
      <div className="c-media-library">
        {this.state.view === "upload" ? this.renderUploadView() : this.renderBrowseView()}
      </div>
    );
  }
}

export default FileManagementPage