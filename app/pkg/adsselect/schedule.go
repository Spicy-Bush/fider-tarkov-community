package adsselect

// Shares include unsold inventory and sum to 100.
func Schedule(shares []int) [100]int {
	var slots [100]int
	scores := make([]int, len(shares))
	for slot := range slots {
		winner := 0
		for i, share := range shares {
			scores[i] += share
			if scores[i] > scores[winner] {
				winner = i
			}
		}

		scores[winner] -= 100
		slots[slot] = winner
	}

	return slots
}
