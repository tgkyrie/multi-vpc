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
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	// appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kubeovnv1 "multi-vpc/api/v1"
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
		log.Log.Error(err, "unable to fetch vpcNatTunnel")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !vpcTunnel.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleDelete(ctx, vpcTunnel)
	}
	return r.handleCreateOrUpdate(ctx, vpcTunnel)

}

func (r *VpcNatTunnelReconciler) execCommandInPod(podName, namespace, containerName, command string) error {
	clientset, err := kubernetes.NewForConfig(r.Config)
	if err != nil {
		return err
	}
	cmd := []string{
		"sh",
		"-c",
		command,
	}
	const tty = false
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).SubResource("exec").Param("container", containerName)
	req.VersionedParams(
		&v1.PodExecOptions{
			Command: cmd,
			Stdin:   false,
			Stdout:  true,
			Stderr:  true,
			TTY:     tty,
		},
		scheme.ParameterCodec,
	)

	var stdout, stderr bytes.Buffer
	exec, err := remotecommand.NewSPDYExecutor(r.Config, "POST", req.URL())
	if err != nil {
		return err
	}
	err = exec.StreamWithContext(context.TODO(), remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return err
	}
	// return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
	if strings.TrimSpace(stderr.String()) != "" {
		return fmt.Errorf(strings.TrimSpace(stderr.String()))
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VpcNatTunnelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Config = mgr.GetConfig()
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeovnv1.VpcNatTunnel{}).
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

func genCreateTunnelCmd(tunnel *kubeovnv1.VpcNatTunnel) string {
	createCmd := fmt.Sprintf("ip tunnel add %s mode gre remote %s local %s ttl 255", tunnel.Name, tunnel.Spec.RemoteIP, tunnel.Spec.InternalIP)
	setUpCmd := fmt.Sprintf("ip link set %s up", tunnel.Name)
	addrCmd := fmt.Sprintf("ip addr add %s dev %s", tunnel.Spec.InterfaceAddr, tunnel.Name)
	return createCmd + ";" + setUpCmd + ";" + addrCmd
}

// func genLastCreateTunnelCmd(tunnel *kubeovnv1.VpcNatTunnel) string {
// 	createCmd := fmt.Sprintf("ip tunnel add %s mode gre remote %s local %s ttl 255", tunnel.Name, tunnel.Status.RemoteIP, tunnel.Status.InternalIP)
// 	setUpCmd := fmt.Sprintf("ip link set %s up", tunnel.Name)
// 	addrCmd := fmt.Sprintf("ip addr add %s dev %s", tunnel.Status.InterfaceAddr, tunnel.Name)
// 	return createCmd + ";" + setUpCmd + ";" + addrCmd
// }

func genDeleteTunnelCmd(tunnel *kubeovnv1.VpcNatTunnel) string {
	delCmd := fmt.Sprintf("ip tunnel del %s", tunnel.Name)
	return delCmd
}

func (r *VpcNatTunnelReconciler) handleCreateOrUpdate(ctx context.Context, vpcTunnel *kubeovnv1.VpcNatTunnel) (ctrl.Result, error) {
	if !containsString(vpcTunnel.ObjectMeta.Finalizers, "tunnel.finalizer.ustc.io") {
		controllerutil.AddFinalizer(vpcTunnel, "tunnel.finalizer.ustc.io")
		err := r.Update(ctx, vpcTunnel)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if !vpcTunnel.Status.Initialized {
		// add tunnel
		podnext, err := r.getNatGwPod(vpcTunnel.Spec.NatGwDp) // find pod named Spec.NatGwDp
		if err != nil {
			return ctrl.Result{}, err
		}
		err = r.execCommandInPod(podnext.Name, podnext.Namespace, "vpc-nat-gw", genCreateTunnelCmd(vpcTunnel))
		if err != nil {
			return ctrl.Result{}, err
		}

		// statefulSet := &appsv1.StatefulSet{}
		// err = r.Get(ctx, types.NamespacedName{Name: "vpc-nat-gw-" + vpcTunnel.Spec.NatGwDp, Namespace: "kube-system"}, statefulSet) // get StatefulSet named Spec.NatGwDp
		// if err != nil {
		// 	return ctrl.Result{}, err
		// }
		// // get StatefulSet Containers command  at the begin is "while true; do sleep 10000; done"
		// oldRestartCommand := statefulSet.Spec.Template.Spec.Containers[0].Args[1]
		// // insert create tunnel command at head  in statefulSet Containers
		// newRestartCommand := genCreateTunnelCmd(vpcTunnel) + oldRestartCommand
		// statefulSet.Spec.Template.Spec.Containers[0].Args[1] = newRestartCommand

		vpcTunnel.Status.Initialized = true
		vpcTunnel.Status.InternalIP = vpcTunnel.Spec.InternalIP
		vpcTunnel.Status.RemoteIP = vpcTunnel.Spec.RemoteIP
		vpcTunnel.Status.InterfaceAddr = vpcTunnel.Spec.InterfaceAddr
		vpcTunnel.Status.NatGwDp = vpcTunnel.Spec.NatGwDp
		r.Status().Update(ctx, vpcTunnel)

	} else if vpcTunnel.Status.Initialized && (vpcTunnel.Status.InternalIP != vpcTunnel.Spec.InternalIP || vpcTunnel.Status.RemoteIP != vpcTunnel.Spec.RemoteIP ||
		vpcTunnel.Status.InterfaceAddr != vpcTunnel.Spec.InterfaceAddr || vpcTunnel.Status.NatGwDp != vpcTunnel.Spec.NatGwDp) {
		if vpcTunnel.Status.NatGwDp == vpcTunnel.Spec.NatGwDp {
			podnext, err := r.getNatGwPod(vpcTunnel.Spec.NatGwDp) // find pod named Spec.NatGwDp
			if err != nil {
				return ctrl.Result{}, err
			}
			err = r.execCommandInPod(podnext.Name, podnext.Namespace, "vpc-nat-gw", genDeleteTunnelCmd(vpcTunnel))
			if err != nil {
				return ctrl.Result{}, err
			}
			err = r.execCommandInPod(podnext.Name, podnext.Namespace, "vpc-nat-gw", genCreateTunnelCmd(vpcTunnel))
			if err != nil {
				return ctrl.Result{}, err
			}

			// statefulSet := &appsv1.StatefulSet{}
			// err := r.Get(ctx, types.NamespacedName{Name: "vpc-nat-gw-" + vpcTunnel.Status.NatGwDp, Namespace: "kube-system"}, statefulSet) // get StatefulSet named Status.NatGwDp
			// if err != nil {
			// 	return ctrl.Result{}, err
			// }
			// // get StatefulSet Containers command  at the begin is "while true; do sleep 10000; done"
			// oldRestartCommand := statefulSet.Spec.Template.Spec.Containers[0].Args[1]
			// // delete create tunnel command  in statefulSet Containers
			// comd := genLastCreateTunnelCmd(vpcTunnel)
			// index := strings.Index(oldRestartCommand, comd)
			// newRestartCommand := genCreateTunnelCmd(vpcTunnel) + oldRestartCommand[:index] + oldRestartCommand[index+len(comd):]
			// statefulSet.Spec.Template.Spec.Containers[0].Args[1] = newRestartCommand

			vpcTunnel.Status.InternalIP = vpcTunnel.Spec.InternalIP
			vpcTunnel.Status.RemoteIP = vpcTunnel.Spec.RemoteIP
			vpcTunnel.Status.InterfaceAddr = vpcTunnel.Spec.InterfaceAddr
			vpcTunnel.Status.NatGwDp = vpcTunnel.Spec.NatGwDp
			r.Status().Update(ctx, vpcTunnel)

		} else { // change the gw pod
			// update
			podlast, err := r.getNatGwPod(vpcTunnel.Status.NatGwDp) // find pod named Status.NatGwDp
			if err != nil {
				return ctrl.Result{}, err
			}
			err = r.execCommandInPod(podlast.Name, podlast.Namespace, "vpc-nat-gw", genDeleteTunnelCmd(vpcTunnel))
			if err != nil {
				return ctrl.Result{}, err
			}
			podnext, err := r.getNatGwPod(vpcTunnel.Spec.NatGwDp) // find pod named Status.NatGwDp
			if err != nil {
				return ctrl.Result{}, err
			}
			err = r.execCommandInPod(podnext.Name, podnext.Namespace, "vpc-nat-gw", genCreateTunnelCmd(vpcTunnel))
			if err != nil {
				return ctrl.Result{}, err
			}

			// statefulSet := &appsv1.StatefulSet{}
			// err := r.Get(ctx, types.NamespacedName{Name: "vpc-nat-gw-" + vpcTunnel.Status.NatGwDp, Namespace: "kube-system"}, statefulSet) // get StatefulSet named Status.NatGwDp
			// if err != nil {
			// 	return ctrl.Result{}, err
			// }
			// // get StatefulSet Containers command  at the begin is "while true; do sleep 10000; done"
			// oldRestartCommand := statefulSet.Spec.Template.Spec.Containers[0].Args[1]
			// // delete create tunnel command  in statefulSet Containers
			// comd := genLastCreateTunnelCmd(vpcTunnel)
			// index := strings.Index(oldRestartCommand, comd)
			// newRestartCommand := oldRestartCommand[:index] + oldRestartCommand[index+len(comd):]
			// statefulSet.Spec.Template.Spec.Containers[0].Args[1] = newRestartCommand

			// statefulSet = &appsv1.StatefulSet{}
			// err = r.Get(ctx, types.NamespacedName{Name: "vpc-nat-gw-" + vpcTunnel.Spec.NatGwDp, Namespace: "kube-system"}, statefulSet) // get StatefulSet named Spec.NatGwDp
			// if err != nil {
			// 	return ctrl.Result{}, err
			// }
			// oldRestartCommand = statefulSet.Spec.Template.Spec.Containers[0].Args[1]
			// // insert create tunnel command at head  in statefulSet Containers
			// newRestartCommand = genCreateTunnelCmd(vpcTunnel) + oldRestartCommand
			// statefulSet.Spec.Template.Spec.Containers[0].Args[1] = newRestartCommand

			vpcTunnel.Status.InternalIP = vpcTunnel.Spec.InternalIP
			vpcTunnel.Status.RemoteIP = vpcTunnel.Spec.RemoteIP
			vpcTunnel.Status.InterfaceAddr = vpcTunnel.Spec.InterfaceAddr
			vpcTunnel.Status.NatGwDp = vpcTunnel.Spec.NatGwDp
			r.Status().Update(ctx, vpcTunnel)
		}
	}
	return ctrl.Result{}, nil
}

func (r *VpcNatTunnelReconciler) handleDelete(ctx context.Context, vpcTunnel *kubeovnv1.VpcNatTunnel) (ctrl.Result, error) {
	if containsString(vpcTunnel.ObjectMeta.Finalizers, "tunnel.finalizer.ustc.io") {
		// TODO: implement clean up the GRE tunnel before deletion
		pod, err := r.getNatGwPod(vpcTunnel.Spec.NatGwDp)
		if err != nil {
			return ctrl.Result{}, err
		}
		err = r.execCommandInPod(pod.Name, pod.Namespace, "vpc-nat-gw", genDeleteTunnelCmd(vpcTunnel))
		if err != nil {
			return ctrl.Result{}, err
		}

		// statefulSet := &appsv1.StatefulSet{}
		// err := r.Get(ctx, types.NamespacedName{Name: "vpc-nat-gw-" + vpcTunnel.Spec.NatGwDp, Namespace: "kube-system"}, statefulSet) // get StatefulSet named Status.NatGwDp
		// if err != nil {
		// 	return ctrl.Result{}, err
		// }
		// // get StatefulSet Containers command  at the begin is "while true; do sleep 10000; done"
		// oldRestartCommand := statefulSet.Spec.Template.Spec.Containers[0].Args[1]
		// // delete create tunnel command  in statefulSet Containers
		// comd := genLastCreateTunnelCmd(vpcTunnel)
		// index := strings.Index(oldRestartCommand, comd)
		// newRestartCommand := oldRestartCommand[:index] + oldRestartCommand[index+len(comd):]
		// statefulSet.Spec.Template.Spec.Containers[0].Args[1] = newRestartCommand

		controllerutil.RemoveFinalizer(vpcTunnel, "tunnel.finalizer.ustc.io")
		err = r.Update(ctx, vpcTunnel)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
