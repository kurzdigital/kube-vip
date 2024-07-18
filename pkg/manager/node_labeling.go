package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

const (
	nodeLabelIndex    = "kube-vip.io/has-ip"
	nodeLabelJSONPath = `kube-vip.io~1has-ip`
)

type patchStringLabel struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

// EscapeJSONPointer escapes a string for use in JSON Patch paths.
func EscapeJSONPointer(s string) string {
	// Replace '~' with '~0' and '/' with '~1' according to JSON Pointer syntax.
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

// applyNodeLabel add/remove node label `kube-vip.io/is-leader=true` to/from
// the node where the virtual IP was added to/removed from.
func applyNodeLabel(clientSet *kubernetes.Clientset, label, id, identity string) {
	ctx := context.Background()
	node, err := clientSet.CoreV1().Nodes().Get(ctx, id, metav1.GetOptions{})
	if err != nil {
		log.Errorf("can't query node %s labels. error: %v", id, err)
		return
	}
	log.Debugf("node %s labels: %+v", id, node.Labels)

	_, ok := node.Labels[label]
	path := fmt.Sprintf("/metadata/labels/%s", EscapeJSONPointer(label))
	if (!ok) && id == identity {
		log.Debugf("setting node label `%s=true` on %s", label, id)
		// Append label
		applyPatchLabels(ctx, clientSet, id, "add", path, "true")
	} else if ok {
		log.Debugf("removing node label `%s=true` on %s", label, id)
		// Remove label
		applyPatchLabels(ctx, clientSet, id, "remove", path, "true")
	} else {
		log.Debugf("no node label change needed")
	}
}

// applyPatchLabels add/remove node labels
func applyPatchLabels(ctx context.Context, clientSet *kubernetes.Clientset,
	name, operation, path, value string) {
	patchLabels := []patchStringLabel{{
		Op:    operation,
		Path:  path,
		Value: value,
	}}
	patchData, err := json.Marshal(patchLabels)
	if err != nil {
		log.Errorf("node patch marshaling failed. error: %v", err)
		return
	}
	// patch node
	node, err := clientSet.CoreV1().Nodes().Patch(ctx,
		name, types.JSONPatchType, patchData, metav1.PatchOptions{})
	if err != nil {
		log.Errorf("can't patch node %s. error: %v", name, err)
		return
	}
	log.Debugf("updated node %s labels: %+v", name, node.Labels)
}
