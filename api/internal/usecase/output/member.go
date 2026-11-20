package output

// MemberAdder is the result of MemberAdder.Add.
type MemberAdder struct {
	Membership
}

// MemberLister is the result of MemberLister.List.
type MemberLister struct {
	Items []Membership
}

// MemberRoleChanger is the result of MemberRoleChanger.ChangeRole.
type MemberRoleChanger struct {
	Membership
}

// Membership is the usecase output copy of a membership.
type Membership struct {
	TenantID     string
	UserID       string
	Affiliation  string
	CompanyID    string
	CompanyName  string
	Role         string
	RoleName     string
	CustomRoleID string
	Email        string
	Name         string
}
