// Package handler exposes Sandrone as a Vercel Go Function.
package handler

import (
	"net/http"

	"github.com/kuuvahki-labs/sandrone/pkg/vercelhandler"
)

// Handler delegates to a package inside the Sandrone module. Vercel compiles
// this entrypoint as handler/api in a generated module, so this package must not
// import Sandrone's internal packages directly.
func Handler(w http.ResponseWriter, r *http.Request) {
	vercelhandler.Handler(w, r)
}
