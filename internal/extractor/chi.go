package extractor

import (
	"fmt"
	"github.com/go-chi/chi/v5"
	"net/http"
)

// Base CLI Command

func ExtractRoutes(r chi.Router) {
	err := chi.Walk(r, func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		fmt.Printf("Method: %s, Route: %s", method, route)
		return nil
	})
	if err != nil {
		return
	}
}
