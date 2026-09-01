package football

import "math/rand"

// A tiny local combinator - no external name generator or dependency,
// no real club names (this is simulated data, not a licensed feed).
// 20 prefixes x 8 suffixes = 160 combinations, plenty for two distinct
// names per match.
var namePrefixes = []string{
	"Ashford", "Brightwell", "Castlemere", "Denbury", "Eastholm", "Foxwick",
	"Greymoor", "Harrowgate", "Ironbridge", "Kingswell", "Lambourne", "Millhaven",
	"Northgate", "Oakfield", "Pinehurst", "Ravenscroft", "Silverdale", "Thornbury",
	"Underwood", "Westbrook",
}

var nameSuffixes = []string{
	"United", "City", "Athletic", "Rovers", "Town", "Wanderers", "Albion", "FC",
}

func randomClubName(rng *rand.Rand) string {
	prefix := namePrefixes[rng.Intn(len(namePrefixes))]
	suffix := nameSuffixes[rng.Intn(len(nameSuffixes))]
	return prefix + " " + suffix
}

// randomTeamNames returns two distinct club names for one match. A
// short retry loop on collision is simpler than hand-deduplicating the
// slices, and cheap - 160 combinations makes a collision on 2 draws rare.
func randomTeamNames(rng *rand.Rand) (home, away string) {
	home = randomClubName(rng)
	for {
		away = randomClubName(rng)
		if away != home {
			return home, away
		}
	}
}
