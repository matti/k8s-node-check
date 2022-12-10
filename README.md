# k8s-node-check

Creates one (1) pod on every node and deletes it in a loop.

If creation/termination takes long, it marks nodes as `PIDPressure` status that is removed by kubelet if it's healthy.

```console
k8s-node-check -create 10s -terminate 15s -every 5s
```

docker image at <https://github.com/matti/k8s-node-check/pkgs/container/k8s-node-check>
