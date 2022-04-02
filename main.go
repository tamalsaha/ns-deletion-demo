package main

import (
	"context"
	"fmt"
	"os"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2/klogr"
	"kmodules.xyz/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func NewClient() (client.Client, error) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)

	ctrl.SetLogger(klogr.New())
	cfg := ctrl.GetConfigOrDie()
	cfg.QPS = 100
	cfg.Burst = 100

	mapper, err := apiutil.NewDynamicRESTMapper(cfg)
	if err != nil {
		return nil, err
	}

	return client.New(cfg, client.Options{
		Scheme: scheme,
		Mapper: mapper,
		//Opts: client.WarningHandlerOptions{
		//	SuppressWarnings:   false,
		//	AllowDuplicateLogs: false,
		//},
	})
}

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	kc, err := NewClient()
	if err != nil {
		return err
	}

	mapper := discovery.NewResourceMapper(kc.RESTMapper())

	var ul unstructured.UnstructuredList
	ul.SetAPIVersion("tamal.com/v1")
	ul.SetKind("Pod")
	err = kc.List(context.TODO(), &ul)
	if err != nil {
		// meta.IsNoMatchError(err)
		// runtime.IsMissingKind()
		// runtime.IsNotRegisteredError(err)
		fmt.Println(err, meta.IsNoMatchError(err), runtime.IsNotRegisteredError(err), runtime.IsMissingKind(err), runtime.IsMissingVersion(err))
		return err
	}

	os.Exit(1)

	var ns core.Namespace
	err = kc.Get(context.TODO(), client.ObjectKey{Name: "kubedb"}, &ns)
	if err != nil {
		return err
	}

	ref := metav1.NewControllerRef(&ns, schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Namespace",
	})

	apiservices := []string{
		"v1alpha1.mutators.autoscaling.kubedb.com",
		"v1alpha1.mutators.dashboard.kubedb.com",
		"v1alpha1.mutators.kubedb.com",
		"v1alpha1.mutators.ops.kubedb.com",
		"v1alpha1.mutators.schema.kubedb.com",
		"v1alpha1.validators.dashboard.kubedb.com",
		"v1alpha1.validators.kubedb.com",
		"v1alpha1.validators.ops.kubedb.com",
		"v1alpha1.validators.schema.kubedb.com",
	}
	for _, name := range apiservices {
		var apsvc unstructured.Unstructured
		apsvc.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "apiregistration.k8s.io",
			Version: "v1",
			Kind:    "APIService",
		})
		err = kc.Get(context.TODO(), client.ObjectKey{Name: name}, &apsvc)
		if err != nil {
			return err
		}
		EnsureOwnerReference(&apsvc, ref)
		err = kc.Update(context.TODO(), &apsvc)
		if err != nil {
			return err
		}
	}
	return nil
}

func EnsureOwnerReference(dependent metav1.Object, owner *metav1.OwnerReference) {
	if owner == nil {
		return
	}

	refs := dependent.GetOwnerReferences()

	fi := -1
	for i := range refs {
		if refs[i].UID == owner.UID {
			fi = i
			break
		}
	}
	if fi == -1 {
		refs = append(refs, *owner)
	} else {
		refs[fi] = *owner
	}

	dependent.SetOwnerReferences(refs)
}
