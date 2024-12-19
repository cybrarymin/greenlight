package data

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type PermissionModel struct {
	db *bun.DB
}

type Permissions []Permission

type Permission struct {
	bun.BaseModel `bun:"table:permissions"`
	ID            int64  `bun:",pk,notnull,autoincrement,type:bigint"`
	Code          string `bun:",notnull,type:text"`
	User          []User `json:"-" bun:",m2m:user_permissions,join:Permission=User"`
}

// junction table for many-to-many relationship
type UserPermission struct {
	User         *User       `bun:",rel:belongs-to,join:user_id=id"`
	UserID       uuid.UUID   `bun:",pk"`
	Permission   *Permission `bun:",rel:belongs-to,join:permission_id=id"`
	PermissionID int64       `bun:",pk"`
}

func (prems *Permissions) IncludesPrem(premCode string) bool {
	for _, v := range *prems {
		if premCode == v.Code {
			return true
		}
	}
	return false
}

func (p *PermissionModel) GetAllPermsForUser(ctx context.Context, userID uuid.UUID) (*Permissions, error) {
	nUser := &User{}
	timeoutCtx, cancelFunc := context.WithTimeout(ctx, time.Second*3)
	defer cancelFunc()

	err := p.db.NewSelect().Model(nUser).Relation("Permission").Where("id = ?", userID).Scan(timeoutCtx)
	if err != nil {
		switch {
		case errors.Is(err, ErrorRecordNotFound):
			return nil, ErrorRecordNotFound
		default:
			return nil, err
		}
	}
	return (*Permissions)(&nUser.Permission), nil
}

func (p *PermissionModel) AddPermForUser(ctx context.Context, userID uuid.UUID, perms ...string) error {
	permsObj, err := p.GetPermID(ctx, perms)
	if err != nil {
		return err
	}

	nUserPerm := []UserPermission{}
	for _, v := range *permsObj {
		nPerm := UserPermission{
			UserID:       userID,
			PermissionID: v.ID,
		}
		nUserPerm = append(nUserPerm, nPerm)
	}
	timeoutCtx, cancelFunc := context.WithTimeout(ctx, time.Second*3)
	defer cancelFunc()

	_, err = p.db.NewInsert().Model(&nUserPerm).Exec(timeoutCtx)
	if err != nil {
		return err
	}
	return nil
}

func (p *PermissionModel) GetPermID(ctx context.Context, permCode []string) (*Permissions, error) {
	perms := &Permissions{}
	timeoutCtx, cancelFunc := context.WithTimeout(ctx, time.Second*3)
	defer cancelFunc()

	err := p.db.NewSelect().Model(perms).Where("code IN (?)", bun.In(permCode)).Scan(timeoutCtx)
	if err != nil {
		switch {
		case errors.Is(err, ErrorRecordNotFound):
			return nil, ErrorRecordNotFound
		default:
			return nil, err
		}
	}
	return perms, nil
}
