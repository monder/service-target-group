package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elbv2"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func main() {
	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		log.Println("Unable to initialize manager: ", err)
		os.Exit(1)
	}

	c, err := controller.New("foo-controller", mgr, controller.Options{
		Reconciler: &endpointReconciler{
			client:           mgr.GetClient(),
			managedResources: make(map[string]map[string]bool, 0),
		},
	})
	if err != nil {
		log.Println("Unable to initialize controller: ", err)
		os.Exit(1)
	}

	if err := c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForObject{}); err != nil {
		log.Println("Unable to watch services: ", err)
		os.Exit(1)
	}
	if err := c.Watch(&source.Kind{Type: &corev1.Endpoints{}}, &handler.EnqueueRequestForObject{}); err != nil {
		log.Println("Unable to watch endpoints: ", err)
		os.Exit(1)
	}

	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Println("Unable to run manager: ", err)
		os.Exit(1)
	}
}

type endpointReconciler struct {
	client           client.Client
	managedResources map[string]map[string]bool
}

func (r *endpointReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	if request.NamespacedName.Namespace != "default" { // TODO
		return reconcile.Result{}, nil
	}

	rss := &corev1.Service{}
	err := r.client.Get(context.TODO(), request.NamespacedName, rss)
	if errors.IsNotFound(err) {
		if r.managedResources[request.NamespacedName.String()] != nil {
			// TODO cleanup all
		}
		return reconcile.Result{}, nil
	}

	targetGroupARN := rss.Annotations["alb.kubernetes.io/target-group"]
	if targetGroupARN == "" { // Skip services that we do not need to register
		return reconcile.Result{}, nil
	}

	rse := &corev1.Endpoints{}
	err = r.client.Get(context.TODO(), request.NamespacedName, rse)
	if errors.IsNotFound(err) {
		// TODO cleanup all
		return reconcile.Result{}, nil
	}

	newState := make(map[string]bool, 0)

	for _, s := range rse.Subsets {
		for _, a := range s.Addresses {
			newState[a.IP] = true
		}
	}

	if reflect.DeepEqual(newState, r.managedResources[request.NamespacedName.String()]) {
		fmt.Println("equal")
		return reconcile.Result{}, nil
	}

	r.managedResources[request.NamespacedName.String()] = newState

	targetsToDeregister := make([]*elbv2.TargetDescription, 0)
	targetsToRegister := make([]*elbv2.TargetDescription, 0)

	svc := elbv2.New(session.New())
	input := &elbv2.DescribeTargetHealthInput{
		TargetGroupArn: aws.String(targetGroupARN),
	}

	result, err := svc.DescribeTargetHealth(input)
	if err != nil {
		fmt.Println(err.Error())
		return reconcile.Result{}, nil
	}

	for _, th := range result.TargetHealthDescriptions {
		_, keep := newState[*th.Target.Id]
		if !keep {
			targetsToDeregister = append(targetsToDeregister, th.Target)
		}
	}

	for ip := range newState {
		found := false
		for _, th := range result.TargetHealthDescriptions {
			if *th.Target.Id == ip {
				found = true
				break
			}
		}
		if !found {
			targetsToRegister = append(targetsToRegister, &elbv2.TargetDescription{
				Id: aws.String(ip),
			})
		}
	}

	fmt.Println("dereg:")
	fmt.Println(targetsToDeregister)
	fmt.Println("reg:")
	fmt.Println(targetsToRegister)

	// Register
	if len(targetsToRegister) > 0 {
		input2 := &elbv2.RegisterTargetsInput{
			TargetGroupArn: aws.String(targetGroupARN),
			Targets:        targetsToRegister,
		}
		_, err = svc.RegisterTargets(input2)
		fmt.Println(err)
	}

	// Deregister
	if len(targetsToDeregister) > 0 {
		input3 := &elbv2.DeregisterTargetsInput{
			TargetGroupArn: aws.String(targetGroupARN),
			Targets:        targetsToDeregister,
		}
		_, err = svc.DeregisterTargets(input3)
		fmt.Println(err)
	}

	fmt.Println("---")
	return reconcile.Result{}, nil
}
