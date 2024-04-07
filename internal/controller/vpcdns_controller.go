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
	ovn "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	kubeovnv1 "multi-vpc/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
)

type VpcDnsReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
}

func (r *VpcDnsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	vpcDns := &kubeovnv1.VpcDns{}
	err := r.Get(ctx, req.NamespacedName, vpcDns)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !vpcDns.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleDelete(ctx, vpcDns)
	}
	return r.handleCreateOrUpdate(ctx, vpcDns)
}

func (r *VpcDnsReconciler) handleCreateOrUpdate(ctx context.Context, vpcDns *kubeovnv1.VpcDns) (ctrl.Result, error) {
	if !containsString(vpcDns.ObjectMeta.Finalizers, "dns.finalizer.ustc.io") {
		controllerutil.AddFinalizer(vpcDns, "dns.finalizer.ustc.io")
		err := r.Update(ctx, vpcDns)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	err := r.createDnsConnection(ctx, vpcDns)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *VpcDnsReconciler) handleDelete(ctx context.Context, vpcDns *kubeovnv1.VpcDns) (ctrl.Result, error) {
	if containsString(vpcDns.ObjectMeta.Finalizers, "dns.finalizer.ustc.io") {
		err := r.deleteDnsConnection(ctx, vpcDns)
		if err != nil {
			return ctrl.Result{}, err
		}
		controllerutil.RemoveFinalizer(vpcDns, "dns.finalizer.ustc.io")
		err = r.Update(ctx, vpcDns)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// 检查 Vpc-Dns 的 Corefile
func (r *VpcDnsReconciler) checkDnsCorefile(ctx context.Context) (bool, error) {
	cm := corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{
		Name:      "vpc-dns-corefile",
		Namespace: "kube-system",
	}, &cm)
	if err != nil {
		return false, err
	}
	if strings.Contains(cm.Data["Corefile"], "clusterset.local") {
		return true, nil
	} else {
		return false, nil
	}
}

// 更新 Vpc-Dns 的 Corefile
func (r *VpcDnsReconciler) updateDnsCorefile(ctx context.Context) error {
	cm := corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{
		Name:      "vpc-dns-corefile",
		Namespace: "kube-system",
	}, &cm)
	if err != nil {
		return err
	}
	// 获取 CoreDNS 的 svc
	var coreDnsSvc corev1.Service
	err = r.Client.Get(ctx, client.ObjectKey{
		Name:      "kube-dns",
		Namespace: "kube-system",
	}, &coreDnsSvc)
	if err != nil {
		return err
	}
	add := `clusterset.local:53 {
    forward . ` + coreDnsSvc.Spec.ClusterIP + `
  }
  .:53 {`
	cm.Data["Corefile"] = strings.Replace(cm.Data["Corefile"], ".:53 {", add, 1)
	err = r.Client.Update(ctx, &cm)
	// 获取 Vpc-Dns CR 和 Deployment
	var vpcDnsList ovn.VpcDnsList
	err = r.Client.List(ctx, &vpcDnsList, &client.ListOptions{})
	if err != nil {
		return err
	}
	// 重启所有的 vpc-dns 的 deployment
	for _, vpcDns := range vpcDnsList.Items {
		var vpcDnsDeployment appsv1.Deployment
		err = r.Client.Get(ctx, client.ObjectKey{
			Name:      "vpc-dns-" + vpcDns.Name,
			Namespace: "kube-system",
		}, &vpcDnsDeployment)
		if err != nil {
			return err
		}
		err = r.Client.Update(ctx, &vpcDnsDeployment)
		if err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}
	return nil
}

// 建立 DNS 路由转发
func (r *VpcDnsReconciler) createDnsConnection(ctx context.Context, vpcDns *kubeovnv1.VpcDns) error {
	state, err := r.checkDnsCorefile(ctx)
	if err != nil {
		return err
	}
	// 更新 Corefile
	if state == false {
		err = r.updateDnsCorefile(ctx)
		if err != nil {
			return err
		}
	}
	// 获取 CoreDNS 的 svc
	var coreDnsSvc corev1.Service
	err = r.Client.Get(ctx, client.ObjectKey{
		Name:      "kube-dns",
		Namespace: "kube-system",
	}, &coreDnsSvc)
	if err != nil {
		return err
	}
	// 获取 Vpc-Dns CR 和 Deployment
	var ovnDnsList ovn.VpcDnsList
	var ovnDns ovn.VpcDns
	err = r.Client.List(ctx, &ovnDnsList, &client.ListOptions{})
	if err != nil {
		return err
	}
	// 寻找资源状态为 true 的 Vpc-Dns
	for _, it := range ovnDnsList.Items {
		if it.Spec.Vpc == vpcDns.Spec.Vpc && it.Status.Active == true {
			ovnDns = it
			break
		}
	}
	var ovnDnsDeployment appsv1.Deployment
	err = r.Client.Get(ctx, client.ObjectKey{
		Name:      "vpc-dns-" + ovnDns.Name,
		Namespace: "kube-system",
	}, &ovnDnsDeployment)
	if err != nil {
		return err
	}
	// 获取默认子网 subnet
	var subnet ovn.Subnet
	err = r.Client.Get(ctx, client.ObjectKey{
		Name: "ovn-default",
	}, &subnet)
	if err != nil {
		return err
	}
	// 在 Vpc-Dns 的 Deployment 中 添加到 CoreDNS 的路由
	initContainers := ovnDnsDeployment.Spec.Template.Spec.InitContainers
	route := `ip -4 route add ` + coreDnsSvc.Spec.ClusterIP + ` via ` + subnet.Spec.Gateway + ` dev net1;`
	for i := 0; i < len(initContainers); i++ {
		for j := 0; j < len(initContainers[i].Command); j++ {
			if strings.Contains(initContainers[i].Command[j], "ip -4 route add") {
				if !strings.Contains(initContainers[i].Command[j], route) {
					initContainers[i].Command[j] = initContainers[i].Command[j] + route
				}
			}
		}
	}
	// 更新 Deployment
	err = r.Client.Update(ctx, &ovnDnsDeployment)
	if err != nil {
		return err
	}
	return nil
}

// 删除 DNS 路由转发
func (r *VpcDnsReconciler) deleteDnsConnection(ctx context.Context, vpcDns *kubeovnv1.VpcDns) error {
	// 获取 CoreDNS 的 svc
	var coreDnsSvc corev1.Service
	err := r.Client.Get(ctx, client.ObjectKey{
		Name:      "kube-dns",
		Namespace: "kube-system",
	}, &coreDnsSvc)
	if err != nil {
		return err
	}
	// 获取 Vpc-Dns CR 和 Deployment
	var ovnDnsList ovn.VpcDnsList
	var ovnDns ovn.VpcDns
	err = r.Client.List(ctx, &ovnDnsList, &client.ListOptions{})
	if err != nil {
		return err
	}
	// 寻找资源状态为 true 的 Vpc-Dns
	for _, it := range ovnDnsList.Items {
		if it.Spec.Vpc == vpcDns.Spec.Vpc && it.Status.Active == true {
			ovnDns = it
			break
		}
	}
	var ovnDnsDeployment appsv1.Deployment
	err = r.Client.Get(ctx, client.ObjectKey{
		Name:      "vpc-dns-" + ovnDns.Name,
		Namespace: "kube-system",
	}, &ovnDnsDeployment)
	if err != nil {
		return err
	}
	// 获取对应的 subnet
	var subnet ovn.Subnet
	err = r.Client.Get(ctx, client.ObjectKey{
		Name: "ovn-default",
	}, &subnet)
	if err != nil {
		return err
	}
	// 在 Vpc-Dns 的 Deployment 中 删除到 CoreDNS 的路由
	route := `ip -4 route add ` + coreDnsSvc.Spec.ClusterIP + ` via ` + subnet.Spec.Gateway + ` dev net1;`
	initContainers := ovnDnsDeployment.Spec.Template.Spec.InitContainers
	for i := 0; i < len(initContainers); i++ {
		for j := 0; j < len(initContainers[i].Command); j++ {
			if strings.Contains(initContainers[i].Command[j], route) {
				initContainers[i].Command[j] = strings.Replace(initContainers[i].Command[j], route, "", -1)
			}
		}
	}
	// 更新 Deployment
	err = r.Client.Update(ctx, &ovnDnsDeployment)
	// 更新状态
	vpcDns.Status.Initialized = true
	err = r.Update(ctx, vpcDns)
	if err != nil {
		return err
	}
	return nil
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *VpcDnsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeovnv1.VpcDns{}).
		Complete(r)
}