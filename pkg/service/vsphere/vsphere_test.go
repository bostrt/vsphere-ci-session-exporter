package vsphere

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_getUsernamePermutations_A(t *testing.T) {
	expected := []string{
		"VSPHERE.LOCAL\\ci_user2",
		"ci_user2@vsphere.local",
	}
	p := GetUsernamePermutations("ci_user2@vsphere.local")
	assert.NotNil(t, p)
	assert.EqualValues(t, expected, p)
}

func Test_getUsernamePermutations_B(t *testing.T) {
	expected := []string{
		"VSPHERE.LOCAL\\ci_user3",
		"ci_user3@vsphere.local",
	}
	p := GetUsernamePermutations("VSPHERE.LOCAL\\ci_user3")
	assert.NotNil(t, p)
	assert.EqualValues(t, expected, p)
}
