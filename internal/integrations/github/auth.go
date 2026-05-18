// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-05-18

package github

import (
	"context"
	"net/http"
	"time"

	"github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"
)

const githubRequestTimeout = 30 * time.Second

// NewClient creates a new GitHub client using the provided token.
// If token is empty, it returns an unauthenticated client.
func NewClient(ctx context.Context, token string) *Client {
	var tc *http.Client
	var graphql *GraphQLClient

	if token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		base := oauth2.NewClient(ctx, ts)
		tc = &http.Client{
			Transport: base.Transport,
			Timeout:   githubRequestTimeout,
		}
		// Initialize GraphQL client for authenticated operations
		graphql = NewGraphQLClient(tc, token)
	} else {
		tc = &http.Client{Timeout: githubRequestTimeout}
	}

	client := github.NewClient(tc)

	return &Client{
		client:  client,
		graphql: graphql,
	}
}
