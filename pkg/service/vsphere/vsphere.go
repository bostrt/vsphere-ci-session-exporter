package vsphere

import (
	"context"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"regexp"
)

var (
	usernameRegex = regexp.MustCompile(`^((.+)@vsphere.local)$|^(VSPHERE.LOCAL\\(.+))$`)
)

type VSphereUsers struct {
	Mappings map[string]map[string]float64 // username => { user agent => count }
}

func (v *VSphereUsers) ForEach(f func(username string, userAgents map[string]float64)) {
	for username, userAgentMap := range v.Mappings {
		f(username, userAgentMap)
	}
}

func (v *VSphereUsers) addMapping(username string, userAgent string) {
	username = StripDomain(username)
	userAgentMap, ok := v.Mappings[username]

	if ! ok {
		// Add new entry to map
		userAgentMap = map[string]float64{}
		v.Mappings[username] = userAgentMap
		log.Debugf("added mapping for user %s (%s)", username, userAgent)
	}

	userAgentMap[userAgent] = userAgentMap[userAgent] + 1 // Increment counter
}

func (v *VSphereUsers) GetUserAgentsForUser(username string) map[string]float64 {
	log.Debugf("checking sessions for user %s", username)
	userAgents, ok := v.Mappings[username]
	if ! ok {
		return nil
	}
	return userAgents
}

func GetVsphereData(vmClient *govmomi.Client) (*VSphereUsers, error) {
	v := &VSphereUsers{
		Mappings: map[string]map[string]float64{},
	}

	m, err := getSessionManager(vmClient, context.TODO())
	if err != nil {
		return nil, errors.Wrap(err, "error getting session manager")
	}

	log.Debugf("Found %d user sessions", len(m.SessionList))
	for _,s := range m.SessionList {
		v.addMapping(s.UserName, s.UserAgent)
	}

	return v, nil
}

// From https://github.com/vmware/govmomi/blob/master/govc/session/ls.go
func getSessionManager(vmClient *govmomi.Client, ctx context.Context) (*mo.SessionManager, error) {
	var m mo.SessionManager
	var props []string
	c := vmClient.Client
	pc := property.DefaultCollector(c)

	err := pc.RetrieveOne(ctx, *c.ServiceContent.SessionManager, props, &m)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

// StripDomain takes a vSphere username (e.g. user@vsphere.local or VSPHERE.LOCAL/admin)
// and removes the domain portion, returning only the username.
func StripDomain(username string) string {
	log.Tracef("stripping domain from user %s", username)
	matches := usernameRegex.FindStringSubmatch(username)
	if matches == nil {
		return ""
	}

	var name string

	if matches[2] != "" {
		// something@vsphere.local
		name = matches[2]
	}

	if matches[4] != "" {
		// VSPHERE.LOCAL\something
		name = matches[4]
	}

	return name
}
