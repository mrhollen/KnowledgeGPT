package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/mrhollen/KnowledgeGPT/internal/db"
	"github.com/mrhollen/KnowledgeGPT/internal/models"
)

type AccessTokenAuthorizer struct {
	DB           *db.PostgresDB
	accessTokens *[]models.AccessToken
}

func NewAccessTokenAuthorizer(db *db.PostgresDB) *AccessTokenAuthorizer {
	return &AccessTokenAuthorizer{
		DB:           db,
		accessTokens: nil,
	}
}

func (a *AccessTokenAuthorizer) CheckToken(accessTokenValue string) (bool, int64, error) {
	if a.accessTokens == nil {
		db := *a.DB
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		defer cancel()

		accessTokens, err := db.GetAccessTokens(ctx)
		if err != nil {
			return false, 0, fmt.Errorf("could not fetch access tokens %w", err)
		}

		a.accessTokens = &accessTokens
	}

	for _, token := range *a.accessTokens {
		if token.Token == accessTokenValue {
			return true, token.UserID, nil
		}

		fmt.Printf("%s and %s are not equal", token.Token, accessTokenValue)
	}

	return false, 0, nil
}
