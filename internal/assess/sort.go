package assess

import "sort"

// sortFindings ranks findings most-severe first, stable by title.
func sortFindings(f []Finding) {
	sort.SliceStable(f, func(i, j int) bool {
		if f[i].Level != f[j].Level {
			return f[i].Level > f[j].Level
		}
		return f[i].Title < f[j].Title
	})
}
