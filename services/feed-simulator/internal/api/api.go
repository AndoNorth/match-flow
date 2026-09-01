// Package api registers Feed Simulator's control routes - start/stop
// the auto-respawn loop, trigger one extra match on demand, and list
// available templates. Nothing here touches match generation itself,
// only internal/simulator/manager does that.
package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/template"
)

// controller is the subset of *manager.Manager the routes need -
// letting tests inject a fake instead of a real manager.
type controller interface {
	Start(ctx context.Context, templateName string) (string, error)
	Stop()
	Trigger(ctx context.Context, templateName string) (string, error)
	RunningCount() int
	TemplatesDir() string
}

type templateInput struct {
	Body struct {
		Template string `json:"template,omitempty" doc:"Template name, or omit for the default unbounded-random mode"`
	}
}

type matchStartedOutput struct {
	Body struct {
		MatchID string `json:"match_id"`
	}
}

type statusOutput struct {
	Body struct {
		Running int `json:"running"`
	}
}

type templatesOutput struct {
	Body struct {
		Templates []string `json:"templates"`
	}
}

// Register wires every control route onto api.
func Register(api huma.API, m controller) {
	huma.Register(api, huma.Operation{
		OperationID: "start",
		Method:      http.MethodPost,
		Path:        "/control/start",
		Summary:     "Turn on the auto-respawn loop, starting one match immediately if none are running",
	}, func(ctx context.Context, in *templateInput) (*matchStartedOutput, error) {
		matchID, err := m.Start(ctx, in.Body.Template)
		if err != nil {
			return nil, huma.Error400BadRequest("failed to start", err)
		}
		out := &matchStartedOutput{}
		out.Body.MatchID = matchID
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "stop",
		Method:      http.MethodPost,
		Path:        "/control/stop",
		Summary:     "Turn off the auto-respawn loop and cancel every running match",
	}, func(_ context.Context, _ *struct{}) (*statusOutput, error) {
		m.Stop()
		out := &statusOutput{}
		out.Body.Running = m.RunningCount()
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "trigger",
		Method:      http.MethodPost,
		Path:        "/matches/trigger",
		Summary:     "Start exactly one additional match right now, independent of the auto-loop",
	}, func(ctx context.Context, in *templateInput) (*matchStartedOutput, error) {
		matchID, err := m.Trigger(ctx, in.Body.Template)
		if err != nil {
			return nil, huma.Error400BadRequest("failed to trigger match", err)
		}
		out := &matchStartedOutput{}
		out.Body.MatchID = matchID
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-templates",
		Method:      http.MethodGet,
		Path:        "/templates",
		Summary:     "List available match template names",
	}, func(_ context.Context, _ *struct{}) (*templatesOutput, error) {
		templates, err := template.LoadAll(m.TemplatesDir())
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list templates", err)
		}
		out := &templatesOutput{}
		out.Body.Templates = make([]string, 0, len(templates))
		for name := range templates {
			out.Body.Templates = append(out.Body.Templates, name)
		}
		return out, nil
	})
}
