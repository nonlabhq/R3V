package cloud

import "strconv"

// Event is a thing that happened in a hosted team, as the service tells
// it ready made (R3V-Cloud's docs/api.md, GET …/events): what the app's
// Activity makes of the team log and the projects' branch logs, titles,
// merges and agents included.
type Event struct {
	Time        string `json:"time"`
	Kind        string `json:"kind"`
	Project     string `json:"project"`
	ProjectName string `json:"projectName"`
	Branch      string `json:"branch"`
	Version     string `json:"version"`
	Title       string `json:"title"`
	By          string `json:"by"`
	ByName      string `json:"byName"`
	Tool        string `json:"tool"`
	Model       string `json:"model"`
	// Section, Change and Was: a sections-changed item's (see the app's
	// ActivityItem), when the service names them.
	Section string `json:"section"`
	Change  string `json:"change"`
	Was     string `json:"was"`
}

// Events asks the service for a team's Activity of the last days, newest
// first (at most 500). One request, instead of reading every project's
// branch log and the versions it names.
func Events(service, team string, days int) ([]Event, error) {
	return list[Event](service, teamPath(team)+"/events?days="+strconv.Itoa(days))
}
