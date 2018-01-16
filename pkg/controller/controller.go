// Package controller deletes pods in a certain state
package controller

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

// PodLister gets a list of pods.
type PodLister interface {
	ListPods(namespace string, selector string) ([]v1.Pod, error)
}

// PodDeleter deletes a pod
type PodDeleter interface {
	DeletePod(namespace string, name string) error
}

// Controller is a struct to hold a lister, deleter, and options
type Controller struct {
	lister     PodLister
	deleter    PodDeleter
	namespace  string
	selector   string
	logger     *zap.Logger
	grace      time.Duration
	interval   time.Duration
	dryRun     bool
	reasons    []string
	reasonsMap map[string]bool
	stopChan   chan struct{}
}

// DefaultReasons is the reaons to delete a pod.
// only used when containers in pod are in terminated of waiting state
var DefaultReasons = []string{
	"CrashLoopBackOff",
	"Error",
}

// Option sets options when creating a new controller
type Option func(*Controller) error

// New creates a new controller
func New(lister PodLister, deleter PodDeleter, options ...Option) (*Controller, error) {
	c := &Controller{
		lister:     lister,
		deleter:    deleter,
		grace:      time.Minute * 30,
		interval:   time.Minute * 10,
		reasons:    DefaultReasons,
		reasonsMap: make(map[string]bool),
		stopChan:   make(chan struct{}),
	}

	for _, o := range options {
		if err := o(c); err != nil {
			return nil, errors.Wrap(err, "option failed")
		}
	}

	if c.logger == nil {
		l, err := zap.NewProduction()
		if err != nil {
			return nil, errors.Wrap(err, "failed to create logger")
		}
		c.logger = l
	}

	for _, r := range c.reasons {
		c.reasonsMap[r] = true
	}

	return c, nil
}

// Once will list all pods and delete those that are in certain states
// and are at least x seconds old.
func (c *Controller) Once(ctx context.Context) error {
	pods, err := c.lister.ListPods(c.namespace, c.selector)
	if err != nil {
		return errors.Wrap(err, "failed to list pods")
	}

	for _, pod := range pods {
		// we only check at the beginning of loop if we are done
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		logger := c.logger.With(
			zap.String("namespace", pod.ObjectMeta.Namespace),
			zap.String("name", pod.ObjectMeta.Name),
		)

		switch pod.Status.Phase {
		case v1.PodPending, v1.PodSucceeded, v1.PodUnknown:
			logger.Debug("skipping pod",
				zap.String("reason", "PodPhase"),
				zap.String("PodPhase", string(pod.Status.Phase)),
			)
			continue
		}

		// only look at pods that are older than the grace period
		if pod.ObjectMeta.CreationTimestamp.Time.Add(c.grace).After(time.Now()) {
			logger.Debug("skipping pod",
				zap.String("reason", "CreationTimestamp"),
				zap.Time("CreationTimestamp", pod.ObjectMeta.CreationTimestamp.Time),
			)
			continue
		}

	STATUS:
		for _, status := range pod.Status.ContainerStatuses {
			reason := ""
			if status.State.Terminated != nil {
				reason = status.State.Terminated.Reason
			} else if status.State.Waiting != nil {
				reason = status.State.Waiting.Reason
			}

			if _, ok := c.reasonsMap[reason]; !ok {
				logger.Debug("skipping pod",
					zap.String("reason", "Reason"),
					zap.String("Reason", reason),
				)
				continue STATUS
			}

			logger.Info("deleting pod",
				zap.String("Reason", reason),
				zap.Bool("dry-run", c.dryRun),
			)

			if !c.dryRun {
				err := c.deleter.DeletePod(pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
				if err != nil {
					// if not found is fine as pod may have exited
					if !k8sErrors.IsNotFound(err) {
						return errors.Wrapf(err, "failed to delete pod %s/%s", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
					}
				}
			}
		}
	}

	return nil
}

// Loop will run the controller periodically until stopped
func (c *Controller) Loop() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := c.Once(ctx); err != nil {
		return errors.Wrap(err, "failed to run")
	}

	t := time.NewTicker(c.interval)
	for {
		select {
		case <-t.C:
			if err := c.Once(ctx); err != nil {
				return errors.Wrap(err, "failed to run")
			}
		case <-c.stopChan:
			cancel()
			return nil
		}
	}
	return nil
}

// Stop the loop
func (c *Controller) Stop() {
	// stop should only be called once, but just in case...
	select {
	case c.stopChan <- struct{}{}:
	default:
	}
}

// WithDryRun returns an Option that sets the dryrun flag.
// When true, pods will not actually be deleted
// Used when creating a new Controller.
func WithDryRun(dryrun bool) Option {
	return func(c *Controller) error {
		c.dryRun = dryrun
		return nil
	}
}

// WithLogger returns an Option that sets the logger.
// Used when creating a new Controller.
func WithLogger(l *zap.Logger) Option {
	return func(c *Controller) error {
		c.logger = l
		return nil
	}
}

// WithNamespace returns an Option that sets the namespace.
// Used when creating a new Controller.
func WithNamespace(namespace string) Option {
	return func(c *Controller) error {
		c.namespace = namespace
		return nil
	}
}

// WithSelector returns an Option that sets the label selector
// used to filter pods when listing them.
// Used when creating a new Controller.
func WithSelector(selector string) Option {
	return func(c *Controller) error {
		c.selector = selector
		return nil
	}
}

// WithGrace returns an Option that sets the grace period for pod deletions.
// Pods that have been created less than this time period ago will
// not be considered for deletion.
// Used when creating a new Controller.
func WithGrace(d time.Duration) Option {
	return func(c *Controller) error {
		c.grace = d
		return nil
	}
}

// WithInterval returns an Option that sets the loop interval.
// Used when creating a new Controller.
func WithInterval(d time.Duration) Option {
	return func(c *Controller) error {
		c.interval = d
		return nil
	}
}

// WithReasons returns an Option that sets the reasons to delete a pod.
// Default is CrashLoopBackOff Error
func WithReasons(reasons []string) Option {
	return func(c *Controller) error {
		c.reasons = reasons
		return nil
	}
}
