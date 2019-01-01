package repo

// Branch ...
type Branch struct {
	Commit string
	Name   string
}

// Branches ...
type Branches []Branch

// Rev ...
type Rev struct {
	Commit  string
	Email   string
	Comment string
	State   string
}

// Revs ...
type Revs []Rev
