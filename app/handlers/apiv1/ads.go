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
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

type selectAdsSlot struct {
	InstanceID  string `json:"instanceId"`
	PlacementID string `json:"placementId"`
}

type selectAdsRequest struct {
	Slots []selectAdsSlot `json:"slots"`
}

// SelectAds handles POST /api/v1/ads/select?locale=
// Pipeline: candidates SQL → adsselect → batch creatives → PublicAd map.
// Blank advertiser → null (#39).
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
		instances := make([]adsselect.InstanceReq, 0, len(req.Slots))
		for _, s := range req.Slots {
			iid := strings.TrimSpace(s.InstanceID)
			pid := strings.TrimSpace(s.PlacementID)
			if iid == "" || pid == "" {
				continue
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
	ImageURL string `json:"imageUrl"`
	HTML     string `json:"html"`
	ClickURL string `json:"clickUrl"`
}

// CreateCreativeVersion appends an immutable version under a campaign.
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
		if req.ImageURL == "" && req.HTML == "" {
			return c.BadRequest(web.Map{"message": "Provide imageUrl or html"})
		}
		return c.WithTransaction(func() error {
			create := &cmd.CreateCreativeVersion{
				CampaignID: campaignID,
				ImageURL:   req.ImageURL,
				HTML:       req.HTML,
				ClickURL:   req.ClickURL,
			}
			if err := bus.Dispatch(c, create); err != nil {
				return c.Failure(err)
			}
			return c.Ok(create.Result)
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

type upsertAssignmentRequest struct {
	PlacementID       string `json:"placementId"`
	CreativeVersionID int    `json:"creativeVersionId"`
}

// UpsertCampaignAssignment sets placement → creative version for a campaign.
func UpsertCampaignAssignment() web.HandlerFunc {
	return func(c *web.Context) error {
		campaignID, err := c.ParamAsInt("id")
		if err != nil {
			return c.BadRequest(web.Map{"message": "Invalid campaign ID"})
		}
		req := upsertAssignmentRequest{}
		if err := c.Bind(&req); err != nil {
			return c.BadRequest(web.Map{"message": "Invalid request body"})
		}
		req.PlacementID = strings.TrimSpace(req.PlacementID)
		if req.PlacementID == "" || req.CreativeVersionID <= 0 {
			return c.BadRequest(web.Map{"message": "placementId and creativeVersionId are required"})
		}
		return c.WithTransaction(func() error {
			up := &cmd.UpsertCampaignAssignment{
				CampaignID:        campaignID,
				PlacementID:       req.PlacementID,
				CreativeVersionID: req.CreativeVersionID,
			}
			if err := bus.Dispatch(c, up); err != nil {
				return c.Failure(err)
			}
			return c.Ok(up.Result)
		})
	}
}

// DeleteCampaignAssignment removes a placement binding.
func DeleteCampaignAssignment() web.HandlerFunc {
	return func(c *web.Context) error {
		campaignID, err := c.ParamAsInt("id")
		if err != nil {
			return c.BadRequest(web.Map{"message": "Invalid campaign ID"})
		}
		placementID := strings.TrimSpace(c.Param("placementId"))
		if placementID == "" {
			return c.BadRequest(web.Map{"message": "placementId is required"})
		}
		return c.WithTransaction(func() error {
			if err := bus.Dispatch(c, &cmd.DeleteCampaignAssignment{
				CampaignID: campaignID, PlacementID: placementID,
			}); err != nil {
				return c.Failure(err)
			}
			return c.Ok(web.Map{})
		})
	}
}
