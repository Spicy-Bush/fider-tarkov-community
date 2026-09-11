package apiv1

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/adsselect"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

// MaxSelectAdsSlots is the hard cap on POST /api/v1/ads/select batch size.
const MaxSelectAdsSlots = 32

type selectAdsSlot struct {
	InstanceID  string `json:"instanceId"`
	PlacementID string `json:"placementId"`
}

type selectAdsRequest struct {
	Slots []selectAdsSlot `json:"slots"`
}

// SelectAds handles POST /api/v1/ads/select?locale=
// Pipeline: validate -> candidates SQL -> adsselect -> batch creatives -> PublicAd map.
// Blank advertiser -> null (#39).
func SelectAds() web.HandlerFunc {
	return func(c *web.Context) error {
		locale := c.QueryParam("locale")
		if locale == "" {
			locale = "all"
		}
		req := selectAdsRequest{}
		if err := c.Bind(&req); err != nil {
			return c.BadRequest(web.Map{"message": "Invalid request body"})
		}
		if len(req.Slots) > MaxSelectAdsSlots {
			return c.BadRequest(web.Map{"message": fmt.Sprintf("Too many slots (max %d)", MaxSelectAdsSlots)})
		}

		seenInstance := map[string]bool{}
		instances := make([]adsselect.InstanceReq, 0, len(req.Slots))
		for _, s := range req.Slots {
			iid := strings.TrimSpace(s.InstanceID)
			pid := strings.TrimSpace(s.PlacementID)
			if iid == "" || pid == "" {
				return c.BadRequest(web.Map{"message": "instanceId and placementId are required for every slot"})
			}
			if seenInstance[iid] {
				return c.BadRequest(web.Map{"message": "duplicate instanceId: " + iid})
			}
			seenInstance[iid] = true
			if !entity.IsCatalogPlacementID(pid) {
				return c.BadRequest(web.Map{"message": "unknown placementId: " + pid})
			}
			instances = append(instances, adsselect.InstanceReq{InstanceID: iid, PlacementID: pid})
		}

		out, err := runAdSelection(c, instances, locale, time.Now().UTC(), rand.New(rand.NewSource(time.Now().UnixNano())))
		if err != nil {
			return c.Failure(err)
		}
		return c.Ok(out)
	}
}

func runAdSelection(
	c *web.Context,
	instances []adsselect.InstanceReq,
	locale string,
	now time.Time,
	rng *rand.Rand,
) (map[string]*entity.PublicAd, error) {
	out := make(map[string]*entity.PublicAd, len(instances))
	for _, item := range instances {
		out[item.InstanceID] = nil
	}
	if len(instances) == 0 {
		return out, nil
	}

	placementIDs := make([]string, 0, len(instances))
	seen := map[string]bool{}
	for _, item := range instances {
		if seen[item.PlacementID] {
			continue
		}
		seen[item.PlacementID] = true
		placementIDs = append(placementIDs, item.PlacementID)
	}

	candQ := &query.GetActiveAdCandidates{
		PlacementIDs: placementIDs,
		Locale:       locale,
		Now:          now,
	}
	if err := bus.Dispatch(c, candQ); err != nil {
		return nil, err
	}

	byPlacement := map[string][]adsselect.Candidate{}
	for _, cand := range candQ.Result {
		byPlacement[cand.PlacementID] = append(byPlacement[cand.PlacementID], cand)
	}

	picked := adsselect.SelectForInstances(instances, byPlacement, rng)

	versionIDs := make([]int, 0)
	vidSeen := map[int]bool{}
	for _, cand := range picked {
		if cand == nil {
			continue
		}
		if vidSeen[cand.CreativeVersionID] {
			continue
		}
		vidSeen[cand.CreativeVersionID] = true
		versionIDs = append(versionIDs, cand.CreativeVersionID)
	}

	verQ := &query.GetCreativeVersionsByIDs{IDs: versionIDs}
	if err := bus.Dispatch(c, verQ); err != nil {
		return nil, err
	}

	for instanceID, cand := range picked {
		if cand == nil {
			out[instanceID] = nil
			continue
		}
		if strings.TrimSpace(cand.Advertiser) == "" {
			out[instanceID] = nil // #39
			continue
		}
		ver := verQ.Result[cand.CreativeVersionID]
		if ver == nil {
			out[instanceID] = nil
			continue
		}
		out[instanceID] = &entity.PublicAd{
			CampaignID:        cand.CampaignID,
			Advertiser:        cand.Advertiser,
			PlacementID:       cand.PlacementID,
			CreativeVersionID: cand.CreativeVersionID,
			ImageURL:          ver.ImageURL,
			HTML:              ver.HTML,
			ClickPath:         fmt.Sprintf("/ads/click/%d?v=%d", cand.CampaignID, cand.CreativeVersionID),
		}
	}
	return out, nil
}

// ListAdPlacements returns the global placement catalog (admin + editor).
func ListAdPlacements() web.HandlerFunc {
	return func(c *web.Context) error {
		q := &query.ListAdPlacements{}
		if err := bus.Dispatch(c, q); err != nil {
			return c.Failure(err)
		}
		if q.Result == nil {
			q.Result = []*entity.AdPlacement{}
		}
		return c.Ok(q.Result)
	}
}

// PublicAdPlacementConfig returns enabled placement AdSense/empty fallback metadata (public).
// Used by the client when select returns no house fill — not part of ads/select.
func PublicAdPlacementConfig() web.HandlerFunc {
	return func(c *web.Context) error {
		q := &query.ListAdPlacements{}
		if err := bus.Dispatch(c, q); err != nil {
			return c.Failure(err)
		}
		out := web.Map{}
		for _, p := range q.Result {
			if p == nil || !p.Enabled {
				continue
			}
			policy := p.EmptyPolicy
			if policy == "" {
				policy = "collapse"
			}
			entry := web.Map{
				"adsenseSlotId": p.AdSenseSlotID,
				"adsenseFormat": p.AdSenseFormat,
				"emptyPolicy":   policy,
				"kind":          p.Kind,
			}
			if p.MaxWidth != nil {
				entry["maxWidth"] = *p.MaxWidth
			}
			if p.MaxHeight != nil {
				entry["maxHeight"] = *p.MaxHeight
			}
			out[p.ID] = entry
		}
		return c.Ok(out)
	}
}

// ListCreativeVersions lists versions for a campaign.
func ListCreativeVersions() web.HandlerFunc {
	return func(c *web.Context) error {
		campaignID, err := c.ParamAsInt("id")
		if err != nil {
			return c.BadRequest(web.Map{"message": "Invalid campaign ID"})
		}
		q := &query.ListCreativeVersionsByCampaign{CampaignID: campaignID}
		if err := bus.Dispatch(c, q); err != nil {
			return c.Failure(err)
		}
		if q.Result == nil {
			q.Result = []*entity.CreativeVersion{}
		}
		return c.Ok(q.Result)
	}
}

type createCreativeVersionRequest struct {
	ImageURL      string `json:"imageUrl"`
	HTML          string `json:"html"`
	ClickURL      string `json:"clickUrl"`
	ConfigVersion int    `json:"configVersion"`
}

// CreateCreativeVersion appends an immutable version under a campaign (OCC).
func CreateCreativeVersion() web.HandlerFunc {
	return func(c *web.Context) error {
		campaignID, err := c.ParamAsInt("id")
		if err != nil {
			return c.BadRequest(web.Map{"message": "Invalid campaign ID"})
		}
		req := createCreativeVersionRequest{}
		if err := c.Bind(&req); err != nil {
			return c.BadRequest(web.Map{"message": "Invalid request body"})
		}
		req.ImageURL = strings.TrimSpace(req.ImageURL)
		req.HTML = strings.TrimSpace(req.HTML)
		req.ClickURL = strings.TrimSpace(req.ClickURL)
		if req.ClickURL == "" {
			return c.BadRequest(web.Map{"message": "clickUrl is required"})
		}
		if !validate.IsHTTPOrHTTPSURL(req.ClickURL) {
			return c.BadRequest(web.Map{"message": "clickUrl must be an http(s) URL"})
		}
		if req.ImageURL == "" && req.HTML == "" {
			return c.BadRequest(web.Map{"message": "Provide imageUrl or html"})
		}
		return c.WithTransaction(func() error {
			create := &cmd.CreateCreativeVersion{
				CampaignID:    campaignID,
				ImageURL:      req.ImageURL,
				HTML:          req.HTML,
				ClickURL:      req.ClickURL,
				ConfigVersion: req.ConfigVersion,
			}
			if err := bus.Dispatch(c, create); err != nil {
				return c.Failure(err)
			}
			return c.Ok(web.Map{
				"version":       create.Result,
				"configVersion": create.NewConfigVersion,
			})
		})
	}
}

// ListCampaignAssignments lists placement bindings for a campaign.
func ListCampaignAssignments() web.HandlerFunc {
	return func(c *web.Context) error {
		campaignID, err := c.ParamAsInt("id")
		if err != nil {
			return c.BadRequest(web.Map{"message": "Invalid campaign ID"})
		}
		q := &query.ListCampaignAssignmentsByCampaign{CampaignID: campaignID}
		if err := bus.Dispatch(c, q); err != nil {
			return c.Failure(err)
		}
		if q.Result == nil {
			q.Result = []*entity.CampaignAssignment{}
		}
		return c.Ok(q.Result)
	}
}

type graphAssignmentInput struct {
	PlacementID       string `json:"placementId"`
	CreativeVersionID int    `json:"creativeVersionId"`
}

type saveCampaignGraphRequest struct {
	Name          string                 `json:"name"`
	Advertiser    string                 `json:"advertiser"`
	StartAt       time.Time              `json:"startAt"`
	EndAt         time.Time              `json:"endAt"`
	Weight        int                    `json:"weight"`
	Locale        string                 `json:"locale"`
	Enabled       bool                   `json:"enabled"`
	PackageID     *int                   `json:"packageId"`
	ConfigVersion int                    `json:"configVersion"`
	Assignments   []graphAssignmentInput `json:"assignments"`
}

// SaveCampaignGraph updates campaign fields + replaces assignments in one OCC txn.
func SaveCampaignGraph() web.HandlerFunc {
	return func(c *web.Context) error {
		campaignID, err := c.ParamAsInt("id")
		if err != nil {
			return c.BadRequest(web.Map{"message": "Invalid campaign ID"})
		}
		req := saveCampaignGraphRequest{}
		if err := c.Bind(&req); err != nil {
			return c.BadRequest(web.Map{"message": "Invalid request body"})
		}
		req.Name = strings.TrimSpace(req.Name)
		req.Advertiser = strings.TrimSpace(req.Advertiser)
		if req.Locale == "" {
			req.Locale = "all"
		}
		if req.Name == "" || req.Advertiser == "" {
			return c.BadRequest(web.Map{"message": "name and advertiser are required"})
		}
		if req.Assignments == nil {
			req.Assignments = []graphAssignmentInput{}
		}
		seenPlacement := map[string]bool{}
		inputs := make([]cmd.CampaignAssignmentInput, 0, len(req.Assignments))
		for _, a := range req.Assignments {
			pid := strings.TrimSpace(a.PlacementID)
			if pid == "" || a.CreativeVersionID <= 0 {
				return c.BadRequest(web.Map{"message": "each assignment needs placementId and creativeVersionId"})
			}
			if !entity.IsCatalogPlacementID(pid) {
				return c.BadRequest(web.Map{"message": "unknown placementId: " + pid})
			}
			if seenPlacement[pid] {
				return c.BadRequest(web.Map{"message": "duplicate placementId in assignments: " + pid})
			}
			seenPlacement[pid] = true
			inputs = append(inputs, cmd.CampaignAssignmentInput{
				PlacementID: pid, CreativeVersionID: a.CreativeVersionID,
			})
		}
		return c.WithTransaction(func() error {
			save := &cmd.SaveSponsorshipCampaignGraph{
				ID: campaignID, Name: req.Name, Advertiser: req.Advertiser,
				StartAt: req.StartAt.UTC(), EndAt: req.EndAt.UTC(),
				Weight: req.Weight, Locale: req.Locale, Enabled: req.Enabled,
				PackageID: req.PackageID, ConfigVersion: req.ConfigVersion,
				Assignments: inputs,
			}
			if err := bus.Dispatch(c, save); err != nil {
				return c.Failure(err)
			}
			return c.Ok(web.Map{
				"campaign":    save.Result,
				"assignments": save.AssignmentResults,
			})
		})
	}
}

type updateAdPlacementRequest struct {
	AdSenseSlotID string `json:"adsenseSlotId"`
	AdSenseFormat string `json:"adsenseFormat"`
	EmptyPolicy   string `json:"emptyPolicy"`
}

// UpdateAdPlacement patches AdSense slot/format/empty_policy on a catalog placement (collab/admin).
// Does not hardcode publisher or slot ids — ops supply slot ids after AdSense unit creation.
func UpdateAdPlacement() web.HandlerFunc {
	return func(c *web.Context) error {
		id := strings.TrimSpace(c.Param("id"))
		if id == "" {
			return c.BadRequest(web.Map{"message": "Invalid placement ID"})
		}
		req := updateAdPlacementRequest{}
		if err := c.Bind(&req); err != nil {
			return c.BadRequest(web.Map{"message": "Invalid request body"})
		}
		policy := strings.TrimSpace(req.EmptyPolicy)
		if policy == "" {
			policy = "collapse"
		}
		if policy != "collapse" && policy != "reserve" {
			return c.BadRequest(web.Map{"message": "emptyPolicy must be collapse or reserve"})
		}
		return c.WithTransaction(func() error {
			update := &cmd.UpdateAdPlacement{
				ID:            id,
				AdSenseSlotID: strings.TrimSpace(req.AdSenseSlotID),
				AdSenseFormat: strings.TrimSpace(req.AdSenseFormat),
				EmptyPolicy:   policy,
			}
			if err := bus.Dispatch(c, update); err != nil {
				return c.Failure(err)
			}
			return c.Ok(update.Result)
		})
	}
}
