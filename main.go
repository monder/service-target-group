package main

import (
	"log"
	"os"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"

	"github.com/monder/service-target-group/reconciler"
)

func main() {
	r := reconciler.New()

	manager, err := builder.SimpleController().
		ForType(&corev1.Service{}).
		ForType(&corev1.Endpoints{}).
		Build(r)

	if err != nil {
		log.Println("Unable to build controller:", err)
		os.Exit(1)
	}
	r.SetClient(manager.GetClient())

	if err := manager.Start(signals.SetupSignalHandler()); err != nil {
		log.Println("Unable to run controller:", err)
		os.Exit(1)
	}

}
