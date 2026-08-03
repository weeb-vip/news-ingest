package graph

// deref applies the schema default when a caller omits an optional argument: gqlgen passes
// a nil pointer rather than the declared default.
//
// It lives here rather than in schema.resolvers.go because gqlgen rewrites that file on
// every generate and moves unrecognised code to the bottom — which it did once already.
func deref(v *int, fallback int) int {
	if v == nil {
		return fallback
	}
	return *v
}
