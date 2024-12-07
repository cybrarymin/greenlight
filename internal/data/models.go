package data

import "github.com/uptrace/bun"

type Models struct {
	Movies MovieModel
	Users  UserModel
	Tokens TokenModel
}

func NewModels(db *bun.DB) *Models {
	return &Models{
		Movies: MovieModel{
			db,
		},
		Users: UserModel{
			db,
		},
		Tokens: TokenModel{
			db,
		},
	}
}
