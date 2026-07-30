package domain

type User struct {
	Id        int64
	Nickname  string
	Email     string
	Password  string
	CreatedAt int64
	UpdatedAt int64
}
