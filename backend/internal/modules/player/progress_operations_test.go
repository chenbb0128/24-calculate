package player

import (
	"context"
	"encoding/json"
	"testing"
)

type progressStore struct {
	*leaderboardStore
	state map[string]any
}

func newProgressStore(t *testing.T) *progressStore {
	t.Helper()
	state := map[string]any{}
	if err := json.Unmarshal([]byte(DefaultProgressJSON), &state); err != nil {
		t.Fatal(err)
	}
	return &progressStore{leaderboardStore: &leaderboardStore{}, state: state}
}

func (s *progressStore) MutatePlayerProgress(_ context.Context, _ uint64, mutate ProgressMutation) (json.RawMessage, error) {
	if err := mutate(s.state); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(s.state)
	return encoded, err
}

func TestPurchaseAndEquipSkinAreServerSideOperations(t *testing.T) {
	store := newProgressStore(t)
	service := NewService(leaderboardProfileReader{profile: testFriendProfile(3)}, store)
	store.state["coins"] = float64(500)

	purchased, err := service.PurchaseSkin(context.Background(), 3, "ocean")
	if err != nil {
		t.Fatalf("PurchaseSkin() error = %v", err)
	}
	if !purchased.Purchased || !purchased.Equipped || purchased.Coins != 180 {
		t.Fatalf("purchase result = %#v, want 360-coin purchase", purchased)
	}
	if store.state["equipped_skin"] != "ocean" {
		t.Fatalf("equipped skin = %#v, want ocean", store.state["equipped_skin"])
	}

	if _, err := service.EquipSkin(context.Background(), 3, "royal"); err == nil {
		t.Fatal("EquipSkin() error = nil for unowned skin")
	}
}

func TestPurchaseWithSameIdempotencyKeyDoesNotChargeTwice(t *testing.T) {
	store := newProgressStore(t)
	service := NewService(leaderboardProfileReader{profile: testFriendProfile(3)}, store)
	store.state["coins"] = float64(500)

	first, err := service.PurchaseSkinWithKey(context.Background(), 3, "ocean", "shop-test-ocean-1")
	if err != nil {
		t.Fatalf("first purchase error = %v", err)
	}
	second, err := service.PurchaseSkinWithKey(context.Background(), 3, "ocean", "shop-test-ocean-1")
	if err != nil {
		t.Fatalf("replayed purchase error = %v", err)
	}
	if !first.Purchased || !second.Purchased || !second.IdempotencyReplayed || readInt(store.state["coins"]) != first.Coins {
		t.Fatalf("first = %#v, second = %#v, coins = %#v; want one charge", first, second, store.state["coins"])
	}
}

func TestUpdatePlayerPreferencesPersistsValidatedAudioSettings(t *testing.T) {
	store := newProgressStore(t)
	service := NewService(leaderboardProfileReader{profile: testFriendProfile(3)}, store)
	musicEnabled := false
	musicVolume := 0.25

	result, err := service.UpdatePlayerPreferences(context.Background(), 3, PlayerPreferencesInput{
		MusicEnabled: &musicEnabled,
		MusicVolume:  &musicVolume,
	})
	if err != nil {
		t.Fatalf("UpdatePlayerPreferences() error = %v", err)
	}
	audio, ok := store.state["audio"].(map[string]any)
	if !ok || audio["music_enabled"] != false || audio["music_volume"] != 0.25 || len(result.Progress) == 0 {
		t.Fatalf("audio = %#v, result = %#v, want persisted settings", audio, result)
	}
}
