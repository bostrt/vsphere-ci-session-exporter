package vsphere

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_StripDomain_A(t *testing.T) {
	expected := "user"
	p := StripDomain("user@vsphere.local")
	assert.Equal(t, expected, p)
}

func Test_StripDomain_B(t *testing.T) {
	expected := "user"
	p := StripDomain("VSPHERE.LOCAL\\user")
	assert.Equal(t, expected, p)
}
