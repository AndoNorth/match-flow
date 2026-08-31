package grpcapi_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGRPCAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "gRPC API Suite")
}
