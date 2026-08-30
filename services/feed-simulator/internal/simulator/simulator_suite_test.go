package simulator_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSimulator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Simulator Suite")
}
