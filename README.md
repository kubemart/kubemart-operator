CR manifest should look like:

```yaml
apiVersion: kubemart.civo.com/v1alpha1
kind: App
metadata:
  name: kubenav
  namespace: default
spec:
  name: kubenav
  action: install | update
# status:
#   installed_version: v1.0.0
#   jobs_executed:
#     kubenav-job-h4vmf:
#       job_status: installation_started
#       started_at: "2021-01-29T03:44:46Z"
#   last_job_executed: kubenav-job-h4vmf
#   last_status: installation_started
#   configurations:
#     key: MYSQL_PASSWORD
#     value: aGVsbG8=
#     value_is_base64: true
```
