# vSphere + OpenShift CI Server Metrics

This application exports Prometheus metrics that bring together vSphere Sessions and related OpenShift CI jobs. This can
be used for troubleshooting session leaks or session limit issues.

# TODO

- Error handle loss of vSphere session 
- Error handle loss of k8s/ocp auth
  - Use service account

# Usage

```shell
Usage:
  vsphere-ci-session-metrics start [flags]

Flags:
  -h, --help                        help for start
      --kubeconfig string           path to build cluster kubeconfig
      --listen-port int             exporter will listen on this port (default 8090)
      --log-level string            set log level (e.g. debug, warn, error) (default "info")
      --prow string                 URL for Prow CI instance (default "prow.ci.openshift.org")
      --vsphere string              vSphere hostname (do not include scheme)
      --vsphere-passwd string       password for vSphere
      --vsphere-user string         username for vSphere
      --vsphere-user-agent string   user agent to vSphere communication (default "vsphere-ci-session-metrics")
```

The following flags are **REQUIRED**:

- `--kubeconfig`
- `--vsphere`
- `--vsphere-passwd`
- `--vsphere-user`

The rest are entirely optional and have default values.

## Environment Variables

If you'd rather use environment variables instead of CLI flags:

- `KUBECONFIG`
- `LISTEN_PORT`
- `LOG_LEVEL`
- `PROW`
- `VSPHERE_PASSWD`
- `VSPHERE_USER`
- `VSPHERE_USER_AGENT`

# Run Locally

Here's an example command:

```shell
./vsphere-ci-session-metrics \
   --kubeconfig mykc \
   --vsphere vc.example.com \
   --vsphere-user administrator@vsphere.local \
   --vsphere-passwd tops3cret

INFO[0000] Launching on :8090...                        
```

# Run in k8s

TODO


## Useful Queries

### Count by CI Username
`sum by(username) (vsphere_ci_user_sessions_correlated)`

### Count by CI Job
`sum by(ci_job) (vsphere_ci_user_sessions_correlated)`
