package template_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/template"
)

func writeTemplate(dir, filename, contents string) {
	Expect(os.WriteFile(filepath.Join(dir, filename), []byte(contents), 0o600)).To(Succeed())
}

var _ = Describe("Load", func() {
	It("loads a valid bounded template", func() {
		dir := GinkgoT().TempDir()
		writeTemplate(dir, "goalless.json", `{
			"name": "goalless_draw", "kind": "bounded",
			"home_goals": {"min": 0, "max": 0}, "away_goals": {"min": 0, "max": 0},
			"yellow_cards": {"min": 0, "max": 0}, "red_cards": {"min": 0, "max": 0}
		}`)

		tmpl, err := template.Load(filepath.Join(dir, "goalless.json"))
		Expect(err).NotTo(HaveOccurred())
		Expect(tmpl.Name).To(Equal("goalless_draw"))
		Expect(tmpl.Kind).To(Equal(template.KindBounded))
	})

	It("loads a valid literal template", func() {
		dir := GinkgoT().TempDir()
		writeTemplate(dir, "scripted.json", `{
			"name": "scripted_demo", "kind": "literal",
			"events": [{"type": "goal", "team": "home", "minute": 10}]
		}`)

		tmpl, err := template.Load(filepath.Join(dir, "scripted.json"))
		Expect(err).NotTo(HaveOccurred())
		Expect(tmpl.Events).To(HaveLen(1))
	})

	It("rejects a literal template with no events", func() {
		dir := GinkgoT().TempDir()
		writeTemplate(dir, "empty.json", `{"name": "x", "kind": "literal"}`)

		_, err := template.Load(filepath.Join(dir, "empty.json"))
		Expect(err).To(HaveOccurred())
	})

	It("rejects a bounded template with an inverted range", func() {
		dir := GinkgoT().TempDir()
		writeTemplate(dir, "bad.json", `{
			"name": "x", "kind": "bounded",
			"home_goals": {"min": 5, "max": 1}
		}`)

		_, err := template.Load(filepath.Join(dir, "bad.json"))
		Expect(err).To(HaveOccurred())
	})

	It("rejects a template with an unknown kind", func() {
		dir := GinkgoT().TempDir()
		writeTemplate(dir, "bad.json", `{"name": "x", "kind": "mystery"}`)

		_, err := template.Load(filepath.Join(dir, "bad.json"))
		Expect(err).To(HaveOccurred())
	})

	It("rejects a template with no name", func() {
		dir := GinkgoT().TempDir()
		writeTemplate(dir, "bad.json", `{"kind": "bounded"}`)

		_, err := template.Load(filepath.Join(dir, "bad.json"))
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("LoadAll", func() {
	It("loads every template in a directory, keyed by name", func() {
		dir := GinkgoT().TempDir()
		writeTemplate(dir, "a.json", `{"name": "alpha", "kind": "bounded"}`)
		writeTemplate(dir, "b.json", `{"name": "beta", "kind": "bounded"}`)

		templates, err := template.LoadAll(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(templates).To(HaveKey("alpha"))
		Expect(templates).To(HaveKey("beta"))
	})

	It("returns an empty map for a directory with no templates", func() {
		templates, err := template.LoadAll(GinkgoT().TempDir())
		Expect(err).NotTo(HaveOccurred())
		Expect(templates).To(BeEmpty())
	})
})
