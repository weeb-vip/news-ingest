package graph

import "github.com/weeb-vip/news-ingest/internal/store"

// Resolver holds what the resolvers need. Dependencies are injected rather than reached for
// globally so the resolvers can be tested against a store double.
type Resolver struct {
	Store *store.Store
}
