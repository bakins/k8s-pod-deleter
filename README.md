# k8s-pod-deleter

`k8s-pod-deleter` deletes pods that are in certain states: CrashLoopBackOff, etc.

We have pods that for some reason get "stuck" and can only be "fixed" by being
deleted. This causes nuisance alerts. This automates deleting them.  We gather
all the Kubernetes events (using [eventrouter](https://github.com/heptiolabs/eventrouter)) and logs, so we generally
do not need to examine the failing pod(s). If we need to examine them, we can
stop `k8s-pod-deleter`.

**work in progress**

## Build

Tested with Go 1.9.2:

```shell
go build -o k8s-pod-deleter ./cmd/k8s-pod-deleter
```

## Usage

```shell
$ ./k8s-pod-deleter

Usage:
  k8s-pod-deleter [flags]

Flags:
      --context string          Kubernetes client context. Only used if kubeconfig is specified. Defaults to value in Kubernetes config file
      --dry-run                 run controller but do not delete pods
      --grace-period duration   pods that were created less than this time ago are not considered for deletion (default 1h0m0s)
  -h, --help                    help for k8s-pod-deleter
      --interval duration       how often to run controller loop (default 5m0s)
      --kubeconfig string       Kubernetes client config. If not specified, an in-cluster client is tried.
      --log-level string        log level (default "info")
      --namespace string        only consider pods in this namespace. Default is all namespaces
      --once                    run controller loop once and exit
      --reasons stringSlice     reasons to delete pod. exact match only. May be passed multiple times for multiple reasons (default [CrashLoopBackOff,Error])
      --selector string         only consider pods that match this label selector. Default is all pods
```