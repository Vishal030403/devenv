package state

import (
	"fmt"
	"strings"
	"time"
)

const maxStableHistoryEntries = 20

// WorkloadDeploymentState tracks CI/CD artifact and rollback metadata for one Kubernetes Deployment.
type WorkloadDeploymentState struct {
	AppName        string    `json:"app_name"`
	Namespace      string    `json:"namespace"`
	Deployment     string    `json:"deployment"`
	ContainerName  string    `json:"container_name"`
	CurrentImage   string    `json:"current_image"`
	PreviousImage  string    `json:"previous_image"`
	CurrentTag     string    `json:"current_tag"`
	PreviousTag    string    `json:"previous_tag"`
	RollbackTarget string    `json:"rollback_target"`
	StableHistory  []string  `json:"stable_history"`
	RegistryTags   []string  `json:"registry_tags"`
	RegistryHost   string    `json:"registry_host"`
	LastResult     string    `json:"last_result"`
	LastDeployedAt time.Time `json:"last_deployed_at"`
}

// DeploymentResult values for LastResult.
const (
	DeploymentInProgress = "in_progress"
	DeploymentSuccess    = "success"
	DeploymentFailed     = "failed"
	DeploymentRolledBack = "rollback"
)

func workloadKey(namespace, deployment string) string {
	return namespace + "/" + deployment
}

// GetWorkloadDeployment returns deployment tracking for a workload (ok false if never recorded).
func GetWorkloadDeployment(namespace, deployment string) (WorkloadDeploymentState, bool, error) {
	st, err := GetRuntimeState()
	if err != nil {
		return WorkloadDeploymentState{}, false, err
	}
	st.mu.RLock()
	defer st.mu.RUnlock()
	if st.Deployments == nil {
		return WorkloadDeploymentState{}, false, nil
	}
	w, ok := st.Deployments[workloadKey(namespace, deployment)]
	return w, ok, nil
}

// UpdateWorkloadDeployment mutates or creates deployment tracking for a workload.
func UpdateWorkloadDeployment(namespace, deployment string, updates func(*WorkloadDeploymentState)) error {
	st, err := GetRuntimeState()
	if err != nil {
		return err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.Deployments == nil {
		st.Deployments = make(map[string]WorkloadDeploymentState)
	}
	key := workloadKey(namespace, deployment)
	w := st.Deployments[key]
	updates(&w)
	st.Deployments[key] = w
	return persistRuntimeState(st)
}

// RecordDeploymentStart snapshots the previous running image and records the new immutable target.
// priorClusterImage is the image currently on the Deployment template before this deploy (may be empty on first deploy).
func RecordDeploymentStart(namespace, deployment, appName, containerName, registryHost, newTag, newImage, priorClusterImage string) error {
	newTag = strings.TrimSpace(newTag)
	newImage = strings.TrimSpace(newImage)
	priorClusterImage = strings.TrimSpace(priorClusterImage)
	return UpdateWorkloadDeployment(namespace, deployment, func(w *WorkloadDeploymentState) {
		if w.AppName == "" {
			w.AppName = appName
			w.Namespace = namespace
			w.Deployment = deployment
		}
		if containerName != "" {
			w.ContainerName = containerName
		}
		w.RegistryHost = registryHost

		if w.CurrentImage != "" && w.CurrentImage != newImage {
			w.PreviousImage = w.CurrentImage
			w.PreviousTag = w.CurrentTag
		} else if priorClusterImage != "" && priorClusterImage != newImage {
			w.PreviousImage = priorClusterImage
			w.PreviousTag = tagFromImageRef(priorClusterImage)
		}
		w.CurrentImage = newImage
		w.CurrentTag = newTag
		if w.PreviousImage != "" {
			w.RollbackTarget = w.PreviousImage
		}
		w.LastResult = DeploymentInProgress
		w.LastDeployedAt = time.Now()
	})
}

func tagFromImageRef(imageRef string) string {
	imageRef = strings.TrimSpace(imageRef)
	if idx := strings.LastIndex(imageRef, ":"); idx >= 0 && idx < len(imageRef)-1 {
		t := imageRef[idx+1:]
		if !strings.Contains(t, "/") {
			return t
		}
	}
	return ""
}

// MarkDeploymentSuccess records a successful rollout and appends the tag to stable history.
func MarkDeploymentSuccess(namespace, deployment, tag string) error {
	tag = strings.TrimSpace(tag)
	return UpdateWorkloadDeployment(namespace, deployment, func(w *WorkloadDeploymentState) {
		w.LastResult = DeploymentSuccess
		w.LastDeployedAt = time.Now()
		if tag == "" {
			return
		}
		w.StableHistory = appendStableTag(w.StableHistory, tag)
	})
}

// MarkDeploymentFailed marks the last attempt as failed without changing current cluster image fields.
func MarkDeploymentFailed(namespace, deployment string) error {
	return UpdateWorkloadDeployment(namespace, deployment, func(w *WorkloadDeploymentState) {
		w.LastResult = DeploymentFailed
	})
}

// MarkDeploymentRolledBack updates state after switching to a prior registry image.
func MarkDeploymentRolledBack(namespace, deployment, rolledBackImage, rolledBackTag string) error {
	return UpdateWorkloadDeployment(namespace, deployment, func(w *WorkloadDeploymentState) {
		if rolledBackImage != "" {
			w.CurrentImage = rolledBackImage
		}
		if rolledBackTag != "" {
			w.CurrentTag = rolledBackTag
			w.StableHistory = appendStableTag(w.StableHistory, rolledBackTag)
		}
		w.LastResult = DeploymentRolledBack
		w.LastDeployedAt = time.Now()
	})
}

// SetRegistryTagsForWorkload stores the latest registry catalog tags for visibility.
func SetRegistryTagsForWorkload(namespace, deployment string, tags []string) error {
	return UpdateWorkloadDeployment(namespace, deployment, func(w *WorkloadDeploymentState) {
		w.RegistryTags = append([]string(nil), tags...)
	})
}

func appendStableTag(history []string, tag string) []string {
	tag = strings.TrimSpace(tag)
	if tag == "" || tag == "latest" {
		return history
	}
	for _, t := range history {
		if t == tag {
			return history
		}
	}
	history = append(history, tag)
	if len(history) > maxStableHistoryEntries {
		history = history[len(history)-maxStableHistoryEntries:]
	}
	return history
}

// RollbackImageRef returns the image reference to use for registry-based rollback.
func RollbackImageRef(namespace, deployment string) (image string, tag string, ok bool, err error) {
	w, found, err := GetWorkloadDeployment(namespace, deployment)
	if err != nil {
		return "", "", false, err
	}
	if !found {
		return "", "", false, nil
	}
	if strings.TrimSpace(w.RollbackTarget) != "" {
		return w.RollbackTarget, w.PreviousTag, true, nil
	}
	if strings.TrimSpace(w.PreviousImage) != "" {
		return w.PreviousImage, w.PreviousTag, true, nil
	}
	if len(w.StableHistory) >= 2 {
		tag = w.StableHistory[len(w.StableHistory)-2]
		if w.RegistryHost != "" && w.AppName != "" {
			return fmt.Sprintf("%s/%s:%s", w.RegistryHost, w.AppName, tag), tag, true, nil
		}
	}
	return "", "", false, nil
}
