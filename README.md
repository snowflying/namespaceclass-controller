# NamespaceClass Controller

A Kubernetes controller that enables namespace templating through custom resource definitions. Define reusable namespace "classes" that automatically provision resources like NetworkPolicies, ServiceAccounts, RBAC rules, and more when namespaces are created.

## Overview

The NamespaceClass controller allows Kubernetes administrators to define templates (classes) of resources that should be automatically created and maintained in namespaces. This is useful for:

- Enforcing network policies across similar namespaces
- Automatically provisioning ServiceAccounts and RBAC
- Setting up resource quotas and limits
- Standardizing namespace configurations

## Features

- **Flexible Resource Templates**: Define any Kubernetes resource in a NamespaceClass
- **Automatic Resource Management**: Resources are created when namespaces join a class
- **Class Switching**: Change a namespace's class and resources are automatically updated
- **Class Updates**: Modify a NamespaceClass and all namespaces using it are updated
- **Garbage Collection**: Resources are cleaned up when switching classes or removing class labels

## Architecture

The controller watches for two types of events:

1. **Namespace Events**: When a namespace is created or labeled with a class
2. **NamespaceClass Events**: When a class definition is created or updated

### How It Works

1. Admin creates a `NamespaceClass` defining a set of resources
2. Admin labels a namespace with `namespaceclass.snowflying.io/name: <class-name>`
3. Controller detects the label and creates all resources with those particular labels from the class in that namespace
4. All created resources are labeled with management metadata for tracking
5. If the class changes, controller updates resources in all namespaces using that class
6. If namespace switches classes, old resources are deleted and new ones created

## Installation

### Prerequisites

- Kubernetes cluster (v1.20+)
- kubectl configured to access the cluster
- Docker (for building the controller image)

### Quick Start

```bash
# 1. Install the CRD
kubectl apply -f config/crd/namespaceclass-crd.yaml

# 2. Deploy the controller
kubectl apply -f config/deployment/

# 3. Verify the controller is running
kubectl get pods -n namespaceclass-system

# 4. Apply example NamespaceClasses
kubectl apply -f examples/
```

## Usage

### Creating a NamespaceClass

Define a NamespaceClass with resources that should be created:

```yaml
apiVersion: snowflying.io/v1alpha1
kind: NamespaceClass
metadata:
  name: secure-network
spec:
  resources:
  - apiVersion: networking.k8s.io/v1
    kind: NetworkPolicy
    metadata:
      name: deny-all-ingress
    spec:
      podSelector: {}
      policyTypes:
      - Ingress
  - apiVersion: v1
    kind: ServiceAccount
    metadata:
      name: app-service-account
```

### Using a NamespaceClass

Label a namespace to apply a class:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: my-app
  labels:
    namespaceclass.snowflying.io/name: secure-network
```

Or label an existing namespace:

```bash
kubectl label namespace my-app namespaceclass.snowflying.io/name=secure-network
```

### Switching Classes

Simply change the label to switch to a different class:

```bash
kubectl label namespace my-app namespaceclass.snowflying.io/name=public-network --overwrite
```

The controller will:
1. Delete resources from the old class
2. Create resources from the new class

### Removing a Class

Remove the label to clean up managed resources:

```bash
kubectl label namespace my-app namespaceclass.snowflying.io/name-
```

### Updating a Class

Modify the NamespaceClass resource:

```bash
kubectl edit namespaceclass secure-network
```

All namespaces using this class will be automatically updated.

### Viewing Class Status

Check which namespaces are using a class:

```bash
kubectl get namespaceclass secure-network -o yaml
```

The status section shows:
- Number of managed namespaces
- Observed generation
- Conditions

## Examples

### Example 1: Network Policies

```yaml
apiVersion: snowflying.io/v1alpha1
kind: NamespaceClass
metadata:
  name: public-network
spec:
  resources:
  - apiVersion: networking.k8s.io/v1
    kind: NetworkPolicy
    metadata:
      name: allow-public-ingress
    spec:
      podSelector: {}
      policyTypes:
      - Ingress
      ingress:
      - from:
        - ipBlock:
            cidr: 0.0.0.0/0
```

### Example 2: Resource Quotas

```yaml
apiVersion: snowflying.io/v1alpha1
kind: NamespaceClass
metadata:
  name: development
spec:
  resources:
  - apiVersion: v1
    kind: ResourceQuota
    metadata:
      name: dev-quota
    spec:
      hard:
        requests.cpu: "4"
        requests.memory: 8Gi
        pods: "20"
  - apiVersion: v1
    kind: LimitRange
    metadata:
      name: dev-limits
    spec:
      limits:
      - max:
          cpu: "2"
          memory: 4Gi
        default:
          cpu: 500m
          memory: 512Mi
        type: Container
```

### Example 3: RBAC Setup

```yaml
apiVersion: snowflying.io/v1alpha1
kind: NamespaceClass
metadata:
  name: team-workspace
spec:
  resources:
  - apiVersion: v1
    kind: ServiceAccount
    metadata:
      name: team-admin
  - apiVersion: rbac.authorization.k8s.io/v1
    kind: Role
    metadata:
      name: admin-role
    rules:
    - apiGroups: ["*"]
      resources: ["*"]
      verbs: ["*"]
  - apiVersion: rbac.authorization.k8s.io/v1
    kind: RoleBinding
    metadata:
      name: team-admin-binding
    roleRef:
      apiGroup: rbac.authorization.k8s.io
      kind: Role
      name: admin-role
    subjects:
    - kind: ServiceAccount
      name: team-admin
```

### Running Locally

```bash
# Run the controller locally (outside cluster)
# make sure you have the kubeconfig 'config' file under your .kube in home folder
go run main.go
```

## Configuration

The controller uses the following labels and annotations:

| Name | Type | Purpose |
|------|------|---------|
| `namespaceclass.snowflying.io/name` | Label | Specifies which class a namespace uses |
| `namespaceclass.snowflying.io/managed` | Label | Marks resources as controller-managed |
| `namespaceclass.snowflying.io/owner` | Label | Tracks which class created the resource |

## Troubleshooting

### Resources Not Created

Check the controller logs:

```bash
kubectl logs -n namespaceclass-system deployment/namespaceclass-controller
```

Verify the NamespaceClass exists:

```bash
kubectl get namespaceclass
```

Check the namespace has the correct label:

```bash
kubectl get namespace <name> --show-labels
```

### Resources Not Deleted

Ensure resources have the management labels. List resources in the namespace:

```bash
kubectl get all -n <namespace> --show-labels
```

Manually clean up if needed:

```bash
kubectl delete <resource> -n <namespace> -l namespaceclass.snowflying.io/managed=true
```


## Limitations

- Resources must be namespace-scoped (cluster-scoped resources are not supported)
- Circular dependencies between resources are not handled
- Resource creation order is not guaranteed
- Large numbers of resources in a class may cause performance issues

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

Apache License 2.0

## Contact

For questions or issues, please open a GitHub issue.
