# webcal-sync Helm Chart

This Helm chart deploys webcal_sync as a CronJob to synchronize iCal feeds to Google Calendar.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- A Google Cloud Platform project with Calendar API enabled
- OAuth2 credentials (`credentials.json`)

## Installation

### 1. Prepare Configuration Files

Before installing, you need to prepare three configuration files:

- `config.yml` - Your webcal sync configuration
- `credentials.json` - Google OAuth2 client credentials
- `token.json` - OAuth2 token (generated during setup)

Example `config.yml`:
```yaml
- url: "https://example.com/calendar.ics"
  color_id: "1"
  id_format: "url"
  reminder: 30
```

### 2. Create PersistentVolume (Optional)

**Most clusters with dynamic provisioning (AWS, GCP, Azure) don't need this step** - the PersistentVolumeClaim will automatically create storage.

If your cluster doesn't support dynamic provisioning (e.g., local/on-premises), create a PersistentVolume first. See example PV files:

```bash
# For local testing (minikube, kind)
kubectl apply -f examples/pv-local.yaml

# OR for NFS storage
kubectl apply -f examples/pv-nfs.yaml
```

Check `examples/README.md` for more details and cloud provider storage class options.

### 3. Create ConfigMap with Initial Files

Create a ConfigMap with your `config.yml` and `credentials.json`:

```bash
kubectl create configmap webcal-sync-config \
  --from-file=config.yml=./config.yml \
  --from-file=credentials.json=./credentials.json
```

### 4. Install the Chart

```bash
helm install webcal-sync ./helm/webcal-sync
```

Or with custom configuration:

```bash
helm install webcal-sync ./helm/webcal-sync \
  --set schedule="0 */2 * * *" \
  --set image.tag=v1.0.0
```

Or create a custom `values.yaml`:

```yaml
image:
  repository: akarnani/webcal_sync
  tag: latest

schedule: "0 */2 * * *"  # Run every 2 hours

persistence:
  enabled: true
  size: 1Gi
```

Then install:

```bash
helm install webcal-sync ./helm/webcal-sync -f values.yaml
```

### 5. Run OAuth Setup (First Time Only)

Before the CronJob can run, you need to complete the OAuth flow to generate `token.json`:

```bash
# Enable the OAuth setup job
helm upgrade webcal-sync ./helm/webcal-sync \
  --set oauthSetup.enabled=true \
  --reuse-values

# Wait for the pod to be created
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/component=oauth-setup --timeout=60s

# Get the pod name
POD_NAME=$(kubectl get pods -l app.kubernetes.io/component=oauth-setup -o jsonpath='{.items[0].metadata.name}')

# Attach to the pod to see the OAuth URL and enter the code
kubectl attach -it $POD_NAME

# Follow the instructions:
# 1. Open the URL in your browser
# 2. Authorize the application
# 3. Copy the authorization code and paste it into the terminal
```

After successful authentication, `token.json` will be saved to the persistent volume.

Disable the OAuth setup job:

```bash
helm upgrade webcal-sync ./helm/webcal-sync \
  --set oauthSetup.enabled=false \
  --reuse-values
```

### 6. Manual Copy of Config Files (Alternative)

If you prefer to manually copy files to the PersistentVolume:

```bash
# Create a temporary pod to access the PVC
kubectl run -it --rm copy-files --image=alpine --restart=Never \
  --overrides='{"spec":{"containers":[{"name":"copy-files","image":"alpine","stdin":true,"tty":true,"volumeMounts":[{"name":"config","mountPath":"/config"}]}],"volumes":[{"name":"config","persistentVolumeClaim":{"claimName":"webcal-sync"}}]}}'

# Inside the pod, you can now copy files
# (Use kubectl cp from another terminal)
```

From another terminal:
```bash
kubectl cp config.yml copy-files:/config/config.yml
kubectl cp credentials.json copy-files:/config/credentials.json
```

## Configuration

### Key Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Docker image repository | `"akarnani/webcal_sync"` |
| `image.tag` | Image tag | `"latest"` |
| `schedule` | CronJob schedule | `"0 * * * *"` (hourly) |
| `healthcheckUrl` | Healthcheck.io URL | `""` |
| `persistence.enabled` | Enable persistent volume | `true` |
| `persistence.existingClaim` | Use existing PVC | `""` |
| `persistence.size` | Size of PVC | `1Gi` |
| `persistence.storageClass` | Storage class | `""` |
| `oauthSetup.enabled` | Enable OAuth setup job | `false` |
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `256Mi` |

See `values.yaml` for all available options.

## Upgrading

```bash
helm upgrade webcal-sync ./helm/webcal-sync --reuse-values
```

## Uninstalling

```bash
helm uninstall webcal-sync
```

Note: The PersistentVolumeClaim is not deleted automatically to prevent data loss. Delete manually if needed:

```bash
kubectl delete pvc webcal-sync
```

## Troubleshooting

### Check CronJob Status

```bash
kubectl get cronjobs
kubectl get jobs
```

### View Logs

```bash
# Get the latest job
JOB_NAME=$(kubectl get jobs --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1].metadata.name}')
POD_NAME=$(kubectl get pods --selector=job-name=$JOB_NAME -o jsonpath='{.items[0].metadata.name}')

# View logs
kubectl logs $POD_NAME
```

### Manually Trigger a Sync

```bash
kubectl create job --from=cronjob/webcal-sync manual-sync-$(date +%s)
```

### Verify Config Files

```bash
kubectl exec -it $(kubectl get pods -l app.kubernetes.io/name=webcal-sync --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1].metadata.name}') -- ls -la /app/config
```
