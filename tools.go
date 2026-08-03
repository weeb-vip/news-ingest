//go:build tools

// Pins the gqlgen code generator as a module dependency so `go run` uses the same version
// everywhere, rather than whatever a developer has installed globally.
package main

import _ "github.com/99designs/gqlgen"
