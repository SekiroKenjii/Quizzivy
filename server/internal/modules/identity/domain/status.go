package domain

// AccountStatus is the one state an account is listed under (DG-54).
type AccountStatus string

const (
	AccountDisabled           AccountStatus = "disabled"
	AccountMustChangePassword AccountStatus = "must_change_password"
	AccountNeverSignedIn      AccountStatus = "never_signed_in"
	AccountActive             AccountStatus = "active"
)

// StatusOf applies DG-54's order: a disabled account is disabled whatever else is true of it, then one on a temporary password must change it, then one that has never signed in is not signed in yet, and any other is active.
func StatusOf(disabled, mustChangePassword, signedIn bool) AccountStatus {
	switch {
	case disabled:
		return AccountDisabled
	case mustChangePassword:
		return AccountMustChangePassword
	case !signedIn:
		return AccountNeverSignedIn
	default:
		return AccountActive
	}
}

// Status is the account's AccountStatus.
func (u User) Status() AccountStatus {
	return StatusOf(u.Disabled(), u.MustChangePassword, u.LastSignInAt != nil)
}

// Status is the student's AccountStatus.
func (s Student) Status() AccountStatus {
	return StatusOf(s.DisabledAt != nil, s.MustChangePassword, s.LastSignInAt != nil)
}
