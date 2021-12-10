package prow

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	prowapiv1 "k8s.io/test-infra/prow/apis/prowjobs/v1"
	"net/http"
	"regexp"
)

const (
	VSphereClusterAlias = "vsphere"
)

var (
	TargetRegex = regexp.MustCompile(`^--target=(.*)$`)
)


type VSphereProwJobs struct {
	allJobs *[]prowapiv1.ProwJob
	targets []string
}

func (v *VSphereProwJobs) ForEach(f func(job prowapiv1.ProwJob, target string)) {
	for i,job := range *v.allJobs {
		f(job, v.targets[i])
	}
}

func getVSphereProwJobs() (*[]prowapiv1.ProwJob, error) {
	// https://prow.ci.openshift.org/prowjobs.js?omit=annotations,decoration_config,pod_spec
	resp, err := http.Get("https://prow.ci.openshift.org/prowjobs.js?omit=decoration_config")
	if err != nil {
		return nil, errors.Wrap(err, "error retrieving list of prow jobs")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status from prow (%d)", resp.StatusCode)
	}

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "error while reading prow response body")
	}

	var prowJobList prowapiv1.ProwJobList
	err = json.Unmarshal(bytes, &prowJobList)
	if err != nil {
		return nil, errors.Wrap(err, "error while parsing prow JSON response")
	}

	var vsphereProwJobs []prowapiv1.ProwJob
	for _,job := range prowJobList.Items {
		if job.ClusterAlias() == VSphereClusterAlias {
			if job.Status.State == prowapiv1.PendingState{
				// Only keep Pending vSphere Jobs
				vsphereProwJobs = append(vsphereProwJobs, job)
			}
		}
	}

	return &vsphereProwJobs, nil
}

func getTargetFromProwJob(job prowapiv1.ProwJob) (string, error) {
	target := getTargetFromProwJobArgs(job.Spec.PodSpec.Containers[0].Args)
	if target == "" {
		return "", fmt.Errorf("unable to find --target arg in prow job")
	}

	return target, nil
}

func getTargetFromProwJobArgs(args []string) string {
	for _,s := range args {
		matches := TargetRegex.FindStringSubmatch(s)
		if matches == nil {
			continue
		}
		return matches[1]
	}
	return ""
}

func GetProwData() (*VSphereProwJobs, error) {
	allVSphereJobs, err := getVSphereProwJobs()
	if err != nil {
		return nil, err
	}

	targets := make([]string, len(*allVSphereJobs))
	for i, j := range *allVSphereJobs {
		t, err := getTargetFromProwJob(j)
		if err != nil {
			log.Error(err)
			continue
		}
		targets[i] = t
	}

	jobs := &VSphereProwJobs{
		allJobs: allVSphereJobs,
		targets: targets,
	}

	return jobs, nil
}

func GetPRLinkFromJob(job prowapiv1.ProwJob) string {
	// TODO I hope getting the first PR link in list is okay
	if job.Spec.Refs == nil || job.Spec.Refs.Pulls == nil {
		return ""
	}
	
	for _,pull := range job.Spec.Refs.Pulls {
		if pull.Link != "" {
			return pull.Link
		}
	}

	return ""
}
