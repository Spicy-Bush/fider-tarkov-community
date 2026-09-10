package apiv1

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

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
