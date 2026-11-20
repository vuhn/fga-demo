package input

// Authenticator is the input of Authenticator.Login.
type Authenticator struct {
	Email    string
	Password string
}

// ProfileGetter is the input of ProfileGetter.Me.
type ProfileGetter struct {
	Actor string
}

// TokenVerifier is the input of TokenVerifier.Verify.
type TokenVerifier struct {
	Token string
}
