// Package example demonstrates metadata declared next to Chi route registration.
package example

import (
	"net/http"

	"github.com/enzujp/walker/pkg/walker"
	"github.com/go-chi/chi/v5"
)

type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// SetupRouter constructs the demo without opening a network listener.
func SetupRouter() *chi.Mux { router, _ := setup(); return router }

func setup() (*chi.Mux, *walker.Docs) {
	router := chi.NewRouter()
	docs := &walker.Docs{}
	handler := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }
	router.Get("/health", handler)
	docs.Describe("GET", "/health", walker.Summary("Health check"), walker.Authentication(walker.Auth{Type: "noauth"}))
	router.Route("/users", func(r chi.Router) {
		r.Get("/", handler)
		docs.Describe("GET", "/users/",
			walker.Summary("List users"), walker.Group("Users/Read"),
			walker.Query(walker.QueryParam{Key: "limit", Value: "20", Description: "Page size"}, walker.QueryParam{Key: "cursor", Value: "{{cursor}}", Disabled: true}),
			walker.Headers(walker.Header{Key: "Accept", Value: "application/json"}),
		)
		r.Get("/{userID}", handler)
		docs.Describe("GET", "/users/{userID}",
			walker.Summary("Get a user"), walker.Group("Users/Read"), walker.PathParam("userID", "42"),
			walker.Examples(
				walker.RequestVariant{Name: "Ada", PathParams: map[string]string{"userID": "42"}},
				walker.RequestVariant{Name: "Grace", PathParams: map[string]string{"userID": "84"}},
			),
		)
		r.Post("/", handler)
		docs.Describe("POST", "/users/",
			walker.Summary("Create a user"), walker.Group("Users/Write"),
			walker.Description("Illustrative request bodies; the demo handler returns 204 and does not persist users."),
			walker.Headers(walker.Header{Key: "X-Request-ID", Value: "{{request_id}}"}),
			walker.Examples(
				walker.RequestVariant{Name: "Ada", Body: CreateUserRequest{Name: "Ada", Email: "ada@example.com"}},
				walker.RequestVariant{Name: "Grace", Body: CreateUserRequest{Name: "Grace", Email: "grace@example.com"}},
			),
		)
	})
	return router, docs
}

// Routes demonstrates extraction with validated, explicitly attached metadata.
func Routes() ([]walker.Route, error) { router, docs := setup(); return docs.Extract(router) }
