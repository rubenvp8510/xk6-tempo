# OpenShift Deployment Guide for xk6-tempo

This guide explains how to deploy and run xk6-tempo performance tests on an OpenShift cluster.

## Prerequisites

- OpenShift cluster access with appropriate permissions
- `oc` CLI tool installed
- Access to the OpenShift internal image registry (or external registry)
- Tempo deployment accessible from the test namespace

## Quick Start

### 1. Build and Push Container Image

#### Option A: Using OpenShift BuildConfig (Recommended)

```bash
# Login to OpenShift
oc login <your-openshift-cluster>

# Create a new project (if needed)
oc new-project tempo-performance-tests

# Create BuildConfig from Dockerfile
oc new-build --strategy=docker --name=xk6-tempo --binary=true

# Start build from current directory
oc start-build xk6-tempo --from-dir=. --follow
```

#### Option B: Build Locally and Push

```bash
# Build the image
docker build -t xk6-tempo:latest .

# Tag for OpenShift registry
docker tag xk6-tempo:latest \
  image-registry.openshift-image-registry.svc:5000/tempo-performance-tests/xk6-tempo:latest

# Login to OpenShift registry
docker login -u $(oc whoami) -p $(oc whoami -t) \
  image-registry.openshift-image-registry.svc:5000

# Push image
docker push image-registry.openshift-image-registry.svc:5000/tempo-performance-tests/xk6-tempo:latest
```

#### Option C: Using Podman (Recommended for OpenShift)

```bash
# Build the image
podman build -t xk6-tempo:latest .

# Tag for OpenShift registry
podman tag xk6-tempo:latest \
  image-registry.openshift-image-registry.svc:5000/tempo-performance-tests/xk6-tempo:latest

# Login to OpenShift registry
podman login -u $(oc whoami) -p $(oc whoami -t) \
  image-registry.openshift-image-registry.svc:5000

# Push image
podman push image-registry.openshift-image-registry.svc:5000/tempo-performance-tests/xk6-tempo:latest
```

### 2. Update Image References

Before deploying, update the image references in the manifest files:

```bash
# Replace YOUR_PROJECT with your actual OpenShift project name
sed -i 's/YOUR_PROJECT/tempo-performance-tests/g' manifests/*.yaml
```

Or manually edit the image paths in:
- `manifests/job-ingestion-test.yaml`
- `manifests/job-query-test.yaml`
- `manifests/job-combined-test.yaml`
- `manifests/cronjob-performance-test.yaml`

### 3. Update Tempo Service Endpoints

Update the Tempo service endpoints to match your Tempo deployment:

```bash
# Replace tempo namespace and service names as needed
sed -i 's/tempo-distributor.tempo.svc.cluster.local/YOUR_TEMPO_DISTRIBUTOR_SVC/g' manifests/*.yaml
sed -i 's/tempo-query-frontend.tempo.svc.cluster.local/YOUR_TEMPO_QUERY_SVC/g' manifests/*.yaml
```

### 4. Deploy Manifests

```bash
# Apply all manifests
oc apply -f manifests/

# Or use kustomize
oc apply -k manifests/
```

### 5. Run Tests

#### Run Ingestion Test

```bash
# Create a one-time job from the template
oc create job --from=job/k6-tempo-ingestion-test ingestion-test-$(date +%s)

# Or manually create a job
oc apply -f manifests/job-ingestion-test.yaml
```

#### Run Query Test

```bash
oc create job --from=job/k6-tempo-query-test query-test-$(date +%s)
```

#### Run Combined Test

```bash
oc create job --from=job/k6-tempo-combined-test combined-test-$(date +%s)
```

### 6. Monitor Test Execution

```bash
# List all jobs
oc get jobs -l app=k6-tempo

# Watch job status
oc get jobs -l app=k6-tempo -w

# View logs
oc logs -f job/k6-tempo-ingestion-test

# Describe job for details
oc describe job/k6-tempo-ingestion-test

# Get pod logs (if job has pods)
oc get pods -l app=k6-tempo
oc logs <pod-name>
```

## Configuration

### Environment Variables

The test jobs support the following environment variables:

- `TEMPO_ENDPOINT`: Tempo OTLP endpoint (default: `http://tempo-distributor:4318`)
- `TEMPO_QUERY_ENDPOINT`: Tempo query endpoint (for combined tests, default: uses TEMPO_ENDPOINT)
- `TEMPO_PROTOCOL`: OTLP protocol (`otlp-http` or `otlp-grpc`, default: `otlp-http`)
- `TEMPO_TENANT`: Tenant ID for multi-tenant deployments (default: empty)

### Resource Limits

Default resource limits are set in the job manifests:

- **Ingestion/Query Jobs**: 500m CPU request, 2 CPU limit, 512Mi memory request, 2Gi limit
- **Combined Job**: 1 CPU request, 4 CPU limit, 1Gi memory request, 4Gi limit

Adjust these based on your cluster capacity and test requirements.

### Test Scripts

Test scripts are stored in the `k6-tempo-test-scripts` ConfigMap. To update:

```bash
# Edit the ConfigMap
oc edit configmap k6-tempo-test-scripts

# Or update from file
oc apply -f manifests/configmap-test-scripts.yaml
```

## Scheduled Testing with CronJob

The CronJob runs performance tests every 6 hours by default. To customize:

```bash
# Edit the schedule
oc edit cronjob k6-tempo-performance-test

# Or update the manifest
vim manifests/cronjob-performance-test.yaml
oc apply -f manifests/cronjob-performance-test.yaml
```

### CronJob Schedule Examples

- `"0 */6 * * *"` - Every 6 hours
- `"0 0 * * *"` - Daily at midnight
- `"0 */2 * * *"` - Every 2 hours
- `"0 9 * * 1"` - Every Monday at 9 AM

## Network Policies

If your cluster uses NetworkPolicies, ensure pods can reach Tempo services:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-k6-to-tempo
  namespace: tempo-performance-tests
spec:
  podSelector:
    matchLabels:
      app: k6-tempo
  policyTypes:
  - Egress
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: tempo  # Adjust to your Tempo namespace
    ports:
    - protocol: TCP
      port: 4318  # OTLP HTTP
    - protocol: TCP
      port: 4317  # OTLP gRPC
    - protocol: TCP
      port: 3200  # Query API
```

## Service Account (Optional)

If your Tempo deployment requires authentication, create a ServiceAccount:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: k6-tempo-sa
  namespace: tempo-performance-tests
---
# Add serviceAccountName to job templates
# spec:
#   template:
#     spec:
#       serviceAccountName: k6-tempo-sa
```

## Troubleshooting

### Image Pull Errors

```bash
# Check if image exists
oc get imagestreamtag xk6-tempo:latest

# Verify registry access
oc registry info

# Check pod events
oc describe pod <pod-name>
```

### Connection Issues

```bash
# Test connectivity from a pod
oc run -it --rm debug --image=busybox --restart=Never -- \
  wget -O- http://tempo-distributor.tempo.svc.cluster.local:4318

# Check DNS resolution
oc run -it --rm debug --image=busybox --restart=Never -- \
  nslookup tempo-distributor.tempo.svc.cluster.local
```

### Job Not Starting

```bash
# Check job status
oc describe job <job-name>

# Check events
oc get events --sort-by='.lastTimestamp'

# Check resource quotas
oc describe quota -n tempo-performance-tests
```

### View Test Results

```bash
# Stream logs in real-time
oc logs -f job/k6-tempo-ingestion-test

# Export logs to file
oc logs job/k6-tempo-ingestion-test > test-results.log

# Get metrics summary (if k6 cloud output is configured)
oc logs job/k6-tempo-ingestion-test | grep -i "summary\|metric"
```

## Cleanup

```bash
# Delete all jobs
oc delete jobs -l app=k6-tempo

# Delete CronJob
oc delete cronjob k6-tempo-performance-test

# Delete ConfigMap
oc delete configmap k6-tempo-test-scripts

# Delete all resources
oc delete -f manifests/

# Or using kustomize
oc delete -k manifests/
```

## Customization

### Custom Test Scripts

1. Create your test script locally
2. Update the ConfigMap:

```bash
# Add to ConfigMap
oc create configmap k6-tempo-test-scripts \
  --from-file=custom-test.js=/path/to/custom-test.js \
  --dry-run=client -o yaml | oc apply -f -
```

3. Create a new Job manifest referencing the script

### Different Test Scenarios

Modify the test scripts in the ConfigMap or create new ones:

- Adjust `rate` for different QPS/MBps targets
- Modify `duration` for test length
- Change `preAllocatedVUs` and `maxVUs` for concurrency
- Update trace generation parameters for different payload sizes

## Best Practices

1. **Start Small**: Begin with lower rates and shorter durations
2. **Monitor Resources**: Watch Tempo and cluster resource usage during tests
3. **Use Namespaces**: Isolate test workloads in dedicated namespaces
4. **Clean Up**: Set appropriate `ttlSecondsAfterFinished` to auto-cleanup jobs
5. **Version Control**: Tag images with versions for reproducibility
6. **Document Results**: Export logs and metrics for analysis

## Additional Resources

- [k6 Documentation](https://grafana.com/docs/k6/latest/)
- [OpenShift Documentation](https://docs.openshift.com/)
- [Kubernetes Jobs Documentation](https://kubernetes.io/docs/concepts/workloads/controllers/job/)
- [Kubernetes CronJobs Documentation](https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/)

