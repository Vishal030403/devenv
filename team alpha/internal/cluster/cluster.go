package cluster

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"

	appconfig "devenv/teamalpha/internal/config"
	"devenv/teamalpha/internal/log"
	"devenv/teamalpha/internal/rollout"
)

const clusterCreateTimeout = 300 * time.Second
const clusterDeleteTimeout = 120 * time.Second

func CreateCluster() error {
	log.Info("Initializing cluster")

	if clusterPresent() {
		log.Info("Cluster already exists")
		useContext()
		return nil
	}

	err := rollout.Retry(func() error {
		return runCreate()
	}, 2)

	if err != nil {
		log.Failed("Cluster creation failed")
		return errors.New("cluster creation failed")
	}

	useContext()

	if err := verifyReachable(); err != nil {
		log.Failed("Cluster not reachable")
		return errors.New("cluster not reachable")
	}

	log.Success("Kubernetes cluster ready")
	return nil
}

func DeleteCluster() error {
	log.Info("Cleaning cluster")

	if !clusterPresent() {
		log.Info("No cluster found")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), clusterDeleteTimeout)
	defer cancel()

	log.Running("Deleting Kubernetes cluster")

	cmd := exec.CommandContext(
		ctx,
		"kind",
		"delete",
		"cluster",
		"--name",
		appconfig.TargetClusterName(),
	)
	if err := cmd.Run(); err != nil {
		return errors.New("failed to delete cluster")
	}

	_ = exec.Command(
		"kubectl",
		"config",
		"delete-context",
		"kind-"+appconfig.TargetClusterName(),
	).Run()

	log.Success("Kubernetes cluster deleted")
	return nil
}

func runCreate() error {
	ctx, cancel := context.WithTimeout(context.Background(), clusterCreateTimeout)
	defer cancel()

	log.Running("Creating kind cluster")

	cmd := exec.CommandContext(
		ctx,
		"kind",
		"create",
		"cluster",
		"--name",
		appconfig.TargetClusterName(),
	)

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return errors.New("cluster creation timed out")
		}
		return err
	}

	log.OK("Cluster created")
	return nil
}

func clusterPresent() bool {
	out, err := exec.Command("kind", "get", "clusters").Output()
	if err != nil {
		return false
	}
	for _, c := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(c) == appconfig.TargetClusterName() {
			return true
		}
	}
	return false
}

func useContext() {

	_ = exec.Command(
		"kubectl",
		"config",
		"use-context",
		"kind-"+appconfig.TargetClusterName(),
	).Run()
}

func verifyReachable() error {
	return exec.Command("kubectl", "cluster-info").Run()
}
