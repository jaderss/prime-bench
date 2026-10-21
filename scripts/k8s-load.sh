#!/usr/bin/env bash
set -euo pipefail

k() { kubectl --context docker-desktop "$@"; }

if [ "$(k get nodes -o jsonpath='{.items[0].metadata.name}')" = "docker-desktop" ]; then
  echo "kubeadm node shares the Docker image store, nothing to load"
  exit 0
fi

case "$(k get nodes -o jsonpath='{.items[0].status.nodeInfo.architecture}')" in
  arm64) libs=/host/usr/lib/aarch64-linux-gnu loader=ld-linux-aarch64.so.1 ;;
  amd64) libs=/host/usr/lib/x86_64-linux-gnu loader=ld-linux-x86-64.so.2 ;;
  *) echo "unsupported node architecture" >&2; exit 1 ;;
esac
node_run=("$libs/$loader" --library-path "$libs")

k apply -f - >/dev/null <<YAML
apiVersion: v1
kind: Pod
metadata:
  name: image-loader
  namespace: prime-bench
spec:
  restartPolicy: Never
  containers:
    - name: loader
      image: registry.k8s.io/pause:3.10
      imagePullPolicy: IfNotPresent
      command: ["$libs/$loader", "--library-path", "$libs", "/host/usr/bin/sleep", "3600"]
      securityContext: {runAsUser: 0}
      volumeMounts:
        - {name: node, mountPath: /host}
  volumes:
    - name: node
      hostPath: {path: /, type: Directory}
YAML
trap 'k -n prime-bench delete pod image-loader --wait=false --grace-period=0 >/dev/null 2>&1' EXIT
k -n prime-bench wait --for=condition=Ready pod/image-loader --timeout=60s >/dev/null

for image in "$@"; do
  echo "loading $image into the kind node"
  docker save "$image" | k -n prime-bench exec -i image-loader -- "${node_run[@]}" /host/usr/local/bin/ctr \
    --address /host/run/containerd/containerd.sock -n k8s.io images import --digests - >/dev/null
done
