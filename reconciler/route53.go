package reconciler

import (
	"context"
	"reflect"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *endpointReconciler) ReconcileRoute53(request reconcile.Request, zone string, domain string) error {

	rse := &corev1.Endpoints{}
	err := r.client.Get(context.TODO(), request.NamespacedName, rse)
	if errors.IsNotFound(err) {
		delete(r.route53Resources, request.NamespacedName.String())
		// TODO deregister everything?
		return nil
	}

	newRecordSet := &route53.ResourceRecordSet{
		Name:            aws.String(domain),
		Type:            aws.String(route53.RRTypeA),
		TTL:             aws.Int64(1),
		ResourceRecords: []*route53.ResourceRecord{},
	}

	for _, s := range rse.Subsets {
		for _, a := range s.Addresses {
			newRecordSet.ResourceRecords = append(newRecordSet.ResourceRecords, &route53.ResourceRecord{
				Value: aws.String(a.IP),
			})
		}
	}

	if reflect.DeepEqual(newRecordSet, r.route53Resources[request.NamespacedName.String()]) {
		return nil
	}

	log.WithFields(
		log.Fields{
			"route53-domain": domain,
		},
	).Info("updating route53 domain")

	svc := route53.New(session.Must(session.NewSession(&aws.Config{})))
	_, err = svc.ChangeResourceRecordSets(&route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action:            aws.String(route53.ChangeActionUpsert),
					ResourceRecordSet: newRecordSet,
				},
			},
		},
		HostedZoneId: aws.String(zone),
	})

	if err != nil {
		log.WithFields(
			log.Fields{
				"route53-domain": domain,
				"error":          err,
			},
		).Error("updating route53 domain")
	}

	r.route53Resources[request.NamespacedName.String()] = newRecordSet
	return nil
}
