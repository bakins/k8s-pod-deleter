package controller

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type testClient struct {
	pods []v1.Pod
}

func (t *testClient) ListPods(namespace string, selector string) ([]v1.Pod, error) {
	return t.pods, nil
}

func (t *testClient) DeletePod(namespace string, name string) error {
	// cheesy
	pods := make([]v1.Pod, 0, len(t.pods))
	for _, p := range t.pods {
		if namespace == p.ObjectMeta.Namespace && name == p.ObjectMeta.Name {
			continue
		}
		pods = append(pods, p)
	}
	t.pods = pods
	return nil
}

func (t *testClient) lenPods() int {
	return len(t.pods)
}

// useful to debug test
func createLogger() *zap.Logger {
	config := zap.NewProductionConfig()
	config.Level.SetLevel(zap.DebugLevel)
	l, err := config.Build()
	if err != nil {
		panic(err)
	}
	return l
}

// create a test pod with the given reason.
func makePod(age time.Duration, namespace string, name string, phase v1.PodPhase, state string, reason string) v1.Pod {
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:         namespace,
			Name:              name,
			CreationTimestamp: metav1.Time{Time: time.Now().Add(-age)},
		},
		Status: v1.PodStatus{
			Phase: phase,
			ContainerStatuses: []v1.ContainerStatus{
				v1.ContainerStatus{},
			},
		},
	}

	switch state {
	case "Running":
		pod.Status.ContainerStatuses[0].State.Running = &v1.ContainerStateRunning{}
	case "Waiting":
		pod.Status.ContainerStatuses[0].State.Waiting = &v1.ContainerStateWaiting{
			Reason: reason,
		}
	case "Terminated":
		pod.Status.ContainerStatuses[0].State.Terminated = &v1.ContainerStateTerminated{
			Reason: reason,
		}
	}

	return pod
}

func TestController(t *testing.T) {
	tests := []struct {
		description string
		pods        []v1.Pod
		expected    int
	}{
		{
			description: "empty",
			pods:        []v1.Pod{},
			expected:    0,
		},
		{
			description: "delete none",
			pods: []v1.Pod{
				makePod(time.Hour, "default", "pod0", v1.PodRunning, "Running", ""),
			},
			expected: 1,
		},
		{
			description: "delete all",
			pods: []v1.Pod{
				makePod(time.Hour, "default", "pod0", v1.PodRunning, "Terminated", "CrashLoopBackOff"),
				makePod(time.Hour*3, "default", "pod1", v1.PodRunning, "Terminated", "CrashLoopBackOff"),
			},
			expected: 0,
		},
		{
			description: "delete one",
			pods: []v1.Pod{
				makePod(time.Minute, "default", "pod0", v1.PodRunning, "Terminated", "CrashLoopBackOff"),
				makePod(time.Hour*3, "default", "pod1", v1.PodRunning, "Terminated", "CrashLoopBackOff"),
			},
			expected: 1,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.description, func(t *testing.T) {
			t.Parallel()

			client := &testClient{}
			client.pods = test.pods

			c, err := New(client, client,
				WithGrace(time.Duration(time.Minute*5)),
				WithLogger(zap.NewNop()),
			)
			require.NoError(t, err)

			err = c.Once(context.Background())
			require.NoError(t, err)

			require.Equal(t, test.expected, client.lenPods())
		})
	}
}
