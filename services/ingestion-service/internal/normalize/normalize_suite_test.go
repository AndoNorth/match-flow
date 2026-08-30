package normalize_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestNormalize(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Normalize Suite")
}
