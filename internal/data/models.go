package data

import "github.com/uptrace/bun"

type Models struct {
	Movies MovieModel
}

func NewModels(db *bun.DB) *Models {
	return &Models{
		Movies: MovieModel{
			db,
		},
	}
}
