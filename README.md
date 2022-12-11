# k8s-node-check

Tests if your nodes are getting slow in pod creation / termination.

k8s-node-check creates one (1) pod on every node and deletes it in a loop.

If the creation/termination takes too long, it marks the node as `PIDPressure` status/taint that is then removed by kubelet if it's healthy.

Also checks for pods that are older than their node (kubernetes bug with spot instance replacement) and force deletes them.

```console
$ k8s-node-check -create 10s -terminate 15s -every 5s -pods=10m
2022/12/10 20:00:11 PROBLEM CREATE ip-192-168-29-39.eu-north-1.compute.internal 11.438242s
2022/12/10 19:59:47 PROBLEM TERMINATING ip-192-168-75-31.eu-north-1.compute.internal 17.589388s
```

docker image at <https://github.com/matti/k8s-node-check/pkgs/container/k8s-node-check>
