package cors_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCORS(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CORS Suite")
}
