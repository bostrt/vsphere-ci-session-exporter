package exporter

import (
	"context"
	"fmt"
	"github.com/bostrt/vsphere-ci-session-metrics/pkg/service/build"
	"github.com/bostrt/vsphere-ci-session-metrics/pkg/service/prow"
	"github.com/bostrt/vsphere-ci-session-metrics/pkg/service/vsphere"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/soap"
	"k8s.io/client-go/kubernetes"
	prowapiv1 "k8s.io/test-infra/prow/apis/prowjobs/v1"
	"net/url"
	"sync"
	"time"
)

var (
	namespace = "vsphere_ci_user_sessions"

	correlatedMetricDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "correlated"),
		"Correlated data between Prow and vCentre",
		[]string{"username", "user_agent", "ci_job", "build_id", "pull_request", "vcenter"},
		nil)

	correlatedMetricType = prometheus.GaugeValue
)



type Exporter struct {
	vcenter string
	prowURI string
	mutex sync.RWMutex

	vmClient *govmomi.Client
	clientset *kubernetes.Clientset

	// Metrics of exporter itself
	// TODO Include Prow and vCenter names in these metrics!
	totalScrapes prometheus.Counter
	vcenterUp prometheus.Gauge
	prowUp prometheus.Gauge
}

func (e *Exporter) Shutdown() {
	log.Info("shutting down exporter...")
	e.vmClient.Logout(context.TODO())
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.totalScrapes.Desc()
	ch <- e.vcenterUp.Desc()
	ch <- e.prowUp.Desc()
	// ...
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock() // To protect metrics from concurrent collects.
	defer e.mutex.Unlock()

	log.Debug("Metric collection starting...")
	vcenterUp, prowUp := e.scrape(ch)

	e.vcenterUp.Set(vcenterUp)
	e.prowUp.Set(prowUp)

	ch <- e.vcenterUp
	ch <- e.prowUp
	ch <- e.totalScrapes

	log.Debug("Metric collection complete.")
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) (vcenterUp float64, prowUp float64) {
	e.totalScrapes.Inc()

	// Get vSphere User Sessions
	v, err := vsphere.GetVsphereData(e.vmClient)
	if err != nil {
		log.Error(errors.Wrap(err, "failed scraping vsphere"))
		return
	}

	// Get Prow Jobs on vSphere
	jobs, err := prow.GetProwData()
	if err != nil {
		log.Error(errors.Wrap(err, "failed getting prow jobs"))
		return
	}

	jobs.ForEach(func(job prowapiv1.ProwJob, target string) {
		buildId := job.GetLabels()["prow.k8s.io/build-id"]
		jobName := job.GetLabels()["prow.k8s.io/job"]
		pullLink := prow.GetPRLinkFromJob(job)

		user, err := build.GetCIUserForBuildID(buildId, target, e.clientset)
		if err != nil {
			log.Error(err)
			return
		}
		users := vsphere.GetUsernamePermutations(user)
		if users == nil {
			log.Errorf("issue getting user permutations")
			return
		}

		for _,u := range users {
			userAgents := v.GetUserAgentsForUser(u)
			if userAgents == nil {
				log.Errorf("no sessions for user: %s", u)
				continue
			}
			for userAgent, count := range userAgents {
				ch <- prometheus.MustNewConstMetric(correlatedMetricDesc,
					correlatedMetricType,
					count,
					u,
					userAgent,
					jobName,
					buildId,
					pullLink,
					"ibmvcenter.vmc-ci.devcluster.openshift.com")
			}
		}
	})

	return 1, 1
}

func NewExporter(kubeconfig, vsphereHost, vsphereUser, vspherePasswd, vsphereUserAgent, prow string) (*Exporter, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), 60*time.Second)
	defer cancel()

	u, err := soap.ParseURL(fmt.Sprintf("https://%s", vsphereHost))
	if err != nil {
		return nil, err
	}

	u.User = nil
	c, err := govmomi.NewClient(ctx, u, false)
	if err != nil {
		return nil, err
	}

	c.UserAgent = vsphereUserAgent
	err = c.Login(ctx, url.UserPassword(vsphereUser, vspherePasswd))
	if err != nil {
		return nil, err
	}

	clientset, err := build.BuildClient(kubeconfig)
	if err != nil {
		return nil, err
	}

	return &Exporter{
		prowURI:  prow,
		vcenter:  vsphereHost,
		vmClient: c,
		clientset: clientset,
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name: "exporter_scrapes_total",
			Help: "Current total scrapes",
		}),
		vcenterUp: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name: "vcenter_up",
			Help: "Was vCenter up last scrape.",
		}),
		prowUp: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name: "prow_up",
			Help: "Was Prow up last scrape.",
		}),
	}, nil
}
