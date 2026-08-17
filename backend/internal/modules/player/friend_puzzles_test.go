package player

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestFriendPuzzleContractMatchesClientSeedProtocol(t *testing.T) {
	room := FriendRoom{
		RoomSeed: 42,
		Rules: FriendRoomRules{
			QuestionCount:       8,
			TimeLimitSeconds:    120,
			Target:              24,
			NoHint:              true,
			UseSameSeed:         true,
			IntegerIntermediate: true,
		},
	}

	hash, ids, puzzles := friendRoomContract(room)
	if len(hash) != 8 || len(ids) != 8 || !strings.HasPrefix(ids[0], "fp_") || ids[0] == ids[7] {
		t.Fatalf("question contract = hash %q, ids %#v", hash, ids)
	}
	if len(puzzles) != 8 {
		t.Fatalf("puzzles = %#v, want 8 deterministic puzzles", puzzles)
	}
	seen := map[string]struct{}{}
	for _, puzzle := range puzzles {
		if len(puzzle.Numbers) != 4 || puzzle.SolutionCount < 1 || puzzle.QuestionHash == "" || puzzle.TimeLimitMS <= 0 {
			t.Fatalf("invalid puzzle contract = %#v", puzzle)
		}
		key := friendNumberKey(puzzle.Numbers)
		if _, exists := seen[key]; exists {
			t.Fatalf("duplicate puzzle numbers = %q", key)
		}
		seen[key] = struct{}{}
		for _, number := range puzzle.Numbers {
			if number < 1 || number > 9 {
				t.Fatalf("number %d is outside 1..9", number)
			}
		}
	}
	encoded, err := json.Marshal(puzzles[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "first_solution_steps") || strings.Contains(string(encoded), "first_indices") {
		t.Fatalf("serialized puzzle leaked solution steps: %s", encoded)
	}
}

func TestFriendPuzzleGenerationIsReproducibleAndSeeded(t *testing.T) {
	first := generateFriendPuzzleContract(12345, 8)
	second := generateFriendPuzzleContract(12345, 8)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same seed generated different contracts")
	}
	if reflect.DeepEqual(first, generateFriendPuzzleContract(12346, 8)) {
		t.Fatalf("different seeds generated the same contract")
	}
}

func TestFriendCandidatePoolIsExpanded(t *testing.T) {
	pool := friendCandidatePool()
	if len(pool) < 100 {
		t.Fatalf("candidate pool size = %d, want at least 100 verified combinations", len(pool))
	}
	t.Logf("verified friend candidate pool size = %d", len(pool))
}

func TestRecentPuzzleHistoryUsesQuestionHashes(t *testing.T) {
	base := generateFriendPuzzleContract(12345, 8)
	if len(base) == 0 {
		t.Fatal("generated puzzle contract is empty")
	}
	excluded := map[string]struct{}{base[0].QuestionHash: {}}
	filtered := generateFriendPuzzleContractExcluding(12345, 8, excluded)
	for _, puzzle := range filtered {
		if _, exists := excluded[puzzle.QuestionHash]; exists {
			t.Fatalf("recent question hash %q was not excluded", puzzle.QuestionHash)
		}
	}
	if same := generateFriendPuzzleContract(12345, 8); len(same) == 0 || same[0].QuestionHash != base[0].QuestionHash {
		t.Fatal("same room seed no longer produces the same deterministic contract")
	}
}

func TestReplayFriendSolutionValidatesOperations(t *testing.T) {
	puzzle := generateFriendPuzzleContract(42, 8)[0]
	solution := friendSolveDetailed(puzzle.Numbers, 1)[0].steps
	if !replayFriendSolution(puzzle.Numbers, solution, puzzle.Rules) {
		t.Fatal("replayFriendSolution() = false for generated solution")
	}
	solution[0].Operator = "-"
	if replayFriendSolution(puzzle.Numbers, solution, puzzle.Rules) {
		t.Fatal("replayFriendSolution() = true for tampered solution")
	}
}

func TestReplayFriendSolutionAcceptsWeChatMultiplicationAndDivisionSymbols(t *testing.T) {
	if value, ok := applyFriendOperator(8, 2, "÷"); !ok || value != 4 {
		t.Fatalf("division symbol result = %d, ok = %v; want 4, true", value, ok)
	}
	if value, ok := applyFriendOperator(6, 4, "×"); !ok || value != 24 {
		t.Fatalf("multiplication symbol result = %d, ok = %v; want 24, true", value, ok)
	}
}
