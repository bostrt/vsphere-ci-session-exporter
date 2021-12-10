package prow

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

var (
	GoodArgs = []string{
		"--gcs-upload-secret=/secrets/gcs/service-account.json",
		"--image-import-pull-secret=/etc/pull-secret/.dockerconfigjson",
		"--lease-server-credentials-file=/etc/boskos/credentials",
		"--report-credentials-file=/etc/report/credentials",
		"--secret-dir=/secrets/ci-pull-credentials",
		"--secret-dir=/usr/local/e2e-vsphere-cluster-profile",
		"--target=e2e-vsphere",
		"--variant=nightly-4.7",
	}

	BadArgs = []string{
		"--verbose=true",
		"--output=log",
	}
)

func Test_getTargetFromProwJobArgs_Good(t *testing.T) {
	target := getTargetFromProwJobArgs(GoodArgs)
	assert.Equal(t, "e2e-vsphere", target)
}

func Test_getTargetFromProwJobArgs_Bad(t *testing.T) {
	target := getTargetFromProwJobArgs(BadArgs)
	assert.Equal(t, "", target)
}
