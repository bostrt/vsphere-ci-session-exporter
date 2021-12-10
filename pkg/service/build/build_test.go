package build

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

const (
	GoodLog = `INFO[2021-12-10T14:28:50Z] Resolved SHA missing for master in https://github.com/openshift/release (will prevent caching) 
INFO[2021-12-10T14:28:50Z] Using explicitly provided pull-spec for release latest (registry.ci.openshift.org/origin/release:4.8.0-0.okd-2021-12-10-122104) 
INFO[2021-12-10T14:28:50Z] Resolved release latest to registry.ci.openshift.org/origin/release:4.8.0-0.okd-2021-12-10-122104 
INFO[2021-12-10T14:28:50Z] Using namespace https://console-openshift-console.apps.build01-us-west-2.vmc.ci.openshift.org/k8s/cluster/projects/ci-op-9nmljnxm 
INFO[2021-12-10T14:28:50Z] Running [input:upi-installer], [input:origin-centos-8], [release:latest], [images], e2e-vsphere 
INFO[2021-12-10T14:28:52Z] Tagging origin/centos:8 into pipeline:origin-centos-8. 
INFO[2021-12-10T14:28:52Z] Tagging origin/4.8:upi-installer into pipeline:upi-installer. 
INFO[2021-12-10T14:28:52Z] Importing release image latest.`

	BadLog = `INFO[2021-12-10T14:28:50Z] Resolved SHA missing for master in https://github.com/openshift/release (will prevent caching) 
INFO[2021-12-10T14:28:50Z] Using explicitly provided pull-spec for release latest (registry.ci.openshift.org/origin/release:4.8.0-0.okd-2021-12-10-122104) 
INFO[2021-12-10T14:28:50Z] Resolved release latest to registry.ci.openshift.org/origin/release:4.8.0-0.okd-2021-12-10-122104`
)

func Test_getCiNamespaceFromPodLogs_Good(t *testing.T) {
	result, err := getCiNamespaceFromPodLogs(GoodLog)
	assert.Nil(t, err)
	assert.Equal(t, "ci-op-9nmljnxm", result)
}

func Test_getCiNamespaceFromPodLogs_Bad(t *testing.T) {
	result, err := getCiNamespaceFromPodLogs(BadLog)
	assert.NotNil(t, err)
	assert.Zero(t, len(result))
}
