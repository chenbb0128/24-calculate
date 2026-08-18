package player

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
)

// FriendPuzzleContract is the server-owned part of a friend match. The client
// may render these numbers, but it cannot replace the puzzle id or number set
// when submitting a result.
type FriendPuzzleContract struct {
	PuzzleID           string                    `json:"puzzle_id"`
	Numbers            []int                     `json:"numbers"`
	Rules              FriendPuzzleRules         `json:"rules"`
	SolutionCount      int                       `json:"solution_count,omitempty"`
	ShortestSteps      int                       `json:"shortest_steps,omitempty"`
	QuestionHash       string                    `json:"question_hash,omitempty"`
	SourceSeed         string                    `json:"source_seed,omitempty"`
	Difficulty         string                    `json:"difficulty"`
	TimeLimitMS        int                       `json:"time_limit_ms"`
	FirstSolutionSteps []FriendMatchSolutionStep `json:"-"`
}

type FriendPuzzleRules struct {
	UseEachNumberOnce          bool     `json:"use_each_number_once"`
	IntegerIntermediateResults bool     `json:"integer_intermediate_results"`
	AllowedOperators           []string `json:"allowed_operators"`
	RequiredOperator           string   `json:"required_operator"`
	ForbiddenOperator          string   `json:"forbidden_operator"`
	AllowNegativeIntermediate  bool     `json:"allow_negative_intermediate"`
}

type FriendMatchSolutionStep struct {
	FirstIndices  []int  `json:"first_indices"`
	SecondIndices []int  `json:"second_indices"`
	First         int    `json:"first"`
	Second        int    `json:"second"`
	Operator      string `json:"operator"`
	Left          *int   `json:"left,omitempty"`
	Right         *int   `json:"right,omitempty"`
	Result        *int   `json:"result,omitempty"`
}

type friendSolution struct {
	expression string
	steps      []FriendMatchSolutionStep
}

type friendSolveItem struct {
	value      int
	expression string
	indices    []int
	steps      []FriendMatchSolutionStep
}

type friendSeededRandom struct {
	seed uint32
}

func newFriendSeededRandom(seed int64) *friendSeededRandom {
	value := uint32(seed)
	if value == 0 {
		value = 1
	}
	return &friendSeededRandom{seed: value}
}

func (r *friendSeededRandom) next() float64 {
	r.seed = 1664525*r.seed + 1013904223
	return float64(r.seed) / 4294967296
}

func (r *friendSeededRandom) int(minimum, maximum int) int {
	return int(math.Floor(r.next()*float64(maximum-minimum+1))) + minimum
}

func friendNumberKey(numbers []int) string {
	values := append([]int(nil), numbers...)
	sort.Ints(values)
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = fmt.Sprintf("%d", value)
	}
	return strings.Join(parts, ",")
}

func friendSolveDetailed(numbers []int, maxSolutions int) []friendSolution {
	if maxSolutions <= 0 {
		maxSolutions = 40
	}
	result := make([]friendSolution, 0, maxSolutions)
	seenExpressions := make(map[string]struct{})
	var search func([]friendSolveItem)
	search = func(items []friendSolveItem) {
		if len(result) >= maxSolutions {
			return
		}
		if len(items) == 1 {
			if items[0].value == 24 {
				if _, exists := seenExpressions[items[0].expression]; !exists {
					seenExpressions[items[0].expression] = struct{}{}
					result = append(result, friendSolution{
						expression: items[0].expression,
						steps:      append([]FriendMatchSolutionStep(nil), items[0].steps...),
					})
				}
			}
			return
		}

		for leftIndex := 0; leftIndex < len(items); leftIndex++ {
			for rightIndex := leftIndex + 1; rightIndex < len(items); rightIndex++ {
				left := items[leftIndex]
				right := items[rightIndex]
				rest := make([]friendSolveItem, 0, len(items)-2)
				for index, item := range items {
					if index != leftIndex && index != rightIndex {
						rest = append(rest, item)
					}
				}
				candidates := []struct {
					value    int
					operator string
					first    friendSolveItem
					second   friendSolveItem
				}{
					{left.value + right.value, "+", left, right},
					{left.value * right.value, "×", left, right},
					{left.value - right.value, "-", left, right},
					{right.value - left.value, "-", right, left},
				}
				if right.value != 0 && left.value%right.value == 0 {
					candidates = append(candidates, struct {
						value    int
						operator string
						first    friendSolveItem
						second   friendSolveItem
					}{left.value / right.value, "÷", left, right})
				}
				if left.value != 0 && right.value%left.value == 0 {
					candidates = append(candidates, struct {
						value    int
						operator string
						first    friendSolveItem
						second   friendSolveItem
					}{right.value / left.value, "÷", right, left})
				}

				for _, candidate := range candidates {
					if math.Abs(float64(candidate.value)) > 10000 {
						continue
					}
					step := FriendMatchSolutionStep{
						FirstIndices:  append([]int(nil), candidate.first.indices...),
						SecondIndices: append([]int(nil), candidate.second.indices...),
						First:         candidate.first.value,
						Second:        candidate.second.value,
						Operator:      candidate.operator,
					}
					next := append([]friendSolveItem(nil), rest...)
					next = append(next, friendSolveItem{
						value:      candidate.value,
						expression: fmt.Sprintf("(%s %s %s)", candidate.first.expression, candidate.operator, candidate.second.expression),
						indices:    append(append([]int(nil), candidate.first.indices...), candidate.second.indices...),
						steps:      append(append(append([]FriendMatchSolutionStep(nil), candidate.first.steps...), candidate.second.steps...), step),
					})
					search(next)
				}
			}
		}
	}

	items := make([]friendSolveItem, len(numbers))
	for index, number := range numbers {
		items[index] = friendSolveItem{
			value:      number,
			expression: fmt.Sprintf("%d", number),
			indices:    []int{index},
		}
	}
	search(items)
	return result
}

func shortestSolutionSteps(solutions []friendSolution) int {
	shortest := 0
	for _, solution := range solutions {
		steps := len(solution.steps)
		if steps > 0 && (shortest == 0 || steps < shortest) {
			shortest = steps
		}
	}
	return shortest
}

type friendCandidate struct {
	numbers     []int
	solutions   []friendSolution
	difficulty  string
	timeLimitMS int
}

var friendCandidateCache struct {
	sync.Once
	values []friendCandidate
}

// friendCandidatePool enumerates every sorted four-number multiset from 1 to
// 9 once and keeps only questions with at least one server-verified integer
// solution. This is much larger and more honest than a hand-written list: the
// finite pool is still bounded by the mathematics of four digits.
func friendCandidatePool() [][]int {
	friendCandidateCache.Do(func() {
		for first := 1; first <= 9; first++ {
			for second := first; second <= 9; second++ {
				for third := second; third <= 9; third++ {
					for fourth := third; fourth <= 9; fourth++ {
						numbers := []int{first, second, third, fourth}
						rules := friendPuzzleRules()
						solutions := verifiedFriendSolutions(numbers, rules, 200)
						if len(solutions) == 0 {
							continue
						}
						friendCandidateCache.values = append(friendCandidateCache.values, friendCandidate{
							numbers: append([]int(nil), numbers...), solutions: solutions,
							difficulty: friendPuzzleDifficulty(solutions), timeLimitMS: friendPuzzleTimeLimit(solutions),
						})
					}
				}
			}
		}
	})
	result := make([][]int, len(friendCandidateCache.values))
	for index, candidate := range friendCandidateCache.values {
		result[index] = append([]int(nil), candidate.numbers...)
	}
	return result
}

func friendPuzzleRules() FriendPuzzleRules {
	return FriendPuzzleRules{
		UseEachNumberOnce:          true,
		IntegerIntermediateResults: true,
		AllowedOperators:           []string{"+", "-", "×", "÷"},
		AllowNegativeIntermediate:  false,
	}
}

func verifiedFriendSolutions(numbers []int, rules FriendPuzzleRules, maxSolutions int) []friendSolution {
	raw := friendSolveDetailed(numbers, maxSolutions)
	verified := make([]friendSolution, 0, len(raw))
	for _, solution := range raw {
		if replayFriendSolution(numbers, solution.steps, rules) {
			verified = append(verified, solution)
		}
	}
	return verified
}

func generateFriendPuzzleContract(roomSeed int64, count int) []FriendPuzzleContract {
	return generateFriendPuzzleContractExcluding(roomSeed, count, nil)
}

func generateFriendPuzzleContractExcluding(roomSeed int64, count int, excluded map[string]struct{}) []FriendPuzzleContract {
	if count <= 0 {
		count = 8
	}
	if roomSeed < 0 {
		roomSeed = -roomSeed
	}
	friendCandidatePool()
	seed := roomSeed
	random := newFriendSeededRandom(seed + 0x9e3779b9)
	candidates := append([]friendCandidate(nil), friendCandidateCache.values...)
	for index := len(candidates) - 1; index > 0; index-- {
		swap := random.int(0, index)
		candidates[index], candidates[swap] = candidates[swap], candidates[index]
	}
	used := make(map[string]struct{})
	result := make([]FriendPuzzleContract, 0, count)
	tryCandidate := func(candidate friendCandidate, allowExcluded bool) {
		if len(result) >= count {
			return
		}
		numbers := candidate.numbers
		key := friendNumberKey(numbers)
		if _, exists := used[key]; exists {
			return
		}
		solutions := candidate.solutions
		if len(solutions) < 1 {
			return
		}
		rules := friendPuzzleRules()
		questionHash := friendPuzzleContentHash(numbers, rules)
		if !allowExcluded {
			// Recent history stores question_hash values. Keep accepting the
			// normalized-number key as a compatibility fallback for any old
			// Redis data written by an earlier build.
			if _, exists := excluded[questionHash]; exists {
				return
			}
			if _, exists := excluded[key]; exists {
				return
			}
		}
		used[key] = struct{}{}
		firstSteps := append([]FriendMatchSolutionStep(nil), solutions[0].steps...)
		result = append(result, FriendPuzzleContract{
			PuzzleID:      fmt.Sprintf("fp_%s", questionHash[:16]),
			Numbers:       append([]int(nil), numbers...),
			Rules:         rules,
			SolutionCount: len(solutions), ShortestSteps: shortestSolutionSteps(solutions),
			QuestionHash: questionHash, Difficulty: candidate.difficulty, TimeLimitMS: candidate.timeLimitMS,
			FirstSolutionSteps: firstSteps,
		})
	}
	for _, candidate := range candidates {
		tryCandidate(candidate, false)
	}
	// Recent-history avoidance is best effort. If it would make a short pool
	// unable to fill a room, reuse the oldest candidates rather than failing a
	// match entirely.
	if len(result) < count {
		for _, candidate := range candidates {
			tryCandidate(candidate, true)
		}
	}
	return result
}

func friendPuzzleContentHash(numbers []int, rules FriendPuzzleRules) string {
	payload, _ := json.Marshal(struct {
		PoolVersion int               `json:"pool_version"`
		Numbers     []int             `json:"numbers"`
		Rules       FriendPuzzleRules `json:"rules"`
	}{PoolVersion: 2, Numbers: numbers, Rules: rules})
	digest := sha256.Sum256(payload)
	return fmt.Sprintf("%x", digest[:])
}

func friendPuzzleDifficulty(solutions []friendSolution) string {
	if len(solutions) <= 2 {
		return "hard"
	}
	if len(solutions) >= 12 {
		return "easy"
	}
	return "standard"
}

func friendPuzzleTimeLimit(solutions []friendSolution) int {
	switch friendPuzzleDifficulty(solutions) {
	case "easy":
		return 12000
	case "hard":
		return 22000
	default:
		return 17000
	}
}

type friendQuestionFingerprint struct {
	Index    int    `json:"index"`
	PuzzleID string `json:"puzzle_id"`
	Numbers  string `json:"numbers"`
}

type friendMatchFingerprint struct {
	RoomSeed int64                       `json:"room_seed"`
	Rules    FriendRoomRules             `json:"rules"`
	Puzzles  []friendQuestionFingerprint `json:"puzzles"`
}

func friendQuestionHash(roomSeed int64, rules FriendRoomRules, puzzles []FriendPuzzleContract) string {
	items := make([]friendQuestionFingerprint, len(puzzles))
	for index, puzzle := range puzzles {
		items[index] = friendQuestionFingerprint{
			Index:    index,
			PuzzleID: puzzle.PuzzleID,
			Numbers:  friendNumberKey(puzzle.Numbers),
		}
	}
	payload, _ := json.Marshal(friendMatchFingerprint{RoomSeed: roomSeed, Rules: rules, Puzzles: items})
	state := uint32(2166136261)
	for _, value := range payload {
		state ^= uint32(value)
		state *= 16777619
	}
	return fmt.Sprintf("%08x", state)
}

func friendRoomContract(room FriendRoom) (string, []string, []FriendPuzzleContract) {
	return friendRoomContractExcluding(room, nil)
}

func friendRoomContractExcluding(room FriendRoom, excluded map[string]struct{}) (string, []string, []FriendPuzzleContract) {
	puzzles := room.Puzzles
	if len(puzzles) == 0 {
		puzzles = generateFriendPuzzleContractExcluding(room.RoomSeed, room.Rules.QuestionCount, excluded)
	}
	questionIDs := make([]string, len(puzzles))
	for index, puzzle := range puzzles {
		questionIDs[index] = puzzle.PuzzleID
	}
	return friendQuestionHash(room.RoomSeed, room.Rules, puzzles), questionIDs, puzzles
}

func friendPuzzleHashes(puzzles []FriendPuzzleContract) []string {
	hashes := make([]string, 0, len(puzzles))
	seen := make(map[string]struct{}, len(puzzles))
	for _, puzzle := range puzzles {
		if puzzle.QuestionHash == "" {
			puzzle.QuestionHash = friendPuzzleContentHash(puzzle.Numbers, puzzle.Rules)
		}
		if _, exists := seen[puzzle.QuestionHash]; exists {
			continue
		}
		seen[puzzle.QuestionHash] = struct{}{}
		hashes = append(hashes, puzzle.QuestionHash)
	}
	return hashes
}

func friendPuzzleContractOverlapsRecentHistory(puzzles []FriendPuzzleContract, recent map[string]struct{}) bool {
	if len(puzzles) == 0 || len(recent) == 0 {
		return false
	}
	for _, hash := range friendPuzzleHashes(puzzles) {
		if _, exists := recent[hash]; exists {
			return true
		}
	}
	return false
}

func replayFriendSolution(numbers []int, steps []FriendMatchSolutionStep, rules FriendPuzzleRules) bool {
	if len(numbers) != 4 || len(steps) != 3 {
		return false
	}
	items := make([]friendSolveItem, len(numbers))
	for index, number := range numbers {
		items[index] = friendSolveItem{value: number, indices: []int{index}}
	}
	sameGroup := func(left, right []int) bool {
		if len(left) != len(right) {
			return false
		}
		seen := make(map[int]int, len(left))
		for _, value := range left {
			seen[value]++
		}
		for _, value := range right {
			if seen[value] == 0 {
				return false
			}
			seen[value]--
		}
		return true
	}
	for _, step := range steps {
		firstIndex, secondIndex := -1, -1
		for index, item := range items {
			if sameGroup(item.indices, step.FirstIndices) {
				firstIndex = index
			}
			if sameGroup(item.indices, step.SecondIndices) {
				secondIndex = index
			}
		}
		if firstIndex < 0 || secondIndex < 0 || firstIndex == secondIndex {
			return false
		}
		first := items[firstIndex]
		second := items[secondIndex]
		value, ok := applyFriendOperator(first.value, second.value, step.Operator)
		if !ok || (rules.IntegerIntermediateResults && value != int(value)) || (!rules.AllowNegativeIntermediate && value < 0) {
			return false
		}
		next := make([]friendSolveItem, 0, len(items)-1)
		for index, item := range items {
			if index != firstIndex && index != secondIndex {
				next = append(next, item)
			}
		}
		next = append(next, friendSolveItem{value: value, indices: append(append([]int(nil), first.indices...), second.indices...)})
		items = next
	}
	return len(items) == 1 && items[0].value == 24
}

func applyFriendOperator(left, right int, operator string) (int, bool) {
	// Accept the canonical symbols emitted by the WeChat client as well as
	// the legacy aliases kept for older stored submissions.
	if operator == "\u00d7" || operator == "*" {
		return left * right, true
	}
	if operator == "\u00f7" || operator == "/" {
		if right == 0 || left%right != 0 {
			return 0, false
		}
		return left / right, true
	}
	switch operator {
	case "+":
		return left + right, true
	case "-":
		return left - right, true
	case "×", "*":
		return left * right, true
	case "÷", "/":
		if right == 0 || left%right != 0 {
			return 0, false
		}
		return left / right, true
	default:
		return 0, false
	}
}

// puzzleQuestionHash is a stable SHA-256 fingerprint for a server-owned
// question contract. The existing friend-room envelope keeps its short
// fingerprint for compatibility; single-player runs can expose this full
// hash to let clients detect a stale or forged question.
func puzzleQuestionHash(numbers []int, rules FriendPuzzleRules, sourceSeed int64, index int) string {
	payload, _ := json.Marshal(struct {
		Index      int               `json:"index"`
		Numbers    []int             `json:"numbers"`
		Rules      FriendPuzzleRules `json:"rules"`
		SourceSeed int64             `json:"source_seed"`
	}{Index: index, Numbers: numbers, Rules: rules, SourceSeed: sourceSeed})
	digest := sha256.Sum256(payload)
	return fmt.Sprintf("%x", digest[:])
}
