package data

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrorPasswordTooLong              = errors.New("user password is too long")
	ErrorDuplicateEmail               = errors.New("error user with same email already exist")
	_                    sql.Scanner  = (*Password)(nil)
	_                    driver.Value = (*Password)(nil)
	AnonymousUser                     = &User{}
)

type UserModel struct {
	db *bun.DB
}

// List of Users
type Users []User

// Define a User struct to represent an individual user. Importantly, notice how we are
// using the json:"-" struct tag to prevent the Password and Version fields appearing in
// any output when we encode it to JSON. Also notice that the Password field uses the
// custom password type defined below.
type User struct {
	bun.BaseModel `bun:"table:users"`
	ID            uuid.UUID    `json:"id" bun:",pk,notnull,type:uuid,default:gen_random_uuid()"`
	Name          string       `json:"name" bun:",notnull"`
	Password      Password     `json:"-" bun:"password_hash,type:bytea,notnull"`
	CreatedAt     time.Time    `json:"created_at,omitempty" bun:",type:timestamptz,notnull,default:current_timestamp()"`
	Activated     bool         `json:"activated" bun:",notnull,type:bool"`
	Email         string       `json:"email" bun:",type:ictext,unique"`
	Version       int          `json:"-" bun:",notnull,default:1"`
	Token         []*Token     `json:"-" bun:",rel:has-many,join:id=user_id"`
	Permission    []Permission `json:"-" bun:",m2m:user_permissions,join:User=Permission"`
}

func (u *User) IsAnonymous() bool {
	return u == AnonymousUser
}

type Password struct {
	Plaintext *string
	Hash      []byte
}

func (p *Password) Value() (driver.Value, error) {
	return p.Hash, nil
}
func (p *Password) Scan(src interface{}) error {
	p.Plaintext = nil
	p.Hash = src.([]byte)
	return nil
}

func (p *Password) Set(passString string) error {
	// consider a hard limit of length check for password. bcrypt will truncate the password plaintext bytes after the 72th byte so we should force client not to provde something more than that
	bcryptPass, err := bcrypt.GenerateFromPassword([]byte(passString), 12)
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrPasswordTooLong):
			return ErrorPasswordTooLong
		default:
			return err
		}
	}
	p.Plaintext = &passString
	p.Hash = bcryptPass
	return nil
}

func (p *Password) Match() (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.Hash, []byte(*p.Plaintext))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}
	return true, nil
}

func (u *UserModel) Insert(ctx context.Context, user *User) error {
	args := []interface{}{&user.ID, &user.Activated, &user.CreatedAt, &user.Version}
	timeoutCtx, cancelFunc := context.WithTimeout(ctx, time.Second*5)
	defer cancelFunc()
	err := u.db.NewInsert().Model(user).Returning("id, activated, created_at, version").Scan(timeoutCtx, args...)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "SQLSTATE=23505"):
			return ErrorDuplicateEmail
		default:
			return err
		}
	}
	return nil
}

func (u *UserModel) Update(id uuid.UUID, ctx context.Context, user *User) error {
	args := []interface{}{&user.CreatedAt, &user.Version}
	user.Version += 1
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	err := u.db.NewUpdate().Model(user).Where("id = ? and version = ?", id, user.Version-1).Returning("created_at, version").Scan(timeoutCtx, args...)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrorRecordNotFound
		case errors.Is(err, ErrorDuplicateEmail):
			return ErrorDuplicateEmail
		default:
			return err
		}
	}
	return nil
}

func (u *UserModel) GetByEmail(email string, ctx context.Context) (*User, error) {
	nUser := &User{}
	timeoutCtx, cancelFunc := context.WithTimeout(ctx, time.Second*5)
	defer cancelFunc()
	err := u.db.NewSelect().Model(nUser).Where("email = ?", email).Scan(timeoutCtx)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrorRecordNotFound
		default:
			return nil, err
		}
	}
	return nUser, nil
}

func (u *UserModel) GetByID(id uuid.UUID, ctx context.Context, user *User) error {
	timeoutCtx, cancelFunc := context.WithTimeout(ctx, time.Second*5)
	defer cancelFunc()
	err := u.db.NewSelect().Model((*User)(nil)).Where("id = ?", id).Scan(timeoutCtx, user)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrorRecordNotFound
		default:
			return err
		}
	}
	return nil
}

func (u *UserModel) List(ctx context.Context, users *Users, name string, email string, filters *Filters) (int, error) {
	timeoutCtx, cancelFunc := context.WithTimeout(ctx, time.Second*5)
	defer cancelFunc()

	orderQuery := filters.SortColumn() + " " + filters.SortDirection()
	count, err := u.db.NewSelect().Model(users).Where("((name LIKE ?) OR (? = '')) AND ((email LIKE ?) OR (? = ''))", fmt.Sprintf("%%%s%%", name), name, fmt.Sprintf("%%%s%%", email), email).Limit(filters.limit()).Offset(filters.offset()).OrderExpr(orderQuery).ScanAndCount(timeoutCtx)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return 0, ErrorRecordNotFound
		default:
			return 0, err
		}
	}

	return count, nil
}

func (u *UserModel) Delete(ctx context.Context, id uuid.UUID) error {
	timeoutCtx, cancelFunc := context.WithTimeout(ctx, time.Second*5)
	defer cancelFunc()
	result, err := u.db.NewDelete().Model((*User)(nil)).Where("id = ?", id).Exec(timeoutCtx)
	if ok, _ := result.RowsAffected(); ok == 0 {
		return ErrorRecordNotFound
	}
	if err != nil {
		return err
	}
	return nil
}

func (u *UserModel) GetUserByToken(ctx context.Context, tokenPlaintext string, tokenScope string) (*User, error) {
	ctx, span := otel.Tracer("database.tracer").Start(ctx, "database.getUserByToken.span")
	defer span.End()

	nToken := &Token{}
	timeoutCtx, cancelFunc := context.WithTimeout(ctx, time.Second*3)
	defer cancelFunc()

	hash := sha256.Sum256([]byte(tokenPlaintext))
	err := u.db.NewSelect().Model(nToken).Relation("User").Where("hash = ?", hash).Scan(timeoutCtx)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			span.RecordError(err)
			return nil, ErrorRecordNotFound
		default:
			span.RecordError(err)
			span.SetStatus(codes.Error, "error in interaction with database")
			return nil, err
		}
	}
	span.SetAttributes(attribute.String("user.name", nToken.User.Name))
	span.SetAttributes(attribute.String("user.email", nToken.User.Email))
	return nToken.User, nil
}

func ValidateEmail(v *Validator, email string) {
	v.Check(email != "", "email", "must be provided")
	v.Check(Matches(email, EmailRX), "email", "must be a valid email address")
}
func ValidatePasswordPlaintext(v *Validator, password string) {
	v.Check(password != "", "password", "must be provided")
	v.Check(len(password) >= 8, "password", "must be at least 8 bytes long")
	v.Check(len(password) <= 72, "password", "must not be more than 72 bytes long")
}
func ValidateUser(v *Validator, user *User) {
	v.Check(user.Name != "", "name", "must be provided")
	v.Check(len(user.Name) <= 500, "name", "must not be more than 500 bytes long")
	// Call the standalone ValidateEmail() helper.
	ValidateEmail(v, user.Email)
	// If the plaintext password is not nil, call the standalone
	// ValidatePasswordPlaintext() helper.
	if user.Password.Plaintext != nil {
		ValidatePasswordPlaintext(v, *user.Password.Plaintext)
	}
	// If the password hash is ever nil, this will be due to a logic error in our
	// codebase (probably because we forgot to set a password for the user). It's a
	// useful sanity check to include here, but it's not a problem with the data
	// provided by the client. So rather than adding an error to the validation map we
	// raise a panic instead.
	if user.Password.Hash == nil {
		panic("missing password hash for user")
	}
}
