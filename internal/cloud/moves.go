package cloud

import (
	"errors"
	"fmt"
	"net/url"
)

// A team moving to R3V Cloud (docs/design/moving.md): the app plans which
// contents of the team's old storage each project needs; the service
// copies them with a read-only key it is given, in the background. And the
// old members, imported to be claimed by whoever they were (R3V-Cloud's
// docs/api.md, "A team moving in" and "Members to claim").

// MoveSource is the team's old storage, as the service reads it.
type MoveSource struct {
	Endpoint  string `json:"endpoint"` // https://<account>.r2.cloudflarestorage.com, https://s3.<region>.amazonaws.com…
	Region    string `json:"region"`   // "auto" for R2
	Bucket    string `json:"bucket"`
	Prefix    string `json:"prefix"` // the team's folder in the bucket, "" for none
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
}

// ErrMoveCantRead: the service can't read the old storage with the key
// given (wrong key, a private network): the move goes through the app.
var ErrMoveCantRead = errors.New("R3V Cloud can't read the team's storage with this key")

func movePath(team, move string) string { return teamPath(team) + "/moves/" + url.PathEscape(move) }

// StartMove gives the service a read-only key to the team's old storage;
// it checks it can read the team's team.json. Owners and admins.
func StartMove(service, team string, src MoveSource) (string, error) {
	tok, err := authed(service)
	if err != nil {
		return "", err
	}
	var out struct{ Move string }
	err = call(service, tok, "POST", teamPath(team)+"/moves", map[string]any{"source": src}, &out)
	var se *serviceError
	if errors.As(err, &se) && se.Code == "cant_read" {
		return "", fmt.Errorf("%w (%s)", ErrMoveCantRead, se.Message)
	}
	return out.Move, err
}

// PlanItem is something the move copies.
type PlanItem struct {
	Src  string `json:"src"`  // a key in the bucket, the prefix included
	Dst  string `json:"dst"`  // in the project: objects/ab/…, chunked/<hash>, snapshots/<id>.json
	Size int64  `json:"size"` // as stored
}

const planBatch = 1000

// AddPlan adds a project's items, in order (sent 1,000 at a time). Items
// already in this move, or in the project, are skipped. They are copied
// in this order: an item under chunked/ (and so the list after it) only
// once everything before it in the project is.
func AddPlan(service, team, move, project string, items []PlanItem) (added, skipped int, err error) {
	tok, err := authed(service)
	if err != nil {
		return 0, 0, err
	}
	for i := 0; i < len(items); i += planBatch {
		var out struct{ Added, Skipped int }
		in := map[string]any{"project": project, "items": items[i:min(i+planBatch, len(items))]}
		if err := call(service, tok, "POST", movePath(team, move)+"/plan", in, &out); err != nil {
			return added, skipped, err
		}
		added, skipped = added+out.Added, skipped+out.Skipped
	}
	return added, skipped, nil
}

// StartCopy starts copying what the plans list, or goes on: after a later
// AddPlan (the pass after the freeze), after a pause, and failed items are
// tried again.
func StartCopy(service, team, move string) error {
	return do(service, "POST", movePath(team, move)+"/start", nil)
}

// MoveStatus is how a move is going.
type MoveStatus struct {
	// State: "planning" (not started), "copying", "paused", "done",
	// "failed" (all tried, some failed: StartCopy tries them again),
	// "cancelled".
	State string `json:"state"`
	// Reason: with paused, "over_limit"; with cancelled, "cancelled" or
	// "key_forgotten" (left alone a day: start the move again).
	Reason    string        `json:"reason"`
	Items     int           `json:"items"`
	ItemsDone int           `json:"itemsDone"` // copied, or there already
	Bytes     int64         `json:"bytes"`
	BytesDone int64         `json:"bytesDone"`
	Failed    []MoveFailure `json:"failed"` // the first 100
}

// MoveFailure is an item that wasn't copied.
type MoveFailure struct {
	Src   string `json:"src"`
	Error string `json:"error"`
}

// Move says how a move is going.
func Move(service, team, move string) (*MoveStatus, error) {
	tok, err := authed(service)
	if err != nil {
		return nil, err
	}
	var st MoveStatus
	if err := call(service, tok, "GET", movePath(team, move), nil, &st); err != nil {
		return nil, err
	}
	if st.Failed == nil {
		st.Failed = []MoveFailure{}
	}
	return &st, nil
}

// CancelMove stops copying; progress and the key stay (StartCopy goes on).
func CancelMove(service, team, move string) error {
	return do(service, "POST", movePath(team, move)+"/cancel", nil)
}

// ForgetMove stops it and has the service forget the key and the plan
// (what was copied stays): at the switch, or when the move is given up.
func ForgetMove(service, team, move string) error {
	return do(service, "DELETE", movePath(team, move), nil)
}

// ImportedMember is a member of the team's old storage, to be claimed.
type ImportedMember struct {
	ID      string `json:"id"` // the old member id: theirs in the hosted team once claimed
	Name    string `json:"name"`
	Color   string `json:"color,omitempty"`
	Picture string `json:"picture,omitempty"` // the picture's SHA-256 (written as usual)
}

// ImportMembers adds the old members to be claimed (again with the same
// ids: names and looks brought up to date). Owners and admins.
func ImportMembers(service, team string, ms []ImportedMember) error {
	if ms == nil {
		ms = []ImportedMember{}
	}
	return do(service, "POST", teamPath(team)+"/members/import", ms)
}

// Claimable is an imported member, and whether someone claimed them.
type Claimable struct {
	ImportedMember
	Claimed bool `json:"claimed"`
}

// InvitationClaimables says what an invitation is for before accepting it:
// the team's name, and its members to claim ("Who were you?").
func InvitationClaimables(service, token string) (string, []Claimable, error) {
	tok, err := authed(service)
	if err != nil {
		return "", nil, err
	}
	var out struct {
		Team       string
		Claimables []struct {
			ImportedMember
			Color   *string `json:"color"`
			Picture *string `json:"picture"`
			Claimed bool    `json:"claimed"`
		}
	}
	if err := call(service, tok, "GET", "/v1/invitations/"+url.PathEscape(token), nil, &out); err != nil {
		return "", nil, err
	}
	ms := []Claimable{}
	for _, c := range out.Claimables {
		m := Claimable{ImportedMember: c.ImportedMember, Claimed: c.Claimed}
		m.Color, m.Picture = deref(c.Color), deref(c.Picture)
		ms = append(ms, m)
	}
	return out.Team, ms, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// AcceptAs accepts an invitation as one of the team's imported members
// (claim "": as a new member, Accept), and brings the team list up to
// date; it returns the team's address.
func AcceptAs(service, token, claim string) (string, error) {
	tok, err := authed(service)
	if err != nil {
		return "", err
	}
	var in any
	if claim != "" {
		in = map[string]string{"claim": claim}
	}
	var out struct{ TeamID string }
	if err := call(service, tok, "POST", "/v1/invitations/"+url.PathEscape(token)+"/accept", in, &out); err != nil {
		return "", err
	}
	if _, err := SyncTeams(service); err != nil {
		return "", err
	}
	return TeamAddress(service, out.TeamID), nil
}

// Claim makes someone in the team already (the owner who ran the move)
// one of its imported members: their member id there becomes id. The
// team list follows.
func Claim(service, team, id string) error {
	if err := do(service, "POST", teamPath(team)+"/members/claim", map[string]string{"id": id}); err != nil {
		return err
	}
	_, err := SyncTeams(service)
	return err
}

// TeamClaimables lists a team's imported members, claimed or not.
func TeamClaimables(service, team string) ([]Claimable, error) {
	type row struct {
		ImportedMember
		Color   *string `json:"color"`
		Picture *string `json:"picture"`
		Claimed bool    `json:"claimed"`
	}
	rows, err := list[row](service, teamPath(team)+"/members/import")
	if err != nil {
		return nil, err
	}
	ms := make([]Claimable, len(rows))
	for i, r := range rows {
		ms[i] = Claimable{ImportedMember: r.ImportedMember, Claimed: r.Claimed}
		ms[i].Color, ms[i].Picture = deref(r.Color), deref(r.Picture)
	}
	return ms, nil
}
