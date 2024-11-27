package data

import "github.com/uptrace/bun"

type Models struct {
	Movies MovieModel
	Users  UserModel
}

func NewModels(db *bun.DB) *Models {
	return &Models{
		Movies: MovieModel{
			db,
		},
		Users: UserModel{
			db,
		},
	}
}
