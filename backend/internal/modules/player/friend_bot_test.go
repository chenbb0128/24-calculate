package player

import "testing"

func TestBotProgressAdvancesAtMostOneQuestionPerPoll(t *testing.T) {
	first, _ := botProgressForElapsed(0, 60_000, 8, 12345, 1)
	if first != 1 {
		t.Fatalf("first bot poll solved = %d, want 1", first)
	}
	second, _ := botProgressForElapsed(first, 60_000, 8, 12345, 1)
	if second != 2 {
		t.Fatalf("second bot poll solved = %d, want 2", second)
	}
}
