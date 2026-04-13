package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"

	"github.com/google/uuid"
)

type Orchestrator struct {
	k8s         kubernetes.Interface
	mlEngineURL string
	logger      *zap.Logger
	httpClient  *http.Client
}

func New(k8s kubernetes.Interface, mlEngineURL string, logger *zap.Logger) *Orchestrator {
	return &Orchestrator{
		k8s:         k8s,
		mlEngineURL: mlEngineURL,
		logger:      logger,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

// ── Types ─────────────────────────────────────────────────────────────────────

type ResourceSpec struct {
	CPURequest    string
	CPULimit      string
	MemoryRequest string
	MemoryLimit   string
}

type DeploySpec struct {
	Name      string
	Image     string
	Replicas  int32
	Namespace string
	DryRun    bool
	Resources ResourceSpec
	EnvVars   map[string]string
	Port      *int32
}

type DeployResult struct {
	DeployID             string  `json:"deploy_id"`
	Status               string  `json:"status"`
	EstimatedCostMonthly float64 `json:"estimated_cost_monthly"`
	ManifestPreview      string  `json:"manifest_preview,omitempty"`
	AppURL               string  `json:"app_url,omitempty"`
}

type Application struct {
	Name          string            `json:"name"`
	Namespace     string            `json:"namespace"`
	Image         string            `json:"image"`
	Replicas      int32             `json:"replicas"`
	ReadyReplicas int32             `json:"ready_replicas"`
	Status        string            `json:"status"`
	CreatedAt     string            `json:"created_at"`
	UpdatedAt     string            `json:"updated_at"`
	AppURL        string            `json:"app_url,omitempty"`
	Labels        map[string]string `json:"labels"`
}

type CostRequest struct {
	Name        string
	Namespace   string
	Replicas    int32
	CPULimit    string
	MemoryLimit string
	Region      string
}

type EnvVar struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
}

type ComponentStatus struct {
	Healthy   bool   `json:"healthy"`
	LatencyMs int64  `json:"latency_ms"`
	Message   string `json:"message,omitempty"`
}

type PlatformStatusResult struct {
	ControlPlane    ComponentStatus `json:"control_plane"`
	MLEngine        ComponentStatus `json:"ml_engine"`
	Kubernetes      ComponentStatus `json:"kubernetes"`
	Database        ComponentStatus `json:"database"`
	TotalApps       int             `json:"total_apps"`
	TotalNamespaces int             `json:"total_namespaces"`
}

// ── Deploy ────────────────────────────────────────────────────────────────────

func (o *Orchestrator) Deploy(ctx context.Context, spec *DeploySpec) (*DeployResult, error) {
	deployID := uuid.New().String()[:8]

	deployment := o.buildDeployment(spec, deployID)
	service := o.buildService(spec)

	if spec.DryRun {
		return &DeployResult{
			DeployID: deployID,
			Status:   "dry-run",
		}, nil
	}

	// Apply Deployment
	existing, err := o.k8s.AppsV1().
		Deployments(spec.Namespace).
		Get(ctx, spec.Name, metav1.GetOptions{})

	if errors.IsNotFound(err) {
		_, err = o.k8s.AppsV1().
			Deployments(spec.Namespace).
			Create(ctx, deployment, metav1.CreateOptions{})
	} else if err == nil {
		existing.Spec = deployment.Spec
		existing.Labels = deployment.Labels
		_, err = o.k8s.AppsV1().
			Deployments(spec.Namespace).
			Update(ctx, existing, metav1.UpdateOptions{})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to apply deployment: %w", err)
	}

	// Apply Service
	_, svcErr := o.k8s.CoreV1().
		Services(spec.Namespace).
		Get(ctx, spec.Name, metav1.GetOptions{})

	if errors.IsNotFound(svcErr) {
		_, err = o.k8s.CoreV1().
			Services(spec.Namespace).
			Create(ctx, service, metav1.CreateOptions{})
		if err != nil {
			o.logger.Warn("failed to create service", zap.Error(err))
		}
	}

	o.logger.Info("application deployed",
		zap.String("name", spec.Name),
		zap.String("namespace", spec.Namespace),
		zap.String("deploy_id", deployID),
	)

	return &DeployResult{
		DeployID: deployID,
		Status:   "deployed",
		AppURL:   fmt.Sprintf("http://%s.%s.svc.cluster.local", spec.Name, spec.Namespace),
	}, nil
}

func (o *Orchestrator) buildDeployment(spec *DeploySpec, deployID string) *appsv1.Deployment {
	labels := map[string]string{
		"app":        spec.Name,
		"managed-by": "idp",
		"deploy-id":  deployID,
	}

	replicas := spec.Replicas
	envVars := []corev1.EnvVar{}
	for k, v := range spec.EnvVars {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}

	resourceReqs := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(orDefault(spec.Resources.CPURequest, "100m")),
			corev1.ResourceMemory: resource.MustParse(orDefault(spec.Resources.MemoryRequest, "128Mi")),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(orDefault(spec.Resources.CPULimit, "500m")),
			corev1.ResourceMemory: resource.MustParse(orDefault(spec.Resources.MemoryLimit, "512Mi")),
		},
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: spec.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"idp.io/deploy-id": deployID,
				"idp.io/managed":   "true",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": spec.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:      spec.Name,
							Image:     spec.Image,
							Env:       envVars,
							Resources: resourceReqs,
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health",
										Port: intOrNamedPort(spec.Port),
									},
								},
								InitialDelaySeconds: 15,
								PeriodSeconds:       20,
							},
						},
					},
				},
			},
		},
	}
}

func (o *Orchestrator) buildService(spec *DeploySpec) *corev1.Service {
	port := int32(80)
	if spec.Port != nil {
		port = *spec.Port
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: spec.Namespace,
			Labels:    map[string]string{"app": spec.Name, "managed-by": "idp"},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": spec.Name},
			Ports: []corev1.ServicePort{
				{Port: port, Protocol: corev1.ProtocolTCP},
			},
		},
	}
}

// ── List / Get / Delete ───────────────────────────────────────────────────────

func (o *Orchestrator) ListApplications(ctx context.Context, namespace string) ([]*Application, error) {
	deployments, err := o.k8s.AppsV1().
		Deployments(namespace).
		List(ctx, metav1.ListOptions{
			LabelSelector: "managed-by=idp",
		})
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}

	apps := make([]*Application, 0, len(deployments.Items))
	for _, d := range deployments.Items {
		apps = append(apps, deploymentToApp(&d))
	}
	return apps, nil
}

func (o *Orchestrator) GetApplication(ctx context.Context, namespace, name string) (*Application, error) {
	d, err := o.k8s.AppsV1().
		Deployments(namespace).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("application %q not found", name)
		}
		return nil, err
	}
	return deploymentToApp(d), nil
}

func (o *Orchestrator) DeleteApplication(ctx context.Context, namespace, name string) error {
	policy := metav1.DeletePropagationForeground
	err := o.k8s.AppsV1().
		Deployments(namespace).
		Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &policy})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}
	_ = o.k8s.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	return nil
}

func (o *Orchestrator) Scale(ctx context.Context, namespace, name string, replicas int32) (*Application, error) {
	d, err := o.k8s.AppsV1().
		Deployments(namespace).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	d.Spec.Replicas = &replicas
	d, err = o.k8s.AppsV1().
		Deployments(namespace).
		Update(ctx, d, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to scale: %w", err)
	}
	return deploymentToApp(d), nil
}

// ── Logs ──────────────────────────────────────────────────────────────────────

func (o *Orchestrator) StreamLogs(
	ctx context.Context,
	w io.Writer,
	namespace, name string,
	tail int64,
	follow bool,
) error {
	pods, err := o.k8s.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", name),
	})
	if err != nil || len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for %s", name)
	}

	pod := pods.Items[0].Name
	req := o.k8s.CoreV1().Pods(namespace).GetLogs(pod, &corev1.PodLogOptions{
		TailLines: &tail,
		Follow:    follow,
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		return fmt.Errorf("failed to open log stream: %w", err)
	}
	defer stream.Close()

	_, err = io.Copy(w, stream)
	return err
}

// ── Env Vars ──────────────────────────────────────────────────────────────────

func (o *Orchestrator) ListEnvVars(ctx context.Context, namespace, name string) ([]*EnvVar, error) {
	d, err := o.k8s.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var vars []*EnvVar
	if len(d.Spec.Template.Spec.Containers) > 0 {
		for _, e := range d.Spec.Template.Spec.Containers[0].Env {
			vars = append(vars, &EnvVar{Key: e.Name, Value: e.Value})
		}
	}
	return vars, nil
}

func (o *Orchestrator) SetEnvVars(ctx context.Context, namespace, name string, vars []EnvVar) error {
	d, err := o.k8s.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if len(d.Spec.Template.Spec.Containers) == 0 {
		return fmt.Errorf("no containers found")
	}

	existing := d.Spec.Template.Spec.Containers[0].Env
	for _, v := range vars {
		found := false
		for i, e := range existing {
			if e.Name == v.Key {
				existing[i].Value = v.Value
				found = true
				break
			}
		}
		if !found {
			existing = append(existing, corev1.EnvVar{Name: v.Key, Value: v.Value})
		}
	}
	d.Spec.Template.Spec.Containers[0].Env = existing

	_, err = o.k8s.AppsV1().Deployments(namespace).Update(ctx, d, metav1.UpdateOptions{})
	return err
}

func (o *Orchestrator) DeleteEnvVar(ctx context.Context, namespace, name, key string) error {
	d, err := o.k8s.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if len(d.Spec.Template.Spec.Containers) > 0 {
		var filtered []corev1.EnvVar
		for _, e := range d.Spec.Template.Spec.Containers[0].Env {
			if e.Name != key {
				filtered = append(filtered, e)
			}
		}
		d.Spec.Template.Spec.Containers[0].Env = filtered
	}

	_, err = o.k8s.AppsV1().Deployments(namespace).Update(ctx, d, metav1.UpdateOptions{})
	return err
}

// ── Cost ──────────────────────────────────────────────────────────────────────

func (o *Orchestrator) EstimateCost(ctx context.Context, req CostRequest) (map[string]any, error) {
	// Proxy to ML engine
	body, _ := json.Marshal(map[string]any{
		"name":         req.Name,
		"namespace":    req.Namespace,
		"replicas":     req.Replicas,
		"cpu_limit":    req.CPULimit,
		"memory_limit": req.MemoryLimit,
		"region":       req.Region,
	})

	httpReq, _ := http.NewRequestWithContext(ctx, "POST",
		o.mlEngineURL+"/predict/cost",
		bytes.NewReader(body),
	)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(httpReq)
	if err != nil {
		// Fallback to simple calculation
		return o.simpleCostEstimate(req), nil
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return o.simpleCostEstimate(req), nil
	}
	return result, nil
}

func (o *Orchestrator) simpleCostEstimate(req CostRequest) map[string]any {
	monthly := float64(req.Replicas) * 15.0 // $15 per replica/month baseline
	return map[string]any{
		"monthly_estimate": monthly,
		"daily_estimate":   monthly / 30,
		"confidence":       0.5,
		"breakdown": map[string]any{
			"compute":    monthly * 0.6,
			"memory":     monthly * 0.3,
			"networking": monthly * 0.05,
			"storage":    monthly * 0.05,
			"overhead":   0.0,
		},
		"recommendations": []string{},
	}
}

// ── Platform Status ───────────────────────────────────────────────────────────

func (o *Orchestrator) PlatformStatus(ctx context.Context) *PlatformStatusResult {
	result := &PlatformStatusResult{
		ControlPlane: ComponentStatus{Healthy: true, LatencyMs: 0},
	}

	// Check K8s
	start := time.Now()
	_, err := o.k8s.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	result.Kubernetes = ComponentStatus{
		Healthy:   err == nil,
		LatencyMs: time.Since(start).Milliseconds(),
	}

	// Check ML Engine
	start = time.Now()
	resp, err := o.httpClient.Get(o.mlEngineURL + "/health")
	result.MLEngine = ComponentStatus{
		Healthy:   err == nil && resp != nil && resp.StatusCode == 200,
		LatencyMs: time.Since(start).Milliseconds(),
	}
	if resp != nil {
		resp.Body.Close()
	}

	return result
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func deploymentToApp(d *appsv1.Deployment) *Application {
	status := "pending"
	if d.Status.ReadyReplicas == *d.Spec.Replicas {
		status = "running"
	} else if d.Status.UnavailableReplicas > 0 {
		status = "deploying"
	}

	return &Application{
		Name:          d.Name,
		Namespace:     d.Namespace,
		Image:         d.Spec.Template.Spec.Containers[0].Image,
		Replicas:      *d.Spec.Replicas,
		ReadyReplicas: d.Status.ReadyReplicas,
		Status:        status,
		CreatedAt:     d.CreationTimestamp.String(),
		UpdatedAt:     d.CreationTimestamp.String(),
		Labels:        d.Labels,
	}
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func intOrNamedPort(p *int32) intstr.IntOrString {
	if p != nil {
		return intstr.FromInt(int(*p))
	}
	return intstr.FromInt(8080)
}
