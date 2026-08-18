package redis

import "testing"

func TestFriendRoundKeysAreIsolated(t *testing.T) {
	roomCode := "123456"
	firstRound := "round-a"
	secondRound := "round-b"

	keyPairs := []struct {
		name string
		make func(string, string) string
	}{
		{"progress", FriendRoomMatchProgressRoundKey},
		{"progress state", FriendRoomMatchProgressStateRoundKey},
		{"progress events", FriendRoomMatchProgressEventsRoundKey},
		{"results", FriendRoomMatchResultsRoundKey},
	}
	for _, item := range keyPairs {
		t.Run(item.name, func(t *testing.T) {
			first := item.make(roomCode, firstRound)
			second := item.make(roomCode, secondRound)
			if first == second {
				t.Fatalf("round keys are equal: %q", first)
			}
			if first == item.make("654321", firstRound) {
				t.Fatalf("room keys are equal: %q", first)
			}
			if first == item.make(roomCode, "") || second == item.make(roomCode, "") {
				t.Fatalf("round key unexpectedly uses legacy key: %q", first)
			}
		})
	}
}

func TestFriendRoundKeysKeepLegacyFallback(t *testing.T) {
	roomCode := "123456"
	checks := []struct {
		name   string
		got    string
		legacy string
	}{
		{"progress", FriendRoomMatchProgressRoundKey(roomCode, ""), FriendRoomMatchProgressKey(roomCode)},
		{"progress state", FriendRoomMatchProgressStateRoundKey(roomCode, ""), FriendRoomMatchProgressStateKey(roomCode)},
		{"progress events", FriendRoomMatchProgressEventsRoundKey(roomCode, ""), FriendRoomMatchProgressEventsKey(roomCode)},
		{"results", FriendRoomMatchResultsRoundKey(roomCode, ""), FriendRoomMatchResultsKey(roomCode)},
	}
	for _, check := range checks {
		if check.got != check.legacy {
			t.Errorf("%s key = %q, want legacy key %q", check.name, check.got, check.legacy)
		}
	}
}
