# token-resource

A Kubernetes [device-plugin](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/)
without HW. It just handles a number of "tokens" that containers can claim.
It is usable for:

* As a device-plugin example/skeleton, as simple as they get (one file ~250 lines)
* To limit the number of PODs per node for a `Deployment` (if they are not too many)

The `token-resource` is started on K8s nodes and creates a number of
"tokens" specified by the `-count` parameter. PODs can then claim a
token by:

```yaml
        resources:
          limits:
            example.com/token: 1
```

To deploy the `token-resource` device-plugin as a `DaemonSet`, first
edit the `token-resource.yaml` manifest to your liking, and deploy with:

```
kubectl create namespace token-resource
kubectl create -n token-resource -f token-resource.yaml
```

## Build

```
# Binary
make binary
# Image
__tag=docker.io/uablrek/token-resource:latest make image
```

## Test

In a cluster with less than 5 nodes:

```
kubectl create namespace token-resource
kubectl create -n token-resource -f token-resource.yaml
kubectl create -n token-resource -f token-resource-test.yaml
kubectl get pods -n token-resource -o wide
#kubectl delete namespace token-resource
```

Max 2 PODs shall be started on each worker node, and some should be
"Pending". Example from KinD with two workers:

```
# kubectl get node default-worker -o json | jq .status.allocatable
{
  "cpu": "24",
  "ephemeral-storage": "383374592Ki",
  "example.com/token": "2",
  "hugepages-1Gi": "0",
  "hugepages-2Mi": "0",
  "memory": "65570368Ki",
  "pods": "110"
}
# kubectl get pods -n token-resource -o wide
NAME                                 READY   STATUS    RESTARTS   AGE   IP           NODE              NOMINATED NODE   READINESS GATES
token-resource-lfsbm                 1/1     Running   0          53s   10.244.1.2   default-worker2   <none>           <none>
token-resource-qc7h4                 1/1     Running   0          53s   10.244.2.2   default-worker    <none>           <none>
token-resource-test-b6655b8c-27fr9   0/1     Pending   0          49s   <none>       <none>            <none>           <none>
token-resource-test-b6655b8c-4r722   0/1     Pending   0          49s   <none>       <none>            <none>           <none>
token-resource-test-b6655b8c-67vnp   1/1     Running   0          49s   10.244.1.3   default-worker2   <none>           <none>
token-resource-test-b6655b8c-6cz72   0/1     Pending   0          49s   <none>       <none>            <none>           <none>
token-resource-test-b6655b8c-ctk6z   0/1     Pending   0          49s   <none>       <none>            <none>           <none>
token-resource-test-b6655b8c-fxw2h   1/1     Running   0          49s   10.244.1.4   default-worker2   <none>           <none>
token-resource-test-b6655b8c-g2zgx   0/1     Pending   0          49s   <none>       <none>            <none>           <none>
token-resource-test-b6655b8c-lnvzw   0/1     Pending   0          49s   <none>       <none>            <none>           <none>
token-resource-test-b6655b8c-qcqtf   1/1     Running   0          49s   10.244.2.4   default-worker    <none>           <none>
token-resource-test-b6655b8c-vrfdz   1/1     Running   0          49s   10.244.2.3   default-worker    <none>           <none>
```
