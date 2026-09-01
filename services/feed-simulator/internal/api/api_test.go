package api_test

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/api"
)

var errUnknownTemplate = errors.New("unknown template")

type fakeController struct {
	startCalledWith   string
	triggerCalledWith string
	stopCalled        bool
	running           int
	templatesDir      string
	failStart         bool
	failTrigger       bool
}

func (f *fakeController) Start(_ context.Context, templateName string) (string, error) {
	f.startCalledWith = templateName
	if f.failStart {
		return "", errUnknownTemplate
	}
	return "match-1", nil
}

func (f *fakeController) Stop() {
	f.stopCalled = true
}

func (f *fakeController) Trigger(_ context.Context, templateName string) (string, error) {
	f.triggerCalledWith = templateName
	if f.failTrigger {
		return "", errUnknownTemplate
	}
	return "match-2", nil
}

func (f *fakeController) RunningCount() int { return f.running }

func (f *fakeController) TemplatesDir() string { return f.templatesDir }

var _ = Describe("Register", func() {
	var (
		ctrl    *fakeController
		testAPI humatest.TestAPI
	)

	BeforeEach(func() {
		ctrl = &fakeController{}
		_, testAPI = humatest.New(GinkgoT(), huma.DefaultConfig("test", "0.0.0"))
		api.Register(testAPI, ctrl)
	})

	Describe("POST /control/start", func() {
		It("starts with the given template and returns the new match_id", func() {
			resp := testAPI.Post("/control/start", map[string]any{"template": "goalless_draw"})
			Expect(resp.Code).To(Equal(http.StatusOK))
			Expect(ctrl.startCalledWith).To(Equal("goalless_draw"))
			Expect(resp.Body.String()).To(ContainSubstring("match-1"))
		})

		It("starts with no template for the default random mode", func() {
			resp := testAPI.Post("/control/start", map[string]any{})
			Expect(resp.Code).To(Equal(http.StatusOK))
			Expect(ctrl.startCalledWith).To(Equal(""))
		})

		It("returns 400 when the manager rejects the template", func() {
			ctrl.failStart = true
			resp := testAPI.Post("/control/start", map[string]any{"template": "does-not-exist"})
			Expect(resp.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("POST /control/stop", func() {
		It("stops the manager and reports how many matches remain", func() {
			ctrl.running = 0
			resp := testAPI.Post("/control/stop", map[string]any{})
			Expect(resp.Code).To(Equal(http.StatusOK))
			Expect(ctrl.stopCalled).To(BeTrue())
			Expect(resp.Body.String()).To(ContainSubstring(`"running":0`))
		})
	})

	Describe("POST /matches/trigger", func() {
		It("triggers one match and returns its match_id", func() {
			resp := testAPI.Post("/matches/trigger", map[string]any{"template": "high_scoring_chaos"})
			Expect(resp.Code).To(Equal(http.StatusOK))
			Expect(ctrl.triggerCalledWith).To(Equal("high_scoring_chaos"))
			Expect(resp.Body.String()).To(ContainSubstring("match-2"))
		})

		It("returns 400 when the manager rejects the template", func() {
			ctrl.failTrigger = true
			resp := testAPI.Post("/matches/trigger", map[string]any{"template": "does-not-exist"})
			Expect(resp.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("GET /templates", func() {
		It("lists every template name in the templates directory", func() {
			dir := GinkgoT().TempDir()
			Expect(
				os.WriteFile(filepath.Join(dir, "a.json"), []byte(`{"name":"alpha","kind":"bounded"}`), 0o600),
			).To(Succeed())
			ctrl.templatesDir = dir

			resp := testAPI.Get("/templates")
			Expect(resp.Code).To(Equal(http.StatusOK))
			Expect(resp.Body.String()).To(ContainSubstring("alpha"))
		})
	})
})
