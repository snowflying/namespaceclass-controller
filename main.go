package main

import (
        "context"
        "fmt"
        "log"
        "time"

        corev1 "k8s.io/api/core/v1"
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
        "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
        "k8s.io/apimachinery/pkg/runtime/schema"
        "k8s.io/apimachinery/pkg/watch"
        "k8s.io/client-go/dynamic"
        "k8s.io/client-go/kubernetes"
        "k8s.io/client-go/rest"
)

const (
        ClassLabel      = "namespaceclass.snowflying.io/name"
        ManagedLabel    = "namespaceclass.snowflying.io/managed"
        OwnerClassLabel = "namespaceclass.snowflying.io/owner"
)

type Controller struct {
        client        *kubernetes.Clientset
        dynamicClient dynamic.Interface
}

func NewController(config *rest.Config) (*Controller, error) {
        log.Println("[INIT] Creating Kubernetes client...")
        client, err := kubernetes.NewForConfig(config)
        if err != nil {
                return nil, err
        }
        log.Println("[INIT] Kubernetes client created successfully")

        log.Println("[INIT] Creating dynamic client...")
        dynamicClient, err := dynamic.NewForConfig(config)
        if err != nil {
                return nil, err
        }
        log.Println("[INIT] Dynamic client created successfully")

        return &Controller{
                client:        client,
                dynamicClient: dynamicClient,
        }, nil
}

func (c *Controller) Run(ctx context.Context) error {
        log.Println("==========================================")
        log.Println("[START] NamespaceClass Controller Starting")
        log.Println("==========================================")

        log.Println("[START] Launching watchers in background...")
        go c.watchNamespaces(ctx)
        go c.watchClasses(ctx)
        log.Println("[START] Watchers launched successfully")
        log.Println("")

        <-ctx.Done()
        log.Println("[STOP] Controller stopped")
        return nil
}

func (c *Controller) watchNamespaces(ctx context.Context) {
        log.Println("[WATCH] Starting to watch Namespaces...")

        for {
                watcher, err := c.client.CoreV1().Namespaces().Watch(ctx, metav1.ListOptions{})
                if err != nil {
                        log.Printf("[ERROR] Failed to create namespace watcher: %v", err)
                        time.Sleep(5 * time.Second)
                        continue
                }

                log.Println("[WATCH] Namespace watcher connected and listening")

                for event := range watcher.ResultChan() {
                        ns, ok := event.Object.(*corev1.Namespace)
                        if !ok {
                                continue
                        }

                        log.Println("")
                        log.Printf("[EVENT] Namespace %s: %s", event.Type, ns.Name)

                        switch event.Type {
                        case watch.Added:
                                log.Printf("[EVENT] Handling namespace ADD event")
                                c.handleNamespace(ctx, ns)

                        case watch.Modified:
                                log.Printf("[EVENT] Handling namespace MODIFY event")
                                c.handleNamespace(ctx, ns)

                        case watch.Deleted:
                                log.Printf("[EVENT] Namespace was deleted, no action needed")
                        }
                }

                log.Println("[WARN] Namespace watch disconnected, reconnecting in 1 second...")
                time.Sleep(time.Second)
        }
}

func (c *Controller) watchClasses(ctx context.Context) {
        log.Println("[WATCH] Starting to watch NamespaceClasses...")

        gvr := schema.GroupVersionResource{
                Group:    "snowflying.io",
                Version:  "v1alpha1",
                Resource: "namespaceclasses",
        }

        for {
                watcher, err := c.dynamicClient.Resource(gvr).Watch(ctx, metav1.ListOptions{})
                if err != nil {
                        log.Printf("[ERROR] Failed to create class watcher: %v", err)
                        time.Sleep(5 * time.Second)
                        continue
                }

                log.Println("[WATCH] NamespaceClass watcher connected and listening")

                for event := range watcher.ResultChan() {
                        class, ok := event.Object.(*unstructured.Unstructured)
                        if !ok {
                                continue
                        }

                        log.Println("")
                        log.Printf("[EVENT] NamespaceClass %s: %s", event.Type, class.GetName())

                        switch event.Type {
                        case watch.Added:
                                log.Printf("[EVENT] NamespaceClass added, ready for use")

                        case watch.Modified:
                                log.Printf("[EVENT] NamespaceClass modified, updating all namespaces...")
                                c.updateNamespacesWithClass(ctx, class.GetName())

                        case watch.Deleted:
                                log.Printf("[EVENT] NamespaceClass deleted, cleaning up all namespaces...")
                                c.cleanupNamespacesWithClass(ctx, class.GetName())
                        }
                }

                log.Println("[WARN] NamespaceClass watch disconnected, reconnecting in 1 second...")
                time.Sleep(time.Second)
        }
}

func (c *Controller) handleNamespace(ctx context.Context, ns *corev1.Namespace) {
        log.Printf("[STEP1] Checking labels on namespace: %s", ns.Name)
        className, hasClass := ns.Labels[ClassLabel]

        if !hasClass {
                log.Printf("[STEP1] No class label found on namespace")
                log.Printf("[STEP1] Cleaning up any managed resources...")
                c.cleanupResources(ctx, ns.Name, "")
                return
        }

        log.Printf("[STEP1] Found class label: %s", className)

        log.Printf("[STEP2] Fetching NamespaceClass definition: %s", className)
        class, err := c.getClass(ctx, className)
        if err != nil {
                log.Printf("[ERROR] Failed to get NamespaceClass: %v", err)
                return
        }
        log.Printf("[STEP2] Successfully retrieved NamespaceClass")

        log.Printf("[STEP3] Applying class to namespace...")
        c.applyClass(ctx, ns.Name, className, class)
}

func (c *Controller) applyClass(ctx context.Context, nsName, className string, class *unstructured.Unstructured) {
        log.Printf("[APPLY] Starting to apply class '%s' to namespace '%s'", className, nsName)

        log.Printf("[APPLY] Phase 1: Cleaning up old resources...")
        c.cleanupResources(ctx, nsName, className)

        log.Printf("[APPLY] Phase 2: Extracting resources from class definition...")
        resources, err := c.getResourcesFromClass(class)
        if err != nil {
                log.Printf("[ERROR] Failed to extract resources: %v", err)
                return
        }
        log.Printf("[APPLY] Found %d resource(s) to create", len(resources))

        log.Printf("[APPLY] Phase 3: Creating resources in namespace...")
        successCount := 0
        for i, resource := range resources {
                log.Printf("[APPLY] Creating resource %d/%d: %s/%s",
                        i+1, len(resources), resource.GetKind(), resource.GetName())

                err := c.createResource(ctx, nsName, className, resource)
                if err != nil {
                        log.Printf("[ERROR] Failed to create resource: %v", err)
                } else {
                        log.Printf("[APPLY] Resource created successfully")
                        successCount++
                }
        }

        log.Printf("[APPLY] Finished applying class: %d/%d resources created", successCount, len(resources))
}

func (c *Controller) getResourcesFromClass(class *unstructured.Unstructured) ([]unstructured.Unstructured, error) {
        spec, found, err := unstructured.NestedMap(class.Object, "spec")
        if err != nil || !found {
                return nil, fmt.Errorf("spec not found in class")
        }

        resourcesList, found, err := unstructured.NestedSlice(spec, "resources")
        if err != nil || !found {
                return nil, fmt.Errorf("resources not found in spec")
        }

        var resources []unstructured.Unstructured
        for _, item := range resourcesList {
                resourceMap, ok := item.(map[string]interface{})
                if !ok {
                        continue
                }
                resource := unstructured.Unstructured{Object: resourceMap}
                resources = append(resources, resource)
        }

        return resources, nil
}

func (c *Controller) createResource(ctx context.Context, nsName, className string, resource unstructured.Unstructured) error {
        resource.SetNamespace(nsName)

        labels := resource.GetLabels()
        if labels == nil {
                labels = make(map[string]string)
        }
        labels[ManagedLabel] = "true"
        labels[OwnerClassLabel] = className
        resource.SetLabels(labels)

        gvk := resource.GroupVersionKind()
        gvr := schema.GroupVersionResource{
                Group:    gvk.Group,
                Version:  gvk.Version,
                Resource: kindToResource(gvk.Kind),
        }

        _, err := c.dynamicClient.Resource(gvr).Namespace(nsName).Create(ctx, &resource, metav1.CreateOptions{})
        return err
}

func (c *Controller) cleanupResources(ctx context.Context, nsName, className string) {
        resourceTypes := []schema.GroupVersionResource{
                {Version: "v1", Resource: "configmaps"},
                {Version: "v1", Resource: "secrets"},
                {Version: "v1", Resource: "serviceaccounts"},
                {Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"},
                {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
                {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
                {Version: "v1", Resource: "resourcequotas"},
                {Version: "v1", Resource: "limitranges"},
        }

        selector := fmt.Sprintf("%s=true", ManagedLabel)
        if className != "" {
                selector = fmt.Sprintf("%s,%s=%s", selector, OwnerClassLabel, className)
        }

        deletedCount := 0

        for _, gvr := range resourceTypes {
                list, err := c.dynamicClient.Resource(gvr).Namespace(nsName).List(ctx, metav1.ListOptions{
                        LabelSelector: selector,
                })
                if err != nil {
                        continue
                }

                for _, item := range list.Items {
                        log.Printf("[CLEANUP] Deleting %s: %s", gvr.Resource, item.GetName())
                        err := c.dynamicClient.Resource(gvr).Namespace(nsName).Delete(ctx, item.GetName(), metav1.DeleteOptions{})
                        if err != nil {
                                log.Printf("[ERROR] Failed to delete: %v", err)
                        } else {
                                deletedCount++
                        }
                }
        }

        if deletedCount > 0 {
                log.Printf("[CLEANUP] Deleted %d resource(s)", deletedCount)
        } else {
                log.Printf("[CLEANUP] No resources to clean up")
        }
}

func (c *Controller) getClass(ctx context.Context, name string) (*unstructured.Unstructured, error) {
        gvr := schema.GroupVersionResource{
                Group:    "snowflying.io",
                Version:  "v1alpha1",
                Resource: "namespaceclasses",
        }

        class, err := c.dynamicClient.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
        return class, err
}

func (c *Controller) updateNamespacesWithClass(ctx context.Context, className string) {
        log.Printf("[UPDATE] Finding all namespaces with class: %s", className)

        namespaces, err := c.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
                LabelSelector: fmt.Sprintf("%s=%s", ClassLabel, className),
        })
        if err != nil {
                log.Printf("[ERROR] Failed to list namespaces: %v", err)
                return
        }

        log.Printf("[UPDATE] Found %d namespace(s) to update", len(namespaces.Items))

        class, err := c.getClass(ctx, className)
        if err != nil {
                log.Printf("[ERROR] Failed to get class: %v", err)
                return
        }

        for _, ns := range namespaces.Items {
                log.Printf("[UPDATE] Updating namespace: %s", ns.Name)
                c.applyClass(ctx, ns.Name, className, class)
        }
}

func (c *Controller) cleanupNamespacesWithClass(ctx context.Context, className string) {
        log.Printf("[DELETE] Finding all namespaces with class: %s", className)

        namespaces, err := c.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
                LabelSelector: fmt.Sprintf("%s=%s", ClassLabel, className),
        })
        if err != nil {
                log.Printf("[ERROR] Failed to list namespaces: %v", err)
                return
        }

        log.Printf("[DELETE] Found %d namespace(s) to clean up", len(namespaces.Items))

        for _, ns := range namespaces.Items {
                log.Printf("[DELETE] Cleaning up namespace: %s", ns.Name)
                c.cleanupResources(ctx, ns.Name, className)
        }
}

func kindToResource(kind string) string {
        switch kind {
        case "ConfigMap":
                return "configmaps"
        case "Secret":
                return "secrets"
        case "ServiceAccount":
                return "serviceaccounts"
        case "NetworkPolicy":
                return "networkpolicies"
        case "Role":
                return "roles"
        case "RoleBinding":
                return "rolebindings"
        case "ResourceQuota":
                return "resourcequotas"
        case "LimitRange":
                return "limitranges"
        case "Endpoints":
                return "endpoints"
        default:
                return fmt.Sprintf("%ss", kind)
        }
}

func main() {
        log.Println("")
        log.Println("==========================================")
        log.Println("NamespaceClass Controller")
        log.Println("Domain: snowflying.io")
        log.Println("==========================================")
        log.Println("")

        log.Println("[MAIN] Getting Kubernetes configuration...")
        config, err := rest.InClusterConfig()
        if err != nil {
                log.Fatalf("[FATAL] Failed to get config: %v", err)
        }
        log.Println("[MAIN] Kubernetes configuration loaded")
        log.Println("")

        controller, err := NewController(config)
        if err != nil {
                log.Fatalf("[FATAL] Failed to create controller: %v", err)
        }
        log.Println("")

        ctx := context.Background()
        if err := controller.Run(ctx); err != nil {
                log.Fatalf("[FATAL] Controller failed: %v", err)
        }
}
