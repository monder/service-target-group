package reconciler

import (
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/route53"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Reconciler interface {
	Reconcile(reconcile.Request) (reconcile.Result, error)
	SetClient(client.Client)
}

type endpointReconciler struct {
	client           client.Client
	elbResources     map[string]map[string]*elbv2.TargetDescription
	route53Resources map[string]*route53.ResourceRecordSet
}
