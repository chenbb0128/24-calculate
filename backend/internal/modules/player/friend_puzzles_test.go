package player

import "testing"

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
	if hash != "6d3066fd" {
		t.Fatalf("question hash = %q, want client-compatible hash", hash)
	}
	if len(ids) != 8 || ids[0] != "L043-Q1" || ids[7] != "L043-Q8" {
		t.Fatalf("puzzle ids = %#v, want L043-Q1 through L043-Q8", ids)
	}
	if len(puzzles) != 8 || friendNumberKey(puzzles[0].Numbers) != "1,2,2,9" {
		t.Fatalf("puzzles = %#v, want deterministic first puzzle", puzzles)
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
