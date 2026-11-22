// Copyright 2026 Chaos Mesh Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package v1alpha1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// These specs run the CEL rules from the CRD schemas against the envtest API server,
// without the validating webhook.
var _ = Describe("CRD validation rules", func() {
	const (
		containerKillMessage = "the name of container is required on container-kill action"
		blockDelayMessage    = "delay should be set on delay action"
		externalTargetsMsg   = "external targets cannot be used with `from` and `both` direction in netem action yet"
		emptyTargetMessage   = "`from` and `both` direction cannot be used when targets is empty in netem action"
		ratePatternMessage   = "spec.bandwidth.rate in body should match"
	)

	meta := func(name string) metav1.ObjectMeta {
		return metav1.ObjectMeta{Name: name, Namespace: "default"}
	}

	containerKill := func(names ...string) *PodChaosSpec {
		return &PodChaosSpec{
			Action: ContainerKillAction,
			ContainerSelector: ContainerSelector{
				PodSelector:    PodSelector{Mode: OneMode},
				ContainerNames: names,
			},
		}
	}

	blockChaos := func(delay *BlockDelaySpec) *BlockChaosSpec {
		return &BlockChaosSpec{
			Action: BlockDelay,
			Delay:  delay,
			ContainerNodeVolumePathSelector: ContainerNodeVolumePathSelector{
				ContainerSelector: ContainerSelector{PodSelector: PodSelector{Mode: OneMode}},
				VolumeName:        "data",
			},
		}
	}

	networkChaos := func(action NetworkChaosAction, direction Direction, target *PodSelector, externalTargets ...string) *NetworkChaosSpec {
		return &NetworkChaosSpec{
			PodSelector:     PodSelector{Mode: OneMode},
			Action:          action,
			TcParameter:     TcParameter{Delay: &DelaySpec{Latency: "10ms"}},
			Direction:       direction,
			Target:          target,
			ExternalTargets: externalTargets,
		}
	}

	target := &PodSelector{Mode: AllMode}

	bandwidth := func(rate string) *NetworkChaosSpec {
		return &NetworkChaosSpec{
			PodSelector: PodSelector{Mode: OneMode},
			Action:      BandwidthAction,
			TcParameter: TcParameter{Bandwidth: &BandwidthSpec{Rate: rate, Limit: 1, Buffer: 1}},
		}
	}

	expectRejected := func(obj client.Object, message string) {
		err := k8sClient.Create(context.TODO(), obj)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(message))
	}

	expectAccepted := func(obj client.Object) {
		Expect(k8sClient.Create(context.TODO(), obj)).To(Succeed())
		Expect(k8sClient.Delete(context.TODO(), obj)).To(Succeed())
	}

	Context("PodChaos", func() {
		It("rejects container-kill without container names", func() {
			expectRejected(&PodChaos{ObjectMeta: meta("cel-container-kill-empty"), Spec: *containerKill()}, containerKillMessage)
		})

		It("accepts container-kill with container names", func() {
			expectAccepted(&PodChaos{ObjectMeta: meta("cel-container-kill"), Spec: *containerKill("app")})
		})

		It("accepts pod-kill without container names", func() {
			spec := containerKill()
			spec.Action = PodKillAction
			expectAccepted(&PodChaos{ObjectMeta: meta("cel-pod-kill"), Spec: *spec})
		})
	})

	Context("BlockChaos", func() {
		It("rejects the delay action without a delay spec", func() {
			expectRejected(&BlockChaos{ObjectMeta: meta("cel-block-no-delay"), Spec: *blockChaos(nil)}, blockDelayMessage)
		})

		It("accepts the delay action with a delay spec", func() {
			expectAccepted(&BlockChaos{ObjectMeta: meta("cel-block-delay"), Spec: *blockChaos(&BlockDelaySpec{Latency: "10ms"})})
		})
	})

	Context("NetworkChaos", func() {
		It("rejects the from direction without a target", func() {
			expectRejected(&NetworkChaos{ObjectMeta: meta("cel-net-from-no-target"), Spec: *networkChaos(DelayAction, From, nil)}, emptyTargetMessage)
		})

		It("rejects the both direction with external targets", func() {
			expectRejected(&NetworkChaos{ObjectMeta: meta("cel-net-both-external"), Spec: *networkChaos(DelayAction, Both, target, "8.8.8.8")}, externalTargetsMsg)
		})

		It("rejects the both direction with an empty external targets list", func() {
			// The typed client drops an empty list, so send it as unstructured like kubectl would.
			obj := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": SchemeGroupVersion.String(),
				"kind":       "NetworkChaos",
				"metadata":   map[string]any{"name": "cel-net-both-empty-external", "namespace": "default"},
				"spec": map[string]any{
					"mode":            string(OneMode),
					"selector":        map[string]any{},
					"action":          string(DelayAction),
					"delay":           map[string]any{"latency": "10ms"},
					"direction":       string(Both),
					"target":          map[string]any{"mode": string(AllMode), "selector": map[string]any{}},
					"externalTargets": []any{},
				},
			}}
			expectRejected(obj, externalTargetsMsg)
		})

		It("accepts the both direction with a target", func() {
			expectAccepted(&NetworkChaos{ObjectMeta: meta("cel-net-both-target"), Spec: *networkChaos(DelayAction, Both, target)})
		})

		It("accepts the to direction with external targets and no target", func() {
			expectAccepted(&NetworkChaos{ObjectMeta: meta("cel-net-to-external"), Spec: *networkChaos(DelayAction, To, nil, "8.8.8.8")})
		})

		It("accepts an absent direction with external targets and no target", func() {
			expectAccepted(&NetworkChaos{ObjectMeta: meta("cel-net-default-direction"), Spec: *networkChaos(DelayAction, "", nil, "8.8.8.8")})
		})

		It("accepts partition with the both direction and no target", func() {
			expectAccepted(&NetworkChaos{ObjectMeta: meta("cel-net-partition-both"), Spec: *networkChaos(PartitionAction, Both, nil)})
		})

		It("rejects bandwidth rates with an unknown unit or a sign", func() {
			expectRejected(&NetworkChaos{ObjectMeta: meta("cel-net-rate-unit"), Spec: *bandwidth("10mbs")}, ratePatternMessage)
			expectRejected(&NetworkChaos{ObjectMeta: meta("cel-net-rate-sign"), Spec: *bandwidth("-1mbit")}, ratePatternMessage)
			expectRejected(&NetworkChaos{ObjectMeta: meta("cel-net-rate-empty"), Spec: *bandwidth("mbit")}, ratePatternMessage)
		})

		It("accepts bandwidth rates the webhook accepts", func() {
			expectAccepted(&NetworkChaos{ObjectMeta: meta("cel-net-rate"), Spec: *bandwidth("1mbps")})
			expectAccepted(&NetworkChaos{ObjectMeta: meta("cel-net-rate-spaces"), Spec: *bandwidth(" 10 MBit ")})
		})
	})

	Context("Schedule", func() {
		schedule := func(name string, embed EmbedChaos, scheduleType ScheduleTemplateType) *Schedule {
			return &Schedule{
				ObjectMeta: meta(name),
				Spec: ScheduleSpec{
					Schedule:          "* * * * 1",
					ScheduleItem:      ScheduleItem{EmbedChaos: embed},
					Type:              scheduleType,
					ConcurrencyPolicy: ForbidConcurrent,
				},
			}
		}

		It("rejects an embedded container-kill without container names", func() {
			expectRejected(schedule("cel-schedule-container-kill", EmbedChaos{PodChaos: containerKill()}, ScheduleTypePodChaos), containerKillMessage)
		})

		It("rejects an embedded delay action without a delay spec", func() {
			expectRejected(schedule("cel-schedule-block", EmbedChaos{BlockChaos: blockChaos(nil)}, ScheduleTypeBlockChaos), blockDelayMessage)
		})

		It("rejects an embedded from direction without a target", func() {
			expectRejected(schedule("cel-schedule-net", EmbedChaos{NetworkChaos: networkChaos(DelayAction, From, nil)}, ScheduleTypeNetworkChaos), emptyTargetMessage)
		})

		It("accepts an embedded partition with the both direction and no target", func() {
			expectAccepted(schedule("cel-schedule-net-ok", EmbedChaos{NetworkChaos: networkChaos(PartitionAction, Both, nil)}, ScheduleTypeNetworkChaos))
		})

		It("accepts a valid embedded container-kill", func() {
			expectAccepted(schedule("cel-schedule-ok", EmbedChaos{PodChaos: containerKill("app")}, ScheduleTypePodChaos))
		})
	})

	Context("Workflow", func() {
		workflow := func(name string, embed EmbedChaos, templateType TemplateType) *Workflow {
			return &Workflow{
				ObjectMeta: meta(name),
				Spec: WorkflowSpec{
					Entry: "entry",
					Templates: []Template{{
						Name:       "entry",
						Type:       templateType,
						EmbedChaos: &embed,
					}},
				},
			}
		}

		It("rejects an embedded container-kill without container names", func() {
			expectRejected(workflow("cel-workflow-container-kill", EmbedChaos{PodChaos: containerKill()}, TypePodChaos), containerKillMessage)
		})

		It("rejects an embedded delay action without a delay spec", func() {
			expectRejected(workflow("cel-workflow-block", EmbedChaos{BlockChaos: blockChaos(nil)}, TypeBlockChaos), blockDelayMessage)
		})

		It("rejects an embedded both direction with external targets", func() {
			expectRejected(workflow("cel-workflow-net", EmbedChaos{NetworkChaos: networkChaos(DelayAction, Both, target, "8.8.8.8")}, TypeNetworkChaos), externalTargetsMsg)
		})

		It("accepts a valid embedded container-kill", func() {
			expectAccepted(workflow("cel-workflow-ok", EmbedChaos{PodChaos: containerKill("app")}, TypePodChaos))
		})
	})
})
