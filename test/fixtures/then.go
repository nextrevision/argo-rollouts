package fixtures

import (
	"fmt"
	"reflect"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	rov1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/argoproj/argo-rollouts/utils/defaults"
)

type Then struct {
	Common
}

type RolloutExpectation func(*rov1.Rollout) bool

func (t *Then) ExpectRollout(expectation string, expectFunc RolloutExpectation) *Then {
	ro, err := t.rolloutClient.ArgoprojV1alpha1().Rollouts(t.namespace).Get(t.rollout.GetName(), metav1.GetOptions{})
	t.CheckError(err)
	if !expectFunc(ro) {
		t.log.Errorf("Rollout expectation '%s' failed", expectation)
		t.t.FailNow()
	}
	t.log.Infof("Rollout expectation '%s' met", expectation)
	return t
}

func (t *Then) ExpectReplicaCounts(desired, current, updated, ready, available interface{}) *Then {
	ro, err := t.rolloutClient.ArgoprojV1alpha1().Rollouts(t.namespace).Get(t.rollout.GetName(), metav1.GetOptions{})
	t.CheckError(err)
	if desired != nil && desired.(int) != int(defaults.GetReplicasOrDefault(ro.Spec.Replicas)) {
		t.t.Fatalf("Expected %d desired replicas. Actual: %d", desired, defaults.GetReplicasOrDefault(ro.Spec.Replicas))
	}
	if current != nil && current.(int) != int(ro.Status.Replicas) {
		t.t.Fatalf("Expected %d current replicas. Actual: %d", current, ro.Status.Replicas)
	}
	if ready != nil && ready.(int) != int(ro.Status.ReadyReplicas) {
		t.t.Fatalf("Expected %d ready replicas. Actual: %d", ready, ro.Status.ReadyReplicas)
	}
	if updated != nil && updated.(int) != int(ro.Status.UpdatedReplicas) {
		t.t.Fatalf("Expected %d updated replicas. Actual: %d", updated, ro.Status.UpdatedReplicas)
	}
	if available != nil && available.(int) != int(ro.Status.AvailableReplicas) {
		t.t.Fatalf("Expected %d available replicas. Actual: %d", available, ro.Status.AvailableReplicas)
	}
	t.log.Infof("Replica count expectation met (desired:%v, current:%v, updated:%v, ready:%v, available:%v)", desired, current, updated, ready, available)
	return t
}

type PodExpectation func(*corev1.PodList) bool

func (t *Then) ExpectPods(expectation string, expectFunc PodExpectation) *Then {
	selector, err := metav1.LabelSelectorAsSelector(t.Rollout().Spec.Selector)
	t.CheckError(err)
	pods, err := t.kubeClient.CoreV1().Pods(t.namespace).List(metav1.ListOptions{LabelSelector: selector.String()})
	t.CheckError(err)
	if !expectFunc(pods) {
		t.log.Errorf("Pod expectation '%s' failed", expectation)
		t.t.FailNow()
	}
	t.log.Infof("Pod expectation '%s' met", expectation)
	return t
}

func (t *Then) ExpectCanaryStablePodCount(canaryCount, stableCount int) *Then {
	ro, err := t.rolloutClient.ArgoprojV1alpha1().Rollouts(t.namespace).Get(t.rollout.GetName(), metav1.GetOptions{})
	t.CheckError(err)
	return t.expectPodCountByHash("canary", ro.Status.CurrentPodHash, canaryCount).
		expectPodCountByHash("stable", ro.Status.Canary.StableRS, stableCount)
}

func (t *Then) ExpectRevisionPodCount(revision string, expectedCount int) *Then {
	rs := t.GetReplicaSetByRevision(revision)
	description := fmt.Sprintf("revision:%s", revision)
	hash := rs.Labels[rov1.DefaultRolloutUniqueLabelKey]
	return t.expectPodCountByHash(description, hash, expectedCount)
}

func (t *Then) expectPodCountByHash(description, hash string, expectedCount int) *Then {
	return t.ExpectPods(fmt.Sprintf("%s pod count == %d", description, expectedCount), func(pods *corev1.PodList) bool {
		count := 0
		for _, pod := range pods.Items {
			if pod.DeletionTimestamp != nil {
				// ignore deleting pods from the count, since it messes with the counts and will
				// disappear soon anyways.
				t.log.Debugf("ignoring deleting pod %s from expected pod count", pod.Name)
				continue
			}
			if pod.Labels[rov1.DefaultRolloutUniqueLabelKey] == hash {
				count += 1
			}
		}
		metExpectation := expectedCount == count
		if !metExpectation {
			t.log.Warnf("unexpected %s (hash %s) pod count: expected %d, saw: %d", description, hash, expectedCount, count)
		}
		return metExpectation
	})
}

type ReplicaSetExpectation func(*appsv1.ReplicaSetList) bool

func (t *Then) ExpectReplicaSets(expectation string, expectFunc ReplicaSetExpectation) *Then {
	selector, err := metav1.LabelSelectorAsSelector(t.Rollout().Spec.Selector)
	t.CheckError(err)
	replicasets, err := t.kubeClient.AppsV1().ReplicaSets(t.namespace).List(metav1.ListOptions{LabelSelector: selector.String()})
	t.CheckError(err)
	if !expectFunc(replicasets) {
		t.log.Errorf("Replicaset expectation '%s' failed", expectation)
		t.t.FailNow()
	}
	t.log.Infof("Replicaset expectation '%s' met", expectation)
	return t
}

type AnalysisRunListExpectation func(*rov1.AnalysisRunList) bool
type AnalysisRunExpectation func(*rov1.AnalysisRun) bool

func (t *Then) ExpectAnalysisRuns(expectation string, expectFunc AnalysisRunListExpectation) *Then {
	aruns := t.GetRolloutAnalysisRuns()
	if !expectFunc(&aruns) {
		t.log.Errorf("AnalysisRun expectation '%s' failed", expectation)
		t.t.FailNow()
	}
	t.log.Infof("AnalysisRun expectation '%s' met", expectation)
	return t
}

func (t *Then) ExpectAnalysisRunCount(expectedCount int) *Then {
	return t.ExpectAnalysisRuns(fmt.Sprintf("analysisrun count == %d", expectedCount), func(aruns *rov1.AnalysisRunList) bool {
		return len(aruns.Items) == expectedCount
	})
}

func (t *Then) ExpectBackgroundAnalysisRun(expectation string, expectFunc AnalysisRunExpectation) *Then {
	bgArun := t.GetBackgroundAnalysisRun()
	if !expectFunc(bgArun) {
		t.log.Errorf("Background AnalysisRun expectation '%s' failed", expectation)
		t.t.FailNow()
	}
	t.log.Infof("Background AnalysisRun expectation '%s' met", expectation)
	return t
}

func (t *Then) ExpectBackgroundAnalysisRunPhase(phase string) *Then {
	return t.ExpectBackgroundAnalysisRun(fmt.Sprintf("background analysis phase == %s", phase),
		func(run *rov1.AnalysisRun) bool {
			return string(run.Status.Phase) == phase
		},
	)
}

// ExpectPreviewRevision verifies the preview service selector is pointing to the specified revision
func (t *Then) ExpectPreviewRevision(revision string) *Then {
	return t.verifyBlueGreenSelectorRevision("preview", revision)
}

// ExpectActiveRevision verifies the active service selector is pointing to the specified revision
func (t *Then) ExpectActiveRevision(revision string) *Then {
	return t.verifyBlueGreenSelectorRevision("active", revision)
}

func (t *Then) verifyBlueGreenSelectorRevision(which string, revision string) *Then {
	verifyRevision := func() error {
		ro, err := t.rolloutClient.ArgoprojV1alpha1().Rollouts(t.namespace).Get(t.rollout.GetName(), metav1.GetOptions{})
		t.CheckError(err)
		var serviceName, selector string
		switch which {
		case "active":
			serviceName = ro.Spec.Strategy.BlueGreen.ActiveService
			selector = ro.Status.BlueGreen.ActiveSelector
		case "preview":
			serviceName = ro.Spec.Strategy.BlueGreen.PreviewService
			selector = ro.Status.BlueGreen.PreviewSelector
		default:
			panic(fmt.Sprintf("unknown selector: %s", which))
		}
		svc, err := t.kubeClient.CoreV1().Services(t.namespace).Get(serviceName, metav1.GetOptions{})
		t.CheckError(err)
		rs := t.GetReplicaSetByRevision(revision)
		if selector != svc.Spec.Selector[rov1.DefaultRolloutUniqueLabelKey] {
			return fmt.Errorf("Expectation failed: blueGreen %s selector/service selector mismatch %s != %s", which, selector, svc.Spec.Selector[rov1.DefaultRolloutUniqueLabelKey])
		}
		if selector != rs.Labels[rov1.DefaultRolloutUniqueLabelKey] {
			return fmt.Errorf("Expectation failed: blueGreen %s selector/replicaset label mismatch %s != %s", which, selector, rs.Labels[rov1.DefaultRolloutUniqueLabelKey])
		}
		return nil
	}
	// we perform several checks because switching the active service selector lags behind the Degraded/Promotion event
	var err error
	deadline := time.Now().Add(3 * time.Second)
	for {
		err = verifyRevision()
		if err == nil {
			t.log.Infof("Expectation: %s revision == '%s' met", which, revision)
			return t
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.log.Error(err)
	t.t.FailNow()
	return t
}

func (t *Then) ExpectServiceSelector(service string, selector map[string]string) *Then {
	svc, err := t.kubeClient.CoreV1().Services(t.namespace).Get(service, metav1.GetOptions{})
	t.CheckError(err)
	if !reflect.DeepEqual(svc.Spec.Selector, selector) {
		t.t.Fatalf("Expected %s selector: %v. Actual: %v", service, selector, svc.Spec.Selector)
	}
	t.log.Infof("Expectation %s selector: %v met", service, selector)
	return t
}

func (t *Then) When() *When {
	return &When{
		Common: t.Common,
	}
}
