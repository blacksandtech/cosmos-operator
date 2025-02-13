package fullnode

import (
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	configChecksumAnnotation = "cosmos.strange.love/config-checksum"
)

// BuildPods creates the final state of pods given the crd.
func BuildPods(crd *cosmosv1.CosmosFullNode, cksums ConfigChecksums) ([]diff.Resource[*corev1.Pod], error) {
	var (
		builder   = NewPodBuilder(crd)
		overrides = crd.Spec.InstanceOverrides
		pods      []diff.Resource[*corev1.Pod]
	)
	candidates := podCandidates(crd)
	for i := int32(0); i < crd.Spec.Replicas; i++ {
		pod, err := builder.WithOrdinal(i).Build()
		if err != nil {
			return nil, err
		}
		if _, shouldSnapshot := candidates[pod.Name]; shouldSnapshot {
			continue
		}
		if len(crd.Spec.ChainSpec.Versions) > 0 {
			instanceHeight := uint64(0)
			if height, ok := crd.Status.Height[pod.Name]; ok {
				instanceHeight = height
			}
			var image string
			for _, version := range crd.Spec.ChainSpec.Versions {
				if instanceHeight < version.UpgradeHeight {
					break
				}
				image = version.Image
			}
			if image != "" {
				setChainContainerImage(pod, image)
			}
		}
		if o, ok := overrides[pod.Name]; ok {
			if o.DisableStrategy != nil {
				continue
			}
			if o.Image != "" {
				setChainContainerImage(pod, o.Image)
			}
		}
		pod.Annotations[configChecksumAnnotation] = cksums[client.ObjectKeyFromObject(pod)]
		pods = append(pods, diff.Adapt(pod, i))
	}
	return pods, nil
}

func setChainContainerImage(pod *corev1.Pod, image string) {
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == mainContainer {
			pod.Spec.Containers[i].Image = image
			break
		}
	}
	for i := range pod.Spec.InitContainers {
		if pod.Spec.InitContainers[i].Name == chainInitContainer {
			pod.Spec.InitContainers[i].Image = image
			break
		}
	}
}

func podCandidates(crd *cosmosv1.CosmosFullNode) map[string]struct{} {
	candidates := make(map[string]struct{})
	for _, v := range crd.Status.ScheduledSnapshotStatus {
		candidates[v.PodCandidate] = struct{}{}
	}
	return candidates
}
