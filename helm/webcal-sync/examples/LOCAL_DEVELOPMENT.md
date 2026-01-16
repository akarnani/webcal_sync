# Local Development with Rancher Desktop

This guide shows how to run webcal-sync on Rancher Desktop (or Docker Desktop) using a simple hostPath volume instead of PersistentVolumeClaims.

## Prerequisites

- Rancher Desktop or Docker Desktop with Kubernetes enabled
- Your config files ready: `config.yml`, `credentials.json`

## Setup

### 1. Create a directory on your Mac for config files

```bash
mkdir -p ~/webcal-sync-config
```

### 2. Copy your config files to this directory

```bash
cp config.yml ~/webcal-sync-config/
cp credentials.json ~/webcal-sync-config/
```

### 3. Edit the values file

Edit `examples/values-local-dev.yaml` and update the `hostPath.path` to match your directory:

```yaml
hostPath:
  enabled: true
  path: /Users/yourname/webcal-sync-config  # Update this!
```

Or specify it at install time:

```bash
helm install webcal-sync ./helm/webcal-sync \
  -f examples/values-local-dev.yaml \
  --set hostPath.path=/Users/andrew/webcal-sync-config
```

### 4. Install the chart

```bash
helm install webcal-sync ./helm/webcal-sync \
  -f examples/values-local-dev.yaml \
  --set hostPath.path=$HOME/webcal-sync-config
```

### 5. Run OAuth Setup (First Time Only)

Enable the OAuth setup job:

```bash
helm upgrade webcal-sync ./helm/webcal-sync \
  -f examples/values-local-dev.yaml \
  --set hostPath.path=$HOME/webcal-sync-config \
  --set oauthSetup.enabled=true
```

Wait for the pod to be ready, then attach:

```bash
# Wait for pod
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/component=oauth-setup --timeout=60s

# Get pod name
POD_NAME=$(kubectl get pods -l app.kubernetes.io/component=oauth-setup -o jsonpath='{.items[0].metadata.name}')

# Attach to the pod
kubectl attach -it $POD_NAME
```

Follow the OAuth flow:
1. Copy the URL and open it in your browser
2. Authorize the application
3. Copy the authorization code
4. Paste it into the terminal

The `token.json` file will be saved to `~/webcal-sync-config/token.json` on your Mac.

### 6. Disable OAuth Setup

After successful authentication:

```bash
helm upgrade webcal-sync ./helm/webcal-sync \
  -f examples/values-local-dev.yaml \
  --set hostPath.path=$HOME/webcal-sync-config \
  --set oauthSetup.enabled=false
```

### 7. Verify Files

Check that the token was created:

```bash
ls -la ~/webcal-sync-config/
# Should show: config.yml, credentials.json, token.json
```

## Testing

### Manually Trigger a Sync

```bash
kubectl create job --from=cronjob/webcal-sync manual-sync-$(date +%s)
```

### View Logs

```bash
# Get the latest job
JOB_NAME=$(kubectl get jobs --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1].metadata.name}')
POD_NAME=$(kubectl get pods --selector=job-name=$JOB_NAME -o jsonpath='{.items[0].metadata.name}')

# View logs
kubectl logs $POD_NAME -f
```

## Advantages of hostPath for Local Development

- **No PVC needed**: Simpler setup, no storage classes to configure
- **Direct file access**: You can edit config files directly on your Mac
- **Easy debugging**: View and modify `token.json` directly
- **Fast iterations**: Changes to config files are immediately available

## Limitations

- **Not for production**: hostPath volumes are only for single-node clusters
- **No redundancy**: Data is only on your local machine
- **Node-specific**: Only works on the node where the path exists

For production deployments, use the PVC-based approach with proper storage classes.
