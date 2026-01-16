# webcal_sync Helm Repository

This branch contains the packaged Helm charts for webcal_sync.

## Usage

Add the Helm repository:

```bash
helm repo add webcal-sync https://akarnani.github.io/webcal_sync
helm repo update
```

Install the chart:

```bash
helm install my-webcal-sync webcal-sync/webcal-sync
```

## Available Charts

See the [charts/](charts/) directory for available chart versions.
