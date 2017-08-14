k8s-crd-poc
===========

POC of a custom k8s operator that can deploy a "shipment/environment".

create the CRD
```
kubectl create -f crd-shipmentenvironments.yml
```

create an instance of the CRD
```
kubectl create -f my-shipment-env.yml
kubectl get shipmentenvironments
kubectl get shipmentenvironments -o json | jq
```

start the controller (locally)
```
go build
./k8s-crd-poc -kubeconf ~/.kube/config
```

or, via deployment
```
kubectl create -f operator.yml
```

```
found 1 Shipment Environments
add: my-shipment::dev, status =
created deployment "my-shipment-dev".
created service "my-shipment-dev".
```
```
kubectl get shipmentenvironments -o json | jq '.items[] | { shipment: .spec.name, env: .spec.environment, status: .status} '
```