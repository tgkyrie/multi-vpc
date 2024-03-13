/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kubeovnv1 "multi-vpc/api/v1"
	"multi-vpc/internal/util"
)

const (
	vpcGwTunnelFinalizerName = "tunnel.finalizer.ustc.io"
)

const (
	natGwTunnelAdd = "tunnel-add"
	natGwTunnelDel = "tunnel-del"
)

// VpcNatTunnelReconciler reconciles a VpcNatTunnel object
type VpcNatTunnelReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Config     *rest.Config
	KubeClient kubernetes.Interface
}

//+kubebuilder:rbac:groups=kubeovn.ustc.io,resources=vpcnattunnels,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubeovn.ustc.io,resources=vpcnattunnels/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubeovn.ustc.io,resources=vpcnattunnels/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods/exec,verbs=get;create

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the VpcNatTunnel object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *VpcNatTunnelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here
	vpcTunnel := &kubeovnv1.VpcNatTunnel{}
	err := r.Get(ctx, req.NamespacedName, vpcTunnel)
	if err != nil {
		// log.Log.Error(err, "unable to fetch vpcNatTunnel")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !vpcTunnel.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleDelete(ctx, vpcTunnel)
	}
	return r.handleCreateOrUpdate(ctx, vpcTunnel)
}

func (r *VpcNatTunnelReconciler) handleCreateOrUpdate(ctx context.Context, vpcTunnel *kubeovnv1.VpcNatTunnel) (ctrl.Result, error) {
	if !util.ContainsString(vpcTunnel.ObjectMeta.Finalizers, vpcGwTunnelFinalizerName) {
		controllerutil.AddFinalizer(vpcTunnel, vpcGwTunnelFinalizerName)
		err := r.Update(ctx, vpcTunnel)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	pod, err := r.getNatGwPod(vpcTunnel.Spec.NatGwDp)
	if err != nil {
		return ctrl.Result{}, err
	}
	log.Log.Info("createOrUpdate vpc gw tunnel")
	err = r.execNatGwRules(pod.Name, pod.Namespace, "vpc-nat-gw", natGwTunnelAdd, vpcTunnel)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *VpcNatTunnelReconciler) handleDelete(ctx context.Context, vpcTunnel *kubeovnv1.VpcNatTunnel) (ctrl.Result, error) {
	if util.ContainsString(vpcTunnel.ObjectMeta.Finalizers, vpcGwTunnelFinalizerName) {
		log.Log.Info("delete vpc gw tunnel")
		pod, err := r.getNatGwPod(vpcTunnel.Spec.NatGwDp)
		if err != nil {
			return ctrl.Result{}, err
		}
		err = r.execNatGwRules(pod.Name, pod.Namespace, "vpc-nat-gw", natGwTunnelDel, vpcTunnel)
		if err != nil {
			return ctrl.Result{}, err
		}
		controllerutil.RemoveFinalizer(vpcTunnel, vpcGwTunnelFinalizerName)
		err = r.Update(ctx, vpcTunnel)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VpcNatTunnelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Config = mgr.GetConfig()
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeovnv1.VpcNatTunnel{}).
		WithOptions(
			controller.Options{
				MaxConcurrentReconciles: 3,
			},
		).
		Complete(r)
}

func GenNatGwStsName(name string) string {
	return fmt.Sprintf("vpc-nat-gw-%s", name)
}

func (r *VpcNatTunnelReconciler) getNatGwPod(name string) (*corev1.Pod, error) {
	podList := &corev1.PodList{}
	matchLabels := map[string]string{"app": GenNatGwStsName(name), "ovn.kubernetes.io/vpc-nat-gw": "true"}
	listOpts := []client.ListOption{
		client.InNamespace("kube-system"),
		client.MatchingLabels(matchLabels),
	}
	err := r.List(context.TODO(), podList, listOpts...)
	if err != nil {
		return nil, err
	}
	pods := podList.Items
	switch {
	case len(pods) == 0:
		return nil, k8serrors.NewNotFound(corev1.Resource("pod"), name)
	case len(pods) != 1:
		time.Sleep(5 * time.Second)
		return nil, fmt.Errorf("too many pod")
	case pods[0].Status.Phase != "Running":
		time.Sleep(5 * time.Second)
		return nil, fmt.Errorf("pod is not active now")
	}

	return &pods[0], nil
}

func (r *VpcNatTunnelReconciler) execNatGwRules(podName, namespace, containerName, operation string, t *kubeovnv1.VpcNatTunnel) error {
	rules := []string{}
	rule := fmt.Sprintf("%s,%s,%s,%s,%s", t.Name, t.Spec.InternalIP, t.Spec.RemoteIP, t.Spec.InterfaceAddr, t.Spec.RemoteInterfaceAddr)
	rules = append(rules, rule)
	cmd := fmt.Sprintf("bash /kube-ovn/nat-gateway.sh %s %s", operation, strings.Join(rules, " "))
	return util.ExecCommandInPod(r.Config, podName, namespace, containerName, cmd)
}
