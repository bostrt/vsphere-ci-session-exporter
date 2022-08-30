package exporter

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/soap"
	"k8s.io/client-go/kubernetes"
	prowclient "k8s.io/test-infra/prow/client/clientset/versioned"

	"github.com/bostrt/vsphere-ci-session-metrics/pkg/service/build"
	"github.com/bostrt/vsphere-ci-session-metrics/pkg/service/prow"
	"github.com/bostrt/vsphere-ci-session-metrics/pkg/service/vsphere"
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
	vcenter          string
	prowURI          string
	mutex            sync.RWMutex
	warningThreshold float64

	vsphereHost      string
	vsphereUser      string
	vspherePasswd    string
	vsphereUserAgent string
	buildClientset   *kubernetes.Clientset
	prowClientset    *prowclient.Clientset

	// Metrics of exporter itself
	// TODO Include Prow and vCenter names in these metrics!
	totalScrapes prometheus.Counter
	vcenterUp    prometheus.Gauge
	prowUp       prometheus.Gauge
}

func (e *Exporter) Shutdown() {
	log.Info("shutting down exporter...")
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
	start := time.Now()
	vcenterUp, prowUp := e.scrape(ch)

	e.vcenterUp.Set(vcenterUp)
	e.prowUp.Set(prowUp)

	ch <- e.vcenterUp
	ch <- e.prowUp
	ch <- e.totalScrapes

	duration := time.Since(start)
	if duration.Seconds() > e.warningThreshold {
		log.Warnf("scrape operation took too long: %.2fs", duration.Seconds())
	}

	log.Debug("Metric collection complete.")
}

func (e *Exporter) vSphereLogin() (*govmomi.Client, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), 60*time.Second)
	defer cancel()

	u, err := soap.ParseURL(fmt.Sprintf("https://%s", e.vsphereHost))
	if err != nil {
		return nil, err
	}

	// Get vSphere User Sessions
	u.User = nil
	c, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
		return nil, err
	}

	c.UserAgent = e.vsphereUserAgent
	err = c.Login(ctx, url.UserPassword(e.vsphereUser, e.vspherePasswd))
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) (vcenterUp float64, prowUp float64) {
	ctx, cancel := context.WithTimeout(context.TODO(), 60*time.Second)
	defer cancel()

	e.totalScrapes.Inc()

	c, err := e.vSphereLogin()
	if err != nil {
		log.Error(err)
		return
	}
	defer c.Logout(ctx)

	v, err := vsphere.GetVsphereData(c)
	if err != nil {
		log.Error(errors.Wrap(err, "failed scraping vsphere"))
		return
	}

	// Get Prow Jobs on vSphere
	var prowDataProvider prow.DataProvider
	if e.prowClientset == nil {
		// Pull data anonymously. This doesn't utilize server-side job filtering.
		prowDataProvider = prow.AnonymousDataProvider{}
	} else {
		// Call to K8s API for Prow Jobs
		prowDataProvider, err = prow.NewAuthenticatedDataProvier(e.prowClientset)
		if err != nil {
			log.Error(err)
			return 1, 0
		}
	}

	prowData, err := prowDataProvider.GetData()
	if err != nil {
		log.Error(errors.Wrap(err, "failed to get prow jobs"))
	}

	// Bring together data from Prow and vSphere.
	// Loop over each vSphere Prow Job and find the CI User assoicated
	// with it by querying the Build cluster.
	for _, job := range prowData {
		buildId := job.GetLabels()["prow.k8s.io/build-id"]
		jobName := job.GetAnnotations()["prow.k8s.io/job"]
		pullLink := prow.GetPRLinkFromJob(job)
		target, err := prow.GetTargetFromProwJob(job)
		if err != nil {
			log.Debug(err)
			continue
		}

		log.Debugf("build-id: %s job: %s PR: %s", buildId, jobName, pullLink)

		// Get CI username from metadata.json for the job
		user, err := build.GetCIUserForBuildID(buildId, target, e.buildClientset)
		if err != nil {
			log.Debug(err)
			continue
		}

		// We're assuming @vsphere.local, strip it away
		user = vsphere.StripDomain(user)
		if user == "" {
			log.Tracef("cannot strip domain from user")
			continue
		}

		// Get map[string]float64 which contains user agent count summary
		userAgents := v.GetUserAgentsForUser(user)
		if userAgents == nil {
			log.Debugf("no sessions for user: %s", user)
			continue
		}

		for userAgent, count := range userAgents {
			ch <- prometheus.MustNewConstMetric(correlatedMetricDesc,
				correlatedMetricType,
				count,
				user,
				userAgent,
				jobName,
				buildId,
				pullLink,
				"ibmvcenter.vmc-ci.devcluster.openshift.com")
		}
	}

	return 1, 1
}

func NewExporter(warning float64, buildKubeconfig, prowKubeconfig, vsphereHost, vsphereUser, vspherePasswd, vsphereUserAgent, prowURI string) (*Exporter, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), 60*time.Second)
	defer cancel()

	u, err := soap.ParseURL(fmt.Sprintf("https://%s", vsphereHost))
	if err != nil {
		return nil, err
	}

	u.User = nil
	c, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
		return nil, err
	}

	// Test login with vSphere
	c.UserAgent = vsphereUserAgent
	err = c.Login(ctx, url.UserPassword(vsphereUser, vspherePasswd))
	if err != nil {
		return nil, err
	}
	defer c.Logout(ctx)

	buildClientset, err := build.BuildClient(buildKubeconfig)
	if err != nil {
		return nil, err
	}

	var prowClientset *prowclient.Clientset
	if prowKubeconfig != "" {
		prowClientset, err = prow.BuildClient(prowKubeconfig)
		if err != nil {
			return nil, err
		}
	}

	return &Exporter{
		prowURI:          prowURI,
		vcenter:          vsphereHost,
		vsphereHost:      vsphereHost,
		vsphereUser:      vsphereUser,
		vspherePasswd:    vspherePasswd,
		vsphereUserAgent: vsphereUserAgent,
		buildClientset:   buildClientset,
		prowClientset:    prowClientset,
		warningThreshold: warning,
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_scrapes_total",
			Help:      "Current total scrapes",
		}),
		vcenterUp: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "vcenter_up",
			Help:      "Was vCenter up last scrape.",
		}),
		prowUp: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "prow_up",
			Help:      "Was Prow up last scrape.",
		}),
	}, nil
}
