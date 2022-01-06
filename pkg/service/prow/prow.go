package prow

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	prowapiv1 "k8s.io/test-infra/prow/apis/prowjobs/v1"
	prowclient "k8s.io/test-infra/prow/client/clientset/versioned"
	"net/http"
	"regexp"
)

const (
	VSphereClusterAlias = "vsphere"
)

var (
	TargetRegex = regexp.MustCompile(`^--target=(.*)$`)
)

type DataProvider interface {
	GetData() ([]prowapiv1.ProwJob, error)
}

type AnonymousDataProvider struct {
}

func (a AnonymousDataProvider) GetData() ([]prowapiv1.ProwJob, error) {
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

	log.Debugf("Found %d relevant Prow jobs", len(vsphereProwJobs))
	return vsphereProwJobs, nil
}


type AuthenticatedDataProvider struct {
	clientset *prowclient.Clientset
}

func NewAuthenticatedDataProvier(client *prowclient.Clientset) (*AuthenticatedDataProvider, error) {
	return &AuthenticatedDataProvider{
		clientset: client,
	}, nil
}

func (b *AuthenticatedDataProvider) GetData() ([]prowapiv1.ProwJob, error) {
	// Get list of vSphere ProwJobs
	log.Trace("Getting data from k8s")
	jobList, err := b.clientset.ProwV1().ProwJobs("ci").List(context.TODO(), metav1.ListOptions{
		LabelSelector: "ci-operator.openshift.io/cloud=vsphere",
	})
	if err != nil {
		return nil, err
	}

	var vsphereProwJobs []prowapiv1.ProwJob
	for _,job := range jobList.Items {
		if job.Status.State == prowapiv1.PendingState{
			// Only keep Pending vSphere Jobs
			vsphereProwJobs = append(vsphereProwJobs, job)
		}
	}

	log.Debugf("Found %d relevant Prow jobs", len(vsphereProwJobs))
	return vsphereProwJobs, nil
}

func BuildClient(kubeconfig string) (*prowclient.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := prowclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}

func GetTargetFromProwJob(job prowapiv1.ProwJob) (string, error) {
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
