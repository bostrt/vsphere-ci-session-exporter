## Useful Queries

### Count by CI Username
`sum by(username) (vsphere_ci_user_sessions_correlated)`

### Count by CI Job
`sum by(ci_job) (vsphere_ci_user_sessions_correlated)`
