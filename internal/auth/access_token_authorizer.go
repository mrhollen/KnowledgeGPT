package auth

import (
	"fmt"

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

func (a *AccessTokenAuthorizer) CheckToken(accessTokenValue string) (bool, error) {
	if a.accessTokens == nil {
		db := *a.DB

		accessTokens, err := db.GetAccessTokens()
		if err != nil {
			return false, fmt.Errorf("could not fetch access tokens %w", err)
		}

		a.accessTokens = accessTokens
	}

	for _, token := range *a.accessTokens {
		if token.Token == accessTokenValue {
			return true, nil
		}

		fmt.Printf("%s and %s are not equal", token.Token, accessTokenValue)
	}

	return false, nil
}
