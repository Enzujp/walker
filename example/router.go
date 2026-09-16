// Package example demonstrates metadata declared next to Chi route registration.
package example

import (
	"net/http"

	"github.com/enzujp/walker/pkg/walker"
)

type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// SetupRouter constructs the demo without opening a network listener.
func SetupRouter() *walker.Router {
	router := walker.NewRouter()
	handler := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }
	router.Get("/health", handler, walker.Summary("Health check"), walker.NoAuth())
	router.Route("/users", func(users *walker.Router) {
		users.Get("/", handler,
			walker.Summary("List users"),
			walker.Query(walker.QueryParam{Key: "limit", Value: "20", Description: "Page size"}, walker.QueryParam{Key: "cursor", Value: "{{cursor}}", Disabled: true}),
			walker.HeaderValue("Accept", "application/json"),
		)
		users.Get("/{userID}", handler,
			walker.Summary("Get a user"), walker.PathValue("userID", "42"),
			walker.Examples(
				walker.RequestVariant{Name: "Ada", PathParams: map[string]string{"userID": "42"}},
				walker.RequestVariant{Name: "Grace", PathParams: map[string]string{"userID": "84"}},
			),
		)
		users.Post("/", handler,
			walker.Summary("Create a user"),
			walker.Description("Illustrative request bodies; the demo handler returns 204 and does not persist users."),
			walker.HeaderValue("X-Request-ID", "{{request_id}}"),
			walker.Examples(
				walker.RequestVariant{Name: "Ada", Body: CreateUserRequest{Name: "Ada", Email: "ada@example.com"}},
				walker.RequestVariant{Name: "Grace", Body: CreateUserRequest{Name: "Grace", Email: "grace@example.com"}},
			),
		)
	}, walker.Group("Users"))
	return router
}

// Routes demonstrates extraction with validated, explicitly attached metadata.
func Routes() ([]walker.Route, error) { return SetupRouter().Routes() }
