package build

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"io"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"regexp"
)

var (
	UsingNamespaceRegex = regexp.MustCompile(`.*Using namespace .*/(ci-op-........)`)
)

type Metadata struct {
	VSphere struct {
		VCenter string `json:"vCenter"`
		Username string `json:"username"`
	} `json:"vsphere"`
}

func BuildClient(kubeconfig string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)

	return clientset, err
}

func GetCIUserForBuildID(buildID string, target string, clientset *kubernetes.Clientset) (string, error) {
	labelSelector := fmt.Sprintf("prow.k8s.io/build-id=%s", buildID)

	log.Debugf("looking for pods in ci namespace with label selector: %s", labelSelector)
	podList, err := clientset.CoreV1().Pods("ci").List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return "", err
	}

	if len(podList.Items) != 1 {
		return "", errors.Wrap(err, "found multiple pods with same build-id annotation")
	}

	log.Debugf("found %d pod[s] for build id %s", len(podList.Items), buildID)

	jobPod := podList.Items[0]
	ns, err := getCiNamespaceFromPod(clientset, jobPod)
	if err != nil {
		return "", err
	}

	return getCIUserFromSecret(clientset, target, ns)
}

func getCiNamespaceFromPod(clientset *kubernetes.Clientset, jobPod corev1.Pod) (string, error) {
	req := clientset.CoreV1().Pods(jobPod.Namespace).GetLogs(jobPod.Name, &corev1.PodLogOptions{
		Container:                    "test",
	})

	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return "", err
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", err
	}

	str := buf.String()
	return getCiNamespaceFromPodLogs(str)
}

func getCiNamespaceFromPodLogs(logs string) (string, error) {
	matches := UsingNamespaceRegex.FindStringSubmatch(logs)
	if matches == nil {
		return "", fmt.Errorf("unable to find any matching ci-op-* namespace in logs")
	}

	return matches[1], nil
}
func getCIUserFromSecret(clientset *kubernetes.Clientset, secretName string, namespace string) (string, error) {
	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	for key,value := range secret.Data {
		if key == "metadata.json" {
			m := Metadata{}
			err = json.Unmarshal(value, &m)
			if err != nil {
				return "", errors.Wrap(err, "error unmarshalling metadata.json")
			}
			if m.VSphere.VCenter == "ibmvcenter.vmc-ci.devcluster.openshift.com" { // TODO Probably filter this somewhere more obvious
				return m.VSphere.Username, nil
			}
		}
	}
	log.Debugf("unable to find CI user from secret %s/%s", namespace, secretName)
	return "", nil
}
