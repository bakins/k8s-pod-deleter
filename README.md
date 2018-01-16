# k8s-pod-deleter

`k8s-pod-deleter` deletes pods that are in certain states: CrashLoopBackOff, etc.

We have pods that for some reason get "stuck" and can only be "fixed" by being
deleted. This causes nuisance alerts. This automates deleting them.  We gather
all the Kubernetes events (using [eventrouter](https://github.com/heptiolabs/eventrouter)) and logs, so we generally
do not need to examine the failing pod(s). If we need to examine them, we can
stop `k8s-pod-deleter`.

**work in progress**