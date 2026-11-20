package input

// MemberAdder is the input of MemberAdder.Add.
type MemberAdder struct {
	Actor     string
	TenantID  string
	UserID    string
	Email     string
	Name      string
	Password  string
	CompanyID string
	Role      string
}

// MemberLister is the input of MemberLister.List.
type MemberLister struct {
	Actor       string
	TenantID    string
	Affiliation string
}

// MemberRemover is the input of MemberRemover.Remove.
type MemberRemover struct {
	Actor    string
	TenantID string
	UserID   string
}

// MemberRoleChanger is the input of MemberRoleChanger.ChangeRole.
type MemberRoleChanger struct {
	Actor    string
	TenantID string
	UserID   string
	Role     string
}
