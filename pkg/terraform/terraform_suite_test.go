package terraform

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestTerraform(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Terraform Suite")
}
