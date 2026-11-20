package input

// UserCreator is the input of UserCreator.Create.
type UserCreator struct {
	Actor     string
	Email     string
	Name      string
	Password  string
	CompanyID string
}

// UserLister is the input of UserLister.List.
type UserLister struct {
	Actor string
}

// UserUpdater is the input of UserUpdater.UpdateName.
type UserUpdater struct {
	Actor string
	ID    string
	Name  string
}
