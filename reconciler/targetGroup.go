package reconciler

import (
	"context"
	"fmt"
	"reflect"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elbv2"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *endpointReconciler) ReconcileTargetGroup(request reconcile.Request, targetGroupARN string) error {
	parsedARN, err := arn.Parse(targetGroupARN)
	if err != nil {
		return err
	}

	rse := &corev1.Endpoints{}
	err = r.client.Get(context.TODO(), request.NamespacedName, rse)
	if errors.IsNotFound(err) {
		delete(r.elbResources, request.NamespacedName.String())
		// TODO deregister everything?
		return nil
	}

	newState := make(map[string]*elbv2.TargetDescription, 0)

	for _, s := range rse.Subsets {
		for _, p := range s.Ports {
			for _, a := range s.Addresses {
				newState[fmt.Sprintf("%s:%d", a.IP, p.Port)] = &elbv2.TargetDescription{
					Id:   aws.String(a.IP),
					Port: aws.Int64(int64(p.Port)),
				}
			}
		}
	}

	if reflect.DeepEqual(newState, r.elbResources[request.NamespacedName.String()]) {
		return nil
	}

	targetsToDeregister := make([]*elbv2.TargetDescription, 0)
	targetsToRegister := make([]*elbv2.TargetDescription, 0)

	svc := elbv2.New(session.Must(session.NewSession(&aws.Config{
		Region: aws.String(parsedARN.Region),
	})))
	result, err := svc.DescribeTargetHealth(&elbv2.DescribeTargetHealthInput{
		TargetGroupArn: aws.String(targetGroupARN),
	})
	if err != nil {
		return err
	}

	for _, th := range result.TargetHealthDescriptions {
		_, keep := newState[fmt.Sprintf("%s:%d", *th.Target.Id, *th.Target.Port)]
		if !keep {
			targetsToDeregister = append(targetsToDeregister, th.Target)
		}
	}

	for _, td := range newState {
		found := false
		for _, th := range result.TargetHealthDescriptions {
			if *th.Target.Id == *td.Id && *th.Target.Port == *td.Port && *th.TargetHealth.State != elbv2.TargetHealthStateEnumDraining {
				found = true
				break
			}
		}
		if !found {
			targetsToRegister = append(targetsToRegister, td)
		}
	}

	log.WithFields(
		log.Fields{
			"targets": targetsToDeregister,
		},
	).Info("deregistering")

	log.WithFields(
		log.Fields{
			"targets": targetsToRegister,
		},
	).Info("registering")

	// Register
	if len(targetsToRegister) > 0 {
		_, err = svc.RegisterTargets(&elbv2.RegisterTargetsInput{
			TargetGroupArn: aws.String(targetGroupARN),
			Targets:        targetsToRegister,
		})
		if err != nil {
			log.WithFields(
				log.Fields{
					"error": err,
				},
			).Error("registering targets")
		}
	}

	// Deregister
	if len(targetsToDeregister) > 0 {
		_, err = svc.DeregisterTargets(&elbv2.DeregisterTargetsInput{
			TargetGroupArn: aws.String(targetGroupARN),
			Targets:        targetsToDeregister,
		})
		if err != nil {
			log.WithFields(
				log.Fields{
					"error": err,
				},
			).Error("deregistering targets")
		}
	}

	log.Info("finished reconciling loop")
	r.elbResources[request.NamespacedName.String()] = newState
	return nil
}
