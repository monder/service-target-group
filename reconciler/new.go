package reconciler

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/route53"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func New() Reconciler {
	return &endpointReconciler{
		elbResources: make(map[string]map[string]*elbv2.TargetDescription, 0),
		route53Resources: make(map[string]*route53.ResourceRecordSet, 0),
	}
}

func (r *endpointReconciler) SetClient(client client.Client) {
	r.client = client
}

func (r *endpointReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	rss := &corev1.Service{}
	err := r.client.Get(context.TODO(), request.NamespacedName, rss)
	if errors.IsNotFound(err) {
		delete(r.elbResources, request.NamespacedName.String())
		delete(r.route53Resources, request.NamespacedName.String())
		// TODO deregister everything?
		return reconcile.Result{}, nil
	}

	targetGroupARN := rss.Annotations["stg.monder.cc/target-group"]
	if targetGroupARN != "" {
		err = r.ReconcileTargetGroup(request, targetGroupARN)
		if err != nil {
			fmt.Println(err.Error())
		}
	}
	route53Domain := rss.Annotations["route53.monder.cc/domain-name"]
	route53Zone := rss.Annotations["route53.monder.cc/zone"]
	if route53Domain != "" && route53Zone != "" {
		err = r.ReconcileRoute53(request, route53Zone, route53Domain)
		if err != nil {
			fmt.Println(err.Error())
		}
	}
	return reconcile.Result{}, nil
}
