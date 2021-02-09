CR manifest should look like:

```yaml
apiVersion: app.bizaar.civo.com/v1alpha1
kind: App
metadata:
  name: kubenav
  namespace: default
spec:
  name: kubenav
  target_status: ""
# status:
#   installed_version: v1.0.0
#   jobs_executed:
#     kubenav-job-h4vmf:
#       job_status: installation_started
#       started_at: "2021-01-29T03:44:46Z"
#   last_job_executed: kubenav-job-h4vmf
#   last_status: installation_started
```
