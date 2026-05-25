package validation

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"devenv/teamalpha/internal/log"
)

type policyList struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			Ingress []json.RawMessage `json:"ingress"`
			Egress  []json.RawMessage `json:"egress"`
		} `json:"spec"`
	} `json:"items"`
}

func ValidateNetworkPolicies(namespace string) error {
	log.Running(fmt.Sprintf("Validating NetworkPolicies in %s", namespace))

	out, err := exec.Command("kubectl", "get", "networkpolicy", "-n", namespace, "-o", "json").Output()
	if err != nil {
		log.Warning("Unable to list NetworkPolicies")
		return nil
	}

	var list policyList
	if err := json.Unmarshal(out, &list); err != nil {
		log.Warning("NetworkPolicy output unparseable")
		return nil
	}

	if len(list.Items) == 0 {
		log.Warning(fmt.Sprintf("No NetworkPolicy in %s", namespace))
		log.Hint("Consider adding a default-deny policy")
		return nil
	}

	complete := 0
	for _, p := range list.Items {
		if len(p.Spec.Ingress) > 0 && len(p.Spec.Egress) > 0 {
			complete++
		}
	}

	log.Info(fmt.Sprintf("%d policy(ies), %d with ingress+egress", len(list.Items), complete))
	if complete == 0 {
		log.Warning("No policy covers both ingress and egress")
	} else {
		log.OK("NetworkPolicy validation passed")
	}

	return nil
}
