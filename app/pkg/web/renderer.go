package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/assets"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/i18n"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/tpl"
)

type clientAssets struct {
	CSS []string
	JS  []string
}

type viteChunk struct {
	File           string   `json:"file"`
	Name           string   `json:"name,omitempty"`
	Src            string   `json:"src,omitempty"`
	IsEntry        bool     `json:"isEntry,omitempty"`
	IsDynamicEntry bool     `json:"isDynamicEntry,omitempty"`
	CSS            []string `json:"css,omitempty"`
	Imports        []string `json:"imports,omitempty"`
	DynamicImports []string `json:"dynamicImports,omitempty"`
}

type viteManifest map[string]viteChunk

const (
	viteEntryPoint  = "public/index.tsx"
	assetsURLPrefix = "/assets/"
)

type Renderer struct {
	assets        *clientAssets
	chunkedAssets map[string]*clientAssets
	reactRenderer *ReactRenderer
}

func NewRenderer() *Renderer {
	reactRenderer, err := NewReactRenderer("ssr.js")
	if err != nil {
		panic(errors.Wrap(err, "failed to initialize SSR renderer"))
	}

	r := &Renderer{
		reactRenderer: reactRenderer,
	}

	if err := r.loadAssets(); err != nil {
		panic(errors.Wrap(err, "failed to load assets"))
	}

	return r
}

func (r *Renderer) loadAssets() error {
	manifestPath := "dist/.vite/manifest.json"

	jsonBytes, err := fs.ReadFile(assets.FS, manifestPath)
	if err != nil {
		return errors.Wrap(err, "failed to read file: manifest.json")
	}

	manifest := make(viteManifest)
	if err = json.Unmarshal(jsonBytes, &manifest); err != nil {
		return errors.Wrap(err, "failed to parse file: manifest.json")
	}

	if _, exists := manifest[viteEntryPoint]; !exists {
		return errors.New("entry point %s not found in manifest", viteEntryPoint)
	}
	entryKey := viteEntryPoint

	r.assets = r.collectChunkAssets(manifest, entryKey, make(map[string]bool))

	r.chunkedAssets = make(map[string]*clientAssets)
	for key, chunk := range manifest {
		if chunk.IsDynamicEntry {
			chunkName := convertSourceToChunkName(key)
			visited := make(map[string]bool)
			visited[entryKey] = true
			for _, imp := range manifest[entryKey].Imports {
				visited[imp] = true
			}
			r.chunkedAssets[chunkName] = r.collectChunkAssets(manifest, key, visited)
		}
	}

	return nil
}

func (r *Renderer) collectChunkAssets(manifest viteManifest, key string, visited map[string]bool) *clientAssets {
	assets := &clientAssets{
		CSS: make([]string, 0),
		JS:  make([]string, 0),
	}

	r.collectAssetsRecursive(manifest, key, visited, assets)
	return assets
}

func (r *Renderer) collectAssetsRecursive(manifest viteManifest, key string, visited map[string]bool, assets *clientAssets) {
	if visited[key] {
		return
	}
	visited[key] = true

	chunk, exists := manifest[key]
	if !exists {
		return
	}

	for _, imp := range chunk.Imports {
		r.collectAssetsRecursive(manifest, imp, visited, assets)
	}

	for _, css := range chunk.CSS {
		assets.CSS = append(assets.CSS, assetsURLPrefix+css)
	}

	if chunk.File != "" && strings.HasSuffix(chunk.File, ".js") {
		assets.JS = append(assets.JS, assetsURLPrefix+chunk.File)
	}
}

func convertSourceToChunkName(src string) string {
	if strings.HasPrefix(src, "locale/") && strings.HasSuffix(src, "/client.json") {
		parts := strings.Split(src, "/")
		if len(parts) >= 2 {
			locale := parts[1]
			return fmt.Sprintf("locale-%s-client-json", locale)
		}
	}

	if strings.HasPrefix(src, "public/pages/") && strings.HasSuffix(src, ".tsx") {
		pagePath := strings.TrimPrefix(src, "public/pages/")
		pagePath = strings.TrimSuffix(pagePath, ".tsx")
		pagePath = strings.ReplaceAll(pagePath, ".", "-")
		pagePath = strings.ReplaceAll(pagePath, "/", "-")
		return pagePath
	}

	if strings.HasPrefix(src, "public/services/") && strings.HasSuffix(src, ".tsx") {
		name := filepath.Base(src)
		name = strings.TrimSuffix(name, ".tsx")
		return name
	}

	name := strings.ReplaceAll(src, "/", "-")
	name = strings.ReplaceAll(name, ".", "-")
	return name
}

type userStandingInfo struct {
	hasWarning      bool
	isMuted         bool
	latestWarningID int
	latestMuteID    int
}

var cachedOAuthProviders atomic.Pointer[[]*dto.OAuthProviderOption]
var cachedNavigationLinks atomic.Pointer[sync.Map]

func init() {
	cachedNavigationLinks.Store(&sync.Map{})
}

func InvalidateOAuthCache() {
	cachedOAuthProviders.Store(nil)
}

func InvalidateNavigationLinksCache() {
	cachedNavigationLinks.Store(&sync.Map{})
}

func getNavigationLinks(ctx *Context, tenantID int) []*entity.NavigationLink {
	cache := cachedNavigationLinks.Load()
	if cached, ok := cache.Load(tenantID); ok {
		return cached.([]*entity.NavigationLink)
	}

	navLinks := &query.GetNavigationLinks{}
	if err := bus.Dispatch(ctx, navLinks); err != nil {
		return nil
	}

	cache.Store(tenantID, navLinks.Result)
	return navLinks.Result
}

func getOAuthProviders(ctx *Context) []*dto.OAuthProviderOption {
	if ctx.IsAuthenticated() {
		return nil
	}

	if providers := cachedOAuthProviders.Load(); providers != nil {
		return *providers
	}

		providers := &query.ListActiveOAuthProviders{
			Result: make([]*dto.OAuthProviderOption, 0),
		}
		if err := bus.Dispatch(ctx, providers); err != nil {
		return nil
	}

	cachedOAuthProviders.Store(&providers.Result)
	return providers.Result
}

func getUserStandingInfo(ctx *Context, userID int) userStandingInfo {
	result := userStandingInfo{}

	standing := &query.GetUserProfileStanding{
		UserID: userID,
		Result: struct {
			Warnings []struct {
				ID        int        `json:"id"`
				Reason    string     `json:"reason"`
				CreatedAt time.Time  `json:"createdAt"`
				ExpiresAt *time.Time `json:"expiresAt,omitempty"`
			} `json:"warnings"`
			Mutes []struct {
				ID        int        `json:"id"`
				Reason    string     `json:"reason"`
				CreatedAt time.Time  `json:"createdAt"`
				ExpiresAt *time.Time `json:"expiresAt,omitempty"`
			} `json:"mutes"`
		}{},
	}
	if err := bus.Dispatch(ctx, standing); err != nil {
		return result
	}

	now := time.Now()
	for _, warning := range standing.Result.Warnings {
		if warning.ExpiresAt == nil || warning.ExpiresAt.After(now) {
			result.hasWarning = true
			if result.latestWarningID == 0 {
				result.latestWarningID = warning.ID
			}
			break
		}
	}
	for _, mute := range standing.Result.Mutes {
		if mute.ExpiresAt == nil || mute.ExpiresAt.After(now) {
			result.isMuted = true
			if result.latestMuteID == 0 {
				result.latestMuteID = mute.ID
			}
			break
		}
	}

	return result
}

func (r *Renderer) Render(w io.Writer, statusCode int, props Props, ctx *Context) {
	public := make(Map)
	private := make(Map)
	if props.Data == nil {
		props.Data = make(Map)
	}

	tenant := ctx.Tenant()
	tenantName := "Fider"
	if tenant != nil {
		tenantName = tenant.Name
	}

	title := tenantName
	if props.Title != "" {
		title = fmt.Sprintf("%s - %s", props.Title, tenantName)
	}

	public["title"] = title

	if props.Description != "" {
		description := strings.Replace(props.Description, "\n", " ", -1)
		public["description"] = fmt.Sprintf("%.150s", description)
	}

	private["assets"] = r.assets
	private["logo"] = LogoURL(ctx)

	locale := i18n.GetLocale(ctx)
	localeChunkName := fmt.Sprintf("locale-%s-client-json", locale)
	pageChunkName := strings.ReplaceAll(strings.ReplaceAll(props.Page, ".", "-"), "/", "-")

	private["preloadAssets"] = []*clientAssets{
		r.chunkedAssets[localeChunkName],
		r.chunkedAssets[pageChunkName],
	}

	if tenant == nil || tenant.LogoBlobKey == "" {
		private["favicon"] = AssetsURL(ctx, "/static/favicon")
	} else {
		private["favicon"] = AssetsURL(ctx, "/static/favicon/%s", tenant.LogoBlobKey)
	}

	private["currentURL"] = ctx.Request.URL.String()
	if canonicalURL := ctx.Value("Canonical-URL"); canonicalURL != nil {
		private["canonicalURL"] = canonicalURL
	}

	var oauthProviders []*dto.OAuthProviderOption
	if statusCode >= 200 && statusCode < 500 {
		oauthProviders = getOAuthProviders(ctx)
	}

	var navigationLinks interface{}
	if tenant != nil {
		navigationLinks = getNavigationLinks(ctx, tenant.ID)
	}

	public["page"] = props.Page
	public["contextID"] = ctx.ContextID()
	public["sessionID"] = ctx.SessionID()
	public["tenant"] = tenant
	public["props"] = props.Data
	public["settings"] = &Map{
		"mode":             env.Config.HostMode,
		"locale":           locale,
		"environment":      env.Config.Environment,
		"googleAnalytics":  env.Config.GoogleAnalytics,
		"googleAdSense":    env.Config.GoogleAdSense,
		"domain":           env.MultiTenantDomain(),
		"hasLegal":         env.HasLegal(),
		"isBillingEnabled": env.IsBillingEnabled(),
		"baseURL":          ctx.BaseURL(),
		"assetsURL":        AssetsURL(ctx, ""),
		"oauth":            oauthProviders,
		"navigationLinks":  navigationLinks,
	}

	if ctx.IsAuthenticated() {
		u := ctx.User()
		standing := getUserStandingInfo(ctx, u.ID)
		public["user"] = &Map{
			"id":              u.ID,
			"name":            u.Name,
			"email":           u.Email,
			"role":            u.Role,
			"visualRole":      u.VisualRole,
			"status":          u.Status,
			"avatarType":      u.AvatarType,
			"avatarURL":       u.AvatarURL,
			"avatarBlobKey":   u.AvatarBlobKey,
			"isAdministrator": u.IsAdministrator(),
			"isCollaborator":  u.IsCollaborator(),
			"isModerator":     u.IsModerator(),
			"isHelper":        u.IsHelper(),
			"hasWarning":      standing.hasWarning,
			"isMuted":         standing.isMuted,
			"latestWarningId": standing.latestWarningID,
			"latestMuteId":    standing.latestMuteID,
		}
	}

	templateName := "index.html"

	if ctx.Request.IsVerifiedCrawler() {
		html, err := r.reactRenderer.Render(ctx.Request.URL, public)
		if err != nil {
			log.Errorf(ctx, "Failed to render react page: @{Error}", dto.Props{
				"Error": err.Error(),
			})
		}
		if html != "" {
			templateName = "ssr.html"
			props.Data["html"] = template.HTML(html)
		}
	}

	tmpl := tpl.GetTemplate("/views/base.html", fmt.Sprintf("/views/%s", templateName))

	err := tpl.Render(ctx, tmpl, w, Map{
		"public":  public,
		"private": private,
	})
	if err != nil {
		panic(errors.Wrap(err, "failed to execute template %s", templateName))
	}
}
