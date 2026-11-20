package output

import (
	"time"
)

// User is the usecase output copy of a user.
type User struct {
	ID            string
	Email         string
	Name          string
	IsSystemAdmin bool
	CompanyID     string
	CompanyName   string
	CreatedAt     time.Time
}

// UserCreator is the result of UserCreator.Create.
type UserCreator struct {
	User
}

// UserLister is the result of UserLister.List.
type UserLister struct {
	Items []User
}

// UserUpdater is the result of UserUpdater.UpdateName.
type UserUpdater struct {
	User
}
