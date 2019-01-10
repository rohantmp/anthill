package glustercluster

import (
	"context"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1alpha1 "github.com/gluster/anthill/pkg/apis/operator/v1alpha1"

	v0 "github.com/gluster/anthill/pkg/controller/glustercluster/v0"
	"github.com/gluster/anthill/pkg/reconciler"
)

var (
	log                                         = logf.Log.WithName("controller_glustercluster")
	allProcedures      reconciler.ProcedureList = []reconciler.Procedure{*v0.V0Procedure}
	reconcileProcedure *reconciler.Procedure
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new GlusterCluster Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileGlusterCluster{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("glustercluster-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource GlusterCluster
	err = c.Watch(&source.Kind{Type: &operatorv1alpha1.GlusterCluster{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner GlusterCluster
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &operatorv1alpha1.GlusterCluster{},
	})
	return err
}

var _ reconcile.Reconciler = &ReconcileGlusterCluster{}

// ReconcileGlusterCluster reconciles a GlusterCluster object
type ReconcileGlusterCluster struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a GlusterCluster object and makes changes based on the state read
// and what is in the GlusterCluster.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.

func (r *ReconcileGlusterCluster) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling GlusterCluster")

	// Fetch the GlusterCluster instance
	instance := &operatorv1alpha1.GlusterCluster{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Get current reconcile version from CR
	ver, ok := instance.Spec.Options["reconcileVersion"]
	if ok {
		// choose the highest compatible version
		version, _ := strconv.Atoi(ver)
		reconcileProcedure, _ = allProcedures.NewestCompatible(version)
	} else {
		// If no current version, use highest version to reconcile
		reconcileProcedure, _ = allProcedures.Newest()
	}

	// Execute the reconcile procedure. Not sure how to handle the error
	procedureStatus, _ := reconcileProcedure.Execute(request, r.client, r.scheme)
	// Walk ProcedureStatus.Results and add to the CR status
	for _, result := range procedureStatus.Results {
		instance.Status.ReconcileActions[result.Name] = result.Result
	}

	err = r.client.Status().Update(context.TODO(), instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	// if ProcedureStatus.FullyReconciled
	//   update reconcile version in the CR to match the Procedure version
	//   use a timed reconcile requeue //left this part out. Why requeue?
	if procedureStatus.FullyReconciled {
		instance.Spec.Options["reconcileVersion"] = strconv.Itoa(reconcileProcedure.Version())
		err := r.client.Update(context.TODO(), instance)
		if err != nil {
			if errors.IsNotFound(err) {
				// Request object not found, could have been deleted after reconcile request.
				// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
				// Return and don't requeue
				return reconcile.Result{}, nil
			}
			// Error reading the object - requeue the request.
			return reconcile.Result{}, err
		}
	} else {
		//   requeue immediately
		return reconcile.Result{Requeue: true}, nil
	}

	return reconcile.Result{}, nil
}
