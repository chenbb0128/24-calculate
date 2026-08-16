package player

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/example/go-service/internal/apperror"
)

type ProgressMutationResponse struct {
	Action              string          `json:"action"`
	ItemID              string          `json:"item_id,omitempty"`
	Purchased           bool            `json:"purchased"`
	Equipped            bool            `json:"equipped"`
	Coins               int             `json:"coins"`
	Progress            json.RawMessage `json:"progress"`
	IdempotencyKey      string          `json:"idempotency_key,omitempty"`
	IdempotencyReplayed bool            `json:"idempotency_replayed,omitempty"`
}

type PurchaseInput struct {
	IdempotencyKey string `json:"idempotency_key"`
}

type PlayerPreferencesInput struct {
	MusicEnabled *bool    `json:"music_enabled"`
	SFXEnabled   *bool    `json:"sfx_enabled"`
	MusicTrack   *int     `json:"music_track"`
	MusicVolume  *float64 `json:"music_volume"`
	SFXVolume    *float64 `json:"sfx_volume"`
}

type shopItem struct {
	ID       string
	Category string
	Price    int
	MinLevel int
	MinStars int
}

var friendShopItems = map[string]shopItem{
	"classic":          {ID: "classic", Category: "skin"},
	"ocean":            {ID: "ocean", Category: "skin", Price: 360},
	"sunset":           {ID: "sunset", Category: "skin", Price: 720, MinLevel: 25, MinStars: 12},
	"candy":            {ID: "candy", Category: "skin", Price: 980, MinLevel: 18, MinStars: 8},
	"forest":           {ID: "forest", Category: "skin", Price: 1280, MinLevel: 35, MinStars: 18},
	"aurora":           {ID: "aurora", Category: "skin", Price: 1680, MinLevel: 50, MinStars: 28},
	"volcano":          {ID: "volcano", Category: "skin", Price: 2200, MinLevel: 75, MinStars: 45},
	"royal":            {ID: "royal", Category: "skin", Price: 3000, MinLevel: 100, MinStars: 72},
	"card_classic":     {ID: "card_classic", Category: "card"},
	"card_neon":        {ID: "card_neon", Category: "card", Price: 240},
	"card_candy":       {ID: "card_candy", Category: "card", Price: 420, MinLevel: 8},
	"operator_classic": {ID: "operator_classic", Category: "operator"},
	"operator_bubble":  {ID: "operator_bubble", Category: "operator", Price: 260, MinLevel: 5},
	"operator_prism":   {ID: "operator_prism", Category: "operator", Price: 520, MinLevel: 15},
	"result_classic":   {ID: "result_classic", Category: "result"},
	"result_burst":     {ID: "result_burst", Category: "result", Price: 360, MinLevel: 10},
	"result_fireworks": {ID: "result_fireworks", Category: "result", Price: 680, MinLevel: 20, MinStars: 6},
}

func (s *Service) PurchaseSkin(ctx context.Context, userID uint64, itemID string) (ProgressMutationResponse, error) {
	return s.purchaseShopItem(ctx, userID, itemID, "skin", "")
}

func (s *Service) PurchaseSkinWithKey(ctx context.Context, userID uint64, itemID, key string) (ProgressMutationResponse, error) {
	return s.purchaseShopItem(ctx, userID, itemID, "skin", key)
}

func (s *Service) EquipSkin(ctx context.Context, userID uint64, itemID string) (ProgressMutationResponse, error) {
	return s.equipShopItem(ctx, userID, itemID, "skin")
}

func (s *Service) PurchaseCosmetic(ctx context.Context, userID uint64, itemID string) (ProgressMutationResponse, error) {
	item, exists := friendShopItems[strings.TrimSpace(itemID)]
	if !exists || item.Category == "skin" {
		return ProgressMutationResponse{}, apperror.BadRequest("cosmetic id is invalid", nil)
	}
	return s.purchaseShopItem(ctx, userID, itemID, item.Category, "")
}

func (s *Service) PurchaseCosmeticWithKey(ctx context.Context, userID uint64, itemID, key string) (ProgressMutationResponse, error) {
	item, exists := friendShopItems[strings.TrimSpace(itemID)]
	if !exists || item.Category == "skin" {
		return ProgressMutationResponse{}, apperror.BadRequest("cosmetic id is invalid", nil)
	}
	return s.purchaseShopItem(ctx, userID, itemID, item.Category, key)
}

func (s *Service) EquipCosmetic(ctx context.Context, userID uint64, itemID string) (ProgressMutationResponse, error) {
	item, exists := friendShopItems[strings.TrimSpace(itemID)]
	if !exists || item.Category == "skin" {
		return ProgressMutationResponse{}, apperror.BadRequest("cosmetic id is invalid", nil)
	}
	return s.equipShopItem(ctx, userID, itemID, item.Category)
}

func (s *Service) purchaseShopItem(ctx context.Context, userID uint64, itemID, category, requestedKey string) (result ProgressMutationResponse, err error) {
	item, exists := friendShopItems[strings.TrimSpace(itemID)]
	if !exists || item.Category != category {
		return ProgressMutationResponse{}, apperror.BadRequest("shop item is invalid", nil)
	}
	result.ItemID = item.ID
	result.Action = "purchase"
	key := strings.TrimSpace(requestedKey)
	if key == "" {
		key = fmt.Sprintf("shop:%d:%s", userID, item.ID)
	}
	if len(key) < 8 || len(key) > 128 {
		return ProgressMutationResponse{}, apperror.BadRequest("idempotency_key length is invalid", nil)
	}
	result.IdempotencyKey = key
	progress, err := s.store.MutatePlayerProgress(ctx, userID, func(state map[string]any) error {
		purchases := ensureObject(state, "shop_purchase_records")
		if previous, ok := purchases[key].(map[string]any); ok {
			result.Purchased = readBool(previous["purchased"])
			result.Equipped = readBool(previous["equipped"])
			result.IdempotencyReplayed = true
			return nil
		}
		if !shopUnlocks(state, item) {
			return apperror.BadRequest("shop item requirements are not met", nil)
		}
		ownedKey := "owned_skins"
		if category != "skin" {
			ownedKey = "owned_cosmetics"
		}
		owned := readStringList(state[ownedKey])
		if containsString(owned, item.ID) {
			result.Equipped = isShopItemEquipped(state, item)
			return nil
		}
		coins := readInt(state["coins"])
		if coins < item.Price {
			return apperror.New(10006, 409, "金币不足", nil)
		}
		state["coins"] = coins - item.Price
		state[ownedKey] = append(owned, item.ID)
		setShopItemEquipped(state, item)
		if category == "skin" {
			reward := unlockServerAchievement(state, "skin_unlock", true, 40)
			state["coins"] = minInt(999999, readInt(state["coins"])+reward)
		}
		result.Purchased = true
		result.Equipped = true
		purchases[result.IdempotencyKey] = map[string]any{"item_id": item.ID, "purchased": true, "equipped": true}
		state["shop_purchase_records"] = purchases
		return nil
	})
	if err != nil {
		return ProgressMutationResponse{}, err
	}
	result.Coins = progressCoins(string(progress))
	result.Progress = progress
	return result, nil
}

func (s *Service) equipShopItem(ctx context.Context, userID uint64, itemID, category string) (result ProgressMutationResponse, err error) {
	item, exists := friendShopItems[strings.TrimSpace(itemID)]
	if !exists || item.Category != category {
		return ProgressMutationResponse{}, apperror.BadRequest("shop item is invalid", nil)
	}
	result.Action = "equip"
	result.ItemID = item.ID
	progress, err := s.store.MutatePlayerProgress(ctx, userID, func(state map[string]any) error {
		key := "owned_skins"
		if category != "skin" {
			key = "owned_cosmetics"
		}
		if !containsString(readStringList(state[key]), item.ID) {
			return apperror.New(10007, 409, "外观尚未拥有", nil)
		}
		setShopItemEquipped(state, item)
		result.Equipped = true
		return nil
	})
	if err != nil {
		return ProgressMutationResponse{}, err
	}
	result.Coins = progressCoins(string(progress))
	result.Progress = progress
	return result, nil
}

func (s *Service) UpdatePlayerPreferences(ctx context.Context, userID uint64, input PlayerPreferencesInput) (ProgressMutationResponse, error) {
	result := ProgressMutationResponse{Action: "preferences"}
	progress, err := s.store.MutatePlayerProgress(ctx, userID, func(state map[string]any) error {
		audio := ensureObject(state, "audio")
		if input.MusicEnabled != nil {
			audio["music_enabled"] = *input.MusicEnabled
		}
		if input.SFXEnabled != nil {
			audio["sfx_enabled"] = *input.SFXEnabled
		}
		if input.MusicTrack != nil {
			if *input.MusicTrack < 0 || *input.MusicTrack > 2 {
				return apperror.BadRequest("music_track is out of range", nil)
			}
			audio["music_track"] = *input.MusicTrack
		}
		if input.MusicVolume != nil {
			if *input.MusicVolume < 0 || *input.MusicVolume > 1 {
				return apperror.BadRequest("music_volume is out of range", nil)
			}
			audio["music_volume"] = *input.MusicVolume
		}
		if input.SFXVolume != nil {
			if *input.SFXVolume < 0 || *input.SFXVolume > 1 {
				return apperror.BadRequest("sfx_volume is out of range", nil)
			}
			audio["sfx_volume"] = *input.SFXVolume
		}
		state["audio"] = audio
		return nil
	})
	if err != nil {
		return ProgressMutationResponse{}, err
	}
	result.Coins = progressCoins(string(progress))
	result.Progress = progress
	return result, nil
}

func shopUnlocks(state map[string]any, item shopItem) bool {
	if readInt(state["unlocked_level"]) < item.MinLevel {
		return false
	}
	levels, _ := state["levels"].(map[string]any)
	stars := 0
	for _, raw := range levels {
		if level, ok := raw.(map[string]any); ok {
			stars += readInt(level["stars"])
		}
	}
	return stars >= item.MinStars
}

func isShopItemEquipped(state map[string]any, item shopItem) bool {
	if item.Category == "skin" {
		return strings.TrimSpace(fmt.Sprint(state["equipped_skin"])) == item.ID
	}
	equipped := ensureObject(state, "equipped_cosmetics")
	return strings.TrimSpace(fmt.Sprint(equipped[item.Category])) == item.ID
}

func setShopItemEquipped(state map[string]any, item shopItem) {
	if item.Category == "skin" {
		state["equipped_skin"] = item.ID
		return
	}
	equipped := ensureObject(state, "equipped_cosmetics")
	equipped[item.Category] = item.ID
	state["equipped_cosmetics"] = equipped
}

func readStringList(value any) []string {
	result := []string{}
	switch values := value.(type) {
	case []any:
		for _, item := range values {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" {
				result = append(result, text)
			}
		}
	case []string:
		result = append(result, values...)
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
