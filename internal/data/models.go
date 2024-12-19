package data

import "github.com/uptrace/bun"

type Models struct {
	Movies      MovieModel
	Users       UserModel
	Tokens      TokenModel
	Permissions PermissionModel
}

func NewModels(db *bun.DB) *Models {
	db.RegisterModel((*UserPermission)(nil))
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
		Permissions: PermissionModel{
			db,
		},
	}
}
