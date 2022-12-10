# k8s-node-check

Tests if your nodes are getting slow in pod creation / termination.

k8s-node-check creates one (1) pod on every node and deletes it in a loop.

If the creation/termination takes too long, it marks the node as `PIDPressure` status/taint that is then removed by kubelet if it's healthy.

```console
k8s-node-check -create 10s -terminate 15s -every 5s
```

docker image at <https://github.com/matti/k8s-node-check/pkgs/container/k8s-node-check>
