# Helm Chart Examples

This directory contains example configuration files for different deployment scenarios.

## PersistentVolume Examples

Most cloud providers (AWS, GCP, Azure) support dynamic volume provisioning, so you don't need to manually create a PersistentVolume. The PVC in the Helm chart will automatically create the underlying storage.

### When You Need a PersistentVolume

Create a PersistentVolume manually if:
- Your cluster doesn't support dynamic provisioning
- You're using local/on-premises storage
- You want to use a specific storage solution (NFS, local disk, etc.)

### Examples

**pv-local.yaml** - For local testing with minikube, kind, or similar
```bash
kubectl apply -f examples/pv-local.yaml
```

**pv-nfs.yaml** - For NFS-based storage
```bash
# Edit the file to set your NFS server details
kubectl apply -f examples/pv-nfs.yaml
```

## Cloud Provider Storage Classes

If you're on a cloud provider, you typically just specify the storage class:

### AWS EKS
```bash
helm install webcal-sync ./helm/webcal-sync \
  --set persistence.storageClass=gp3
```

### GCP GKE
```bash
helm install webcal-sync ./helm/webcal-sync \
  --set persistence.storageClass=standard-rwo
```

### Azure AKS
```bash
helm install webcal-sync ./helm/webcal-sync \
  --set persistence.storageClass=managed-premium
```

### Check Available Storage Classes
```bash
kubectl get storageclass
```

If you see a storage class marked as `(default)`, you don't need to specify anything - the chart will use it automatically.
