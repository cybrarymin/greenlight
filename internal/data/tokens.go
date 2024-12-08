package data

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

const (
	ActivationScope     = "activation"
	AuthenticationScope = "BearerAuthentication"
)

type TokenModel struct {
	db *bun.DB
}

type Tokens []*Token

type Token struct {
	bun.BaseModel `bun:"table:tokens"`
	PlainText     string    `json:"token" bun:"-"` // ignoring this field
	Hash          []byte    `json:"-" bun:",pk,notnull,type:bytea"`
	UserID        uuid.UUID `json:"-"`
	User          *User     `json:"-" bun:"rel:belongs-to,join:user_id=id"`
	Expiry        time.Time `json:"expiry" bun:",notnull,type:timestamptz"`
	Scope         string    `json:"scope" bun:",type:text,notnull"`
}

func generateToken(userID uuid.UUID, ttl time.Duration, scope string) (*Token, error) {
	nToken := &Token{
		Expiry: time.Now().Add(ttl),
		UserID: userID,
		Scope:  scope,
	}

	bs := make([]byte, 16)
	_, err := rand.Read(bs)
	if err != nil {
		return nil, err
	}
	// use base32 to avoid special and non interpretable characters read by crypto rand function
	tokenString := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bs)
	nToken.PlainText = tokenString

	hash := sha256.Sum256([]byte(tokenString))
	nToken.Hash = hash[:]

	return nToken, nil
}

func (t Tokens) Match(token string) (*Token, bool) {
	hash := sha256.Sum256([]byte(token))
	for _, v := range t {
		if bytes.Equal(v.Hash, hash[:]) {
			return v, true
		}
	}
	return nil, false
}

func (tm TokenModel) New(ctx context.Context, ttl time.Duration, userID uuid.UUID, tokenScope string) (*Token, error) {
	nToken, err := generateToken(userID, ttl, tokenScope)
	if err != nil {
		return nil, err
	}
	err = tm.InsertToken(ctx, nToken)
	if err != nil {
		return nil, err
	}

	return nToken, nil
}

func (tm TokenModel) InsertToken(ctx context.Context, t *Token) error {
	timeoutCtx, cancelFunc := context.WithTimeout(ctx, time.Second*5)
	defer cancelFunc()
	_, err := tm.db.NewInsert().Model(t).Exec(timeoutCtx)
	if err != nil {
		return err
	}
	return nil
}

func (tm TokenModel) GetTokensOfUserID(ctx context.Context, userID uuid.UUID, tokenScope string) (*Tokens, error) {
	nTokens := &Tokens{}
	timeoutCtx, cancelFunc := context.WithTimeout(ctx, time.Second*3)
	defer cancelFunc()
	err := tm.db.NewSelect().Model(nTokens).Relation("User").Where("user_id = ? AND scope = ?", userID, tokenScope).Scan(timeoutCtx)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrorRecordNotFound
		default:
			return nil, err
		}
	}
	return nTokens, nil
}

func (tm TokenModel) DeleteAllForUser(ctx context.Context, userID uuid.UUID, scope string) error {
	timeoutCtx, cancelFunc := context.WithTimeout(ctx, time.Second*3)
	defer cancelFunc()
	result, err := tm.db.NewDelete().Model((*Token)(nil)).Where("user_id = ? AND scope = ?", userID, scope).Exec(timeoutCtx)
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrorRecordNotFound
	}
	if err != nil {
		return err
	}
	return nil
}

func ValidateTokenPlaintext(v *Validator, tokenPlaintext string) {
	v.Check(tokenPlaintext != "", "token", "must be provided")
	v.Check(len(tokenPlaintext) == 26, "token", "must be 26 bytes long")
}
