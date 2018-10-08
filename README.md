
![Docker Build Status](https://img.shields.io/docker/build/monder/service-target-group.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/monder/service-target-group)](https://goreportcard.com/report/github.com/monder/service-target-group)
![MicroBadger Size](https://img.shields.io/microbadger/image-size/monder/service-target-group/latest.svg)
![GitHub](https://img.shields.io/github/license/monder/service-target-group.svg)


> Kubernetes controller that registers service endpoints in AWS target group

## Summary

This project was created as an alternative to built-in [LoadBalancer](https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer) and [aws-alb-ingress-controller](https://github.com/kubernetes-sigs/aws-alb-ingress-controller). Main difference from ingress controller is that it does not create any new AWS resources. It could be handy when migrating infrastructure to kubernetes and want to reuse existing load balancers that are managed elsewhere.

## Overview

This controller assumes that you have existing ALB configured with some target groups. It also requires that your pods have routable IP addresses within the VPC. This could be achived by using [vpc-cni](https://github.com/aws/amazon-vpc-cni-k8s) plugin.

Lets have a service defined as:
```yaml
kind: Service
apiVersion: v1
metadata:
  name: foo
  annotations:
    stg.monder.cc/target-group: arn:aws:elasticloadbalancing:eu-west-1:000000000000:targetgroup/foo/bar
spec:
  clusterIP: None
  selector:
    name: foo
  ports:
  - protocol: TCP
    port: 3000
    targetPort: 3000
```
When new pod is added and its endpoint becomes `ready`, it will be added to target group provided in annotation. When pod is removed it will automatically be removed from the group.

Kubernetes:

<img width="759" alt="image 2018-10-07 at 11 23 07 am" src="https://user-images.githubusercontent.com/232147/46579958-b4c0cb00-ca23-11e8-841e-03ccd6796313.png">

AWS:

<img width="410" alt="image 2018-10-07 at 11 21 48 am" src="https://user-images.githubusercontent.com/232147/46579956-b25e7100-ca23-11e8-8b8e-72bbbf632d1f.png">


**Please note that AWS target group type must be `ip`. See more [here](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-target-groups.html#target-type)**

## Setup

Controller requires following [IAM policy](https://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies.html):
```json
{
    "Effect": "Allow",
    "Action": [
        "elasticloadbalancing:DescribeTargetHealth",
        "elasticloadbalancing:RegisterTargets",
        "elasticloadbalancing:DeregisterTargets"
    ],
    "Resource": "*"
},     
```

Controller definition:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: stg-controller
spec:
  selector:
    matchLabels:
      name: stg-controller
  replicas: 1
  template:
    metadata:
      annotations:
        iam.amazonaws.com/role: stg_controller
      labels:
        name: stg-controller
    spec:
      serviceAccountName: stg-controller
      containers:
      - name: stg-controller
        image: monder/service-target-group:latest
        env:
        - name: AWS_REGION
          value: eu-west-1
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: stg-controller
rules:
- apiGroups: [""]
  resources: ["services", "endpoints"]
  verbs: ["get", "watch", "list"]
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: stg-controller
  namespace: default
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: stg-controller
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: stg-controller
subjects:
- kind: ServiceAccount
  name: stg-controller
  namespace: default
```

## TODO

* Deregister all targets when kubernetes service is destroyed.
* Make namespace configurable
