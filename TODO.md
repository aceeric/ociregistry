# In progress

# On deck

- cmd/subcmd/serve.go: `swagger, err := api.GetSwagger()` deprecated
- Image in ghcr.io?
- Helm chart: switch the deployment strategy to Recreate, or maxSurge: 0, maxUnavailable: 1, so
  two pods never run on the same cache during an upgrade or if evicted as part of cluster auto
  scaling. Look st terminationGracePeriodSeconds.Ensure PV uses Retain reclaim policy. Does chart
  support region affinity for EBS?? Document lack of availability on Recreate.
- Refactor tests to use `t.TempDir()` and `t.Helper()`

# Parked

# Backlog

- Document every command option with an example
- Config parse
- Server base url (for ingress)
- Consider https://github.com/OpenAPITools/openapi-generator
- Enable swagger UI (https://github.com/go-swagger/go-swagger)?
