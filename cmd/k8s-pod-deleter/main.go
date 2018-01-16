package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bakins/k8s-pod-deleter/pkg/controller"
	"github.com/bakins/k8s-pod-deleter/pkg/k8s"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	// load auth methods
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

type mainCommand struct {
	kubeconfig  string
	kubeContext string
	namespace   string
	selector    string
	logLevel    logLevel
	reasons     []string
	dryRun      bool
	once        bool
	grace       time.Duration
	interval    time.Duration
}

func main() {
	m := &mainCommand{}

	var cmd = &cobra.Command{
		Use:           "k8s-pod-deleter",
		Short:         "delete pods in certain states",
		RunE:          m.runDeleter,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	f := cmd.Flags()
	f.StringVar(&m.kubeconfig, "kubeconfig", "", "Kubernetes client config. If not specified, an in-cluster client is tried.")
	f.StringVar(&m.kubeContext, "context", "", "Kubernetes client context. Only used if kubeconfig is specified. Defaults to value in Kubernetes config file")
	f.StringVar(&m.namespace, "namespace", "", "only consider pods in this namespace. Default is all namespaces")
	f.StringVar(&m.selector, "selector", "", "only consider pods that match this label selector. Default is all pods")
	f.BoolVar(&m.once, "once", false, "run controller loop once and exit")
	f.BoolVar(&m.dryRun, "dry-run", false, "run controller but do not delete pods")
	f.StringSliceVar(&m.reasons, "reasons", controller.DefaultReasons, "reasons to delete pod. exact match only. May be passed multiple times for multiple reasons")
	f.DurationVar(&m.grace, "grace-period", time.Hour, "pods that were created less than this time ago are not considered for deletion")
	f.DurationVar(&m.interval, "interval", time.Minute*5, "how often to run controller loop")
	levelFlag(f, &m.logLevel, "log-level", zapcore.InfoLevel, "log level")

	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (m *mainCommand) runDeleter(cmd *cobra.Command, args []string) error {

	client, err := k8s.New(m.kubeconfig, m.kubeContext)

	if err != nil {
		return errors.Wrap(err, "failed to create Kubernetes client")
	}

	logger, err := createLogger(m.logLevel.Level)
	if err != nil {
		return errors.Wrap(err, "failed to create logger")
	}

	c, err := controller.New(client, client,
		controller.WithNamespace(m.namespace),
		controller.WithSelector(m.selector),
		controller.WithLogger(logger),
		controller.WithDryRun(m.dryRun),
		controller.WithGrace(m.grace),
		controller.WithInterval(m.interval),
	)

	if err != nil {
		return errors.Wrap(err, "failed to create controller")
	}

	if m.once {
		return c.Once(context.Background())
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		c.Stop()
	}()

	return c.Loop()
}

type logLevel struct {
	zapcore.Level
}

func (l *logLevel) Type() string {
	return "string"
}

func levelFlag(f *pflag.FlagSet, l *logLevel, name string, defaultLevel zapcore.Level, usage string) {
	l.Level = defaultLevel
	f.Var(l, name, usage)
}

func createLogger(level zapcore.Level) (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	config.Level.SetLevel(level)
	return config.Build()
}
