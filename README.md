# Kubernetes ConfigMap Cleaner

A Go-based tool to identify and optionally delete unused ConfigMaps in your Kubernetes cluster.
The tool scans your cluster for ConfigMap references in various Kubernetes resources and helps you clean up unused ConfigMaps safely.

## Features

- Scans for ConfigMap usage across multiple resource types:
    - Pods
    - Deployments
    - StatefulSets
    - DaemonSets
    - Jobs
    - CronJobs
- Checks both volume mounts and environment variables
- Supports scanning specific namespaces or entire cluster
- Interactive deletion confirmation for safety
- Uses current kubectl context
- Detailed success/failure reporting for deletions

## Prerequisites

- Go 1.16 or later
- Access to a Kubernetes cluster
- kubectl configured with appropriate context and permissions
- Required RBAC permissions:
    - `get`, `list` on namespaces
    - `get`, `list` on pods, deployments, statefulsets, daemonsets, jobs, cronjobs
    - `get`, `list`, `delete` on configmaps (if using deletion feature)

## Installation

1. Clone the repository:
    ```bash
    git clone https://github.com/junereycasuga/k8s-configmap-cleaner.git
    cd k8s-configmap-cleaner
    ```

2. Initialize the Go module and install dependencies:
    ```bash
    go mod init configmap-cleaner
    go mod tidy
    ```

3. Build the binary:
    ```bash
    go build -o configmap-cleaner
    ```

## Usage

### Run without building

```bash
# Scan all namespaces
go run main.go

# Scan specific namespace
go run main.go --namespace my-namespace

# Scan and delete unused ConfigMaps in all namespaces
go run main.go --delete

# Scan and delete unused ConfigMaps in specific namespace
go run main.go --namespace my-namespace --delete
```

### Run built binary

```bash
# Scan all namespaces
./configmap-cleaner

# Scan specific namespace
./configmap-cleaner --namespace my-namespace

# Scan and delete unused ConfigMaps in all namespaces
./configmap-cleaner --delete

# Scan and delete unused ConfigMaps in specific namespace
./configmap-cleaner --namespace my-namespace --delete
```

## Command Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--namespace` | Namespace to scan | ""(all namespaces) |
| `--delete` | Enable deletion of unused ConfigMaps | false |

## Safety Features

1. Delete operation requires explicit `--delete` flag
2. Interactive confirmation prompt before deletion
3. Detailed preview of ConfigMaps to be deleted
4. Comprehensive error reporting
5. Namespaces validation before operations

## Output Example

```bash
Using current context: my-cluster-context
Scanning namespace: my-namespace

ConfigMaps currently in use:
============================
Namespace: my-namespace, ConfigMap: app-config
Namespace: my-namespace, ConfigMap: logging-config

Unused ConfigMaps:
==================
Namespace: my-namespace, ConfigMap: old-config
Namespace: my-namespace, ConfigMap: test-config
```

## Limitations

1. The tool only detects ConfigMap references in the following ways:
    - Volume mounts
    - Environment variables (direct and from ConfigMap)
    - envFrom references
2. Custom resource definitions (CRDs) that might reference ConfigMaps are not scanned
3. The tool cannot detect programmatic references to ConfigMaps from within applications

## Best Practices

1. Always run without `--delete` flag first to review what would be deleted
2. Back up important ConfigMaps before deletion
3. Use with caution in production environments
4. Verify you have the necessary permissions before running
5. Consider running in a specific namespace instead of cluster-wide

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Disclaimer

This tool comes with no warranties. Always verify the results before deleting any resources from your cluster.
