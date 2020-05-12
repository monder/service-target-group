package main

import (
	"os"

	"github.com/monder/service-target-group/reconciler"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

func main() {
	r := reconciler.New()

	cfg, err := config.GetConfig()
	assertError(err, "getting kubeconfig")

	manager, err := manager.New(cfg, manager.Options{})
	assertError(err, "configuring manager")

	r.SetClient(manager.GetClient())

	_, err = builder.ControllerManagedBy(manager).
		ForType(&corev1.Service{}).
		ForType(&corev1.Endpoints{}).
		Build(r)
	assertError(err, "building controller")

	defer log.Info("exiting..")

	err = manager.Start(signals.SetupSignalHandler())
	assertError(err, "starting manager")
}

func assertError(err error, description string) {
	if err != nil {
		log.WithFields(
			log.Fields{
				"error": err,
			},
		).Fatal(description)
		os.Exit(1)
	}
}
